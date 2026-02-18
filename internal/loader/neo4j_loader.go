package loader

import (
	"context"
	"fmt"
	"graphdb/internal/graph"
	"log"
	"strings"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// Neo4jLoader handles batch loading of graph data into Neo4j.
type Neo4jLoader struct {
	Driver              neo4j.DriverWithContext
	DBName              string
	EmbeddingDimensions int
}

// NewNeo4jLoader creates a new loader instance.
func NewNeo4jLoader(driver neo4j.DriverWithContext, dbName string, embeddingDimensions int) *Neo4jLoader {
	return &Neo4jLoader{
		Driver:              driver,
		DBName:              dbName,
		EmbeddingDimensions: embeddingDimensions,
	}
}

// BatchLoadNodes loads a batch of nodes using UNWIND.
func (l *Neo4jLoader) BatchLoadNodes(ctx context.Context, nodes []graph.Node) error {
	if len(nodes) == 0 {
		return nil
	}

	batches := groupNodesByLabel(nodes)

	session := l.Driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: l.DBName})
	defer session.Close(ctx)

	for label, batch := range batches {
		query := buildNodeQuery(label)
		_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
			return tx.Run(ctx, query, map[string]any{"batch": batch})
		})
		if err != nil {
			return fmt.Errorf("failed to load nodes for label %s: %w", label, err)
		}
	}

	return nil
}

// BatchLoadEdges loads a batch of edges using UNWIND.
func (l *Neo4jLoader) BatchLoadEdges(ctx context.Context, edges []graph.Edge) error {
	if len(edges) == 0 {
		return nil
	}

	batches := groupEdgesByType(edges)

	session := l.Driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: l.DBName})
	defer session.Close(ctx)

	for relType, batch := range batches {
		query := buildEdgeQuery(relType)
		_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
			return tx.Run(ctx, query, map[string]any{"batch": batch})
		})
		if err != nil {
			return fmt.Errorf("failed to load edges for type %s: %w", relType, err)
		}
	}

	return nil
}

// Wipe deletes all data from the database.
func (l *Neo4jLoader) Wipe(ctx context.Context) error {
	session := l.Driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: l.DBName})
	defer session.Close(ctx)

	query := buildWipeQuery()
	// Using APOC if available is better: "CALL apoc.periodic.iterate('MATCH (n) RETURN n', 'DETACH DELETE n', {batchSize:1000})".
	// But without APOC, we just run a simple delete. For large DBs this might timeout, but it matches the JS impl.
	
	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		return tx.Run(ctx, query, nil)
	})
	return err
}

// ApplyConstraints creates uniqueness constraints and indexes.
func (l *Neo4jLoader) ApplyConstraints(ctx context.Context) error {
	session := l.Driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: l.DBName})
	defer session.Close(ctx)
	
	var errs []string

	for _, query := range l.getConstraints() {
		_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
			return tx.Run(ctx, query, nil)
		})
		if err != nil {
			// Log the error but continue to the next constraint
			msg := fmt.Sprintf("failed to apply constraint: %s. Error: %v", query, err)
			log.Printf("WARN: %s", msg)
			errs = append(errs, msg)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("encountered %d errors applying constraints:\n%s", len(errs), strings.Join(errs, "\n"))
	}

	return nil
}

// RecreateDatabase drops and recreates the database to ensure a clean slate.
func (l *Neo4jLoader) RecreateDatabase(ctx context.Context) error {
	session := l.Driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: "system"})
	defer session.Close(ctx)

	// Probe for Enterprise/Standard Edition capabilities using dbms.components()
	// Community Edition does not support CREATE DATABASE (multi-tenancy)
	edition, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		result, err := tx.Run(ctx, "CALL dbms.components() YIELD edition RETURN edition", nil)
		if err != nil {
			return "", err
		}
		records, err := result.Collect(ctx)
		if err != nil {
			return "", err
		}
		if len(records) > 0 && len(records[0].Values) > 0 {
			if ed, ok := records[0].Values[0].(string); ok {
				return ed, nil
			}
		}
		return "", nil
	})

	isCommunity := false
	if err != nil {
		// If dbms.components() fails, assume Community Edition
		log.Printf("WARN: Edition detection failed: %v. Assuming Community Edition.", err)
		isCommunity = true
	} else if edition == "community" {
		isCommunity = true
	}

	if isCommunity {
		log.Println("Community Edition detected. Using Wipe() + DropSchema() strategy.")
		if err := l.Wipe(ctx); err != nil {
			return err
		}
		return l.DropSchema(ctx)
	}

	// Enterprise/Standard Strategy: Full Drop/Create
	log.Printf("Enterprise/Standard Edition detected (%s). Using DROP/CREATE strategy.", edition)
	commands := []string{
		fmt.Sprintf("STOP DATABASE %s", l.DBName),
		fmt.Sprintf("DROP DATABASE %s IF EXISTS", l.DBName),
		fmt.Sprintf("CREATE DATABASE %s", l.DBName),
		fmt.Sprintf("START DATABASE %s", l.DBName),
	}

	for _, cmd := range commands {
		_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
			return tx.Run(ctx, cmd, nil)
		})
		if err != nil {
			// If STOP fails because it doesn't exist, ignore
			if strings.HasPrefix(cmd, "STOP DATABASE") {
				continue
			}
			return fmt.Errorf("failed to execute '%s': %w", cmd, err)
		}
	}

	return l.waitForDatabaseOnline(ctx, session)
}

// DropSchema drops all constraints and indexes.
func (l *Neo4jLoader) DropSchema(ctx context.Context) error {
	session := l.Driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: l.DBName})
	defer session.Close(ctx)

	// 1. Drop Constraints
	constraints, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		result, err := tx.Run(ctx, "SHOW CONSTRAINTS YIELD name", nil)
		if err != nil {
			return nil, err
		}
		var names []string
		for result.Next(ctx) {
			if name, ok := result.Record().Get("name"); ok {
				names = append(names, name.(string))
			}
		}
		return names, nil
	})
	if err != nil {
		return fmt.Errorf("failed to list constraints: %w", err)
	}

	for _, name := range constraints.([]string) {
		_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
			return tx.Run(ctx, fmt.Sprintf("DROP CONSTRAINT %s", name), nil)
		})
		if err != nil {
			log.Printf("WARN: Failed to drop constraint %s: %v", name, err)
		}
	}

	// 2. Drop Indexes
	indexes, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		result, err := tx.Run(ctx, "SHOW INDEXES YIELD name, type WHERE type <> 'LOOKUP'", nil) // Skip LOOKUP indexes (internal)
		if err != nil {
			return nil, err
		}
		var names []string
		for result.Next(ctx) {
			if name, ok := result.Record().Get("name"); ok {
				names = append(names, name.(string))
			}
		}
		return names, nil
	})
	if err != nil {
		return fmt.Errorf("failed to list indexes: %w", err)
	}

	for _, name := range indexes.([]string) {
		_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
			return tx.Run(ctx, fmt.Sprintf("DROP INDEX %s", name), nil)
		})
		if err != nil {
			log.Printf("WARN: Failed to drop index %s: %v", name, err)
		}
	}

	return nil
}

func (l *Neo4jLoader) waitForDatabaseOnline(ctx context.Context, session neo4j.SessionWithContext) error {
	query := "SHOW DATABASES YIELD name, currentStatus WHERE name = $name AND currentStatus = 'online'"
	
	// Poll for status "online"
	for i := 0; i < 30; i++ { // Try for 30 seconds
		online, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
			result, err := tx.Run(ctx, query, map[string]any{"name": l.DBName})
			if err != nil {
				return false, err
			}
			if result.Next(ctx) {
				return true, nil
			}
			return false, nil
		})
		
		if err != nil {
			return fmt.Errorf("error checking database status: %w", err)
		}
		
		if online == true {
			return nil
		}
		
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1 * time.Second):
			// continue loop
		}
	}
	return fmt.Errorf("timeout waiting for database %s to come online", l.DBName)
}

// UpdateGraphState updates the commit hash.
func (l *Neo4jLoader) UpdateGraphState(ctx context.Context, commit string) error {
	session := l.Driver.NewSession(ctx, neo4j.SessionConfig{DatabaseName: l.DBName})
	defer session.Close(ctx)

	query := buildGraphStateQuery()
	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		return tx.Run(ctx, query, map[string]any{"commit": commit})
	})
	return err
}

// Helpers extracted for testing
func groupNodesByLabel(nodes []graph.Node) map[string][]map[string]any {
	batches := make(map[string][]map[string]any)
	for _, n := range nodes {
		label := n.Label
		if label == "" {
			label = "Generic"
		}
		
		props := make(map[string]any)
		for k, v := range n.Properties {
			props[k] = v
		}
		props["id"] = n.ID
		
		batches[label] = append(batches[label], props)
	}
	return batches
}

func buildNodeQuery(label string) string {
	return fmt.Sprintf(`
			UNWIND $batch AS row
			MERGE (n:%s {id: row.id})
			SET n += row
			SET n:CodeElement
		`, sanitizeLabel(label))
}

func groupEdgesByType(edges []graph.Edge) map[string][]map[string]any {
	batches := make(map[string][]map[string]any)
	for _, e := range edges {
		relType := e.Type
		if relType == "" {
			relType = "RELATED_TO"
		}
		
		row := map[string]any{
			"sourceId": e.SourceID,
			"targetId": e.TargetID,
		}
		batches[relType] = append(batches[relType], row)
	}
	return batches
}

func buildEdgeQuery(relType string) string {
	return fmt.Sprintf(`
			UNWIND $batch AS row
			MATCH (source:CodeElement {id: row.sourceId})
			MATCH (target:CodeElement {id: row.targetId})
			MERGE (source)-[r:%s]->(target)
		`, sanitizeLabel(relType))
}

func buildWipeQuery() string {
	return "MATCH (n) DETACH DELETE n"
}

func buildGraphStateQuery() string {
	return `
		MERGE (s:GraphState)
		SET s.commit = $commit, s.updatedAt = datetime()
	`
}

func (l *Neo4jLoader) getConstraints() []string {
	dims := l.EmbeddingDimensions
	if dims <= 0 {
		dims = 768 // Fallback default
	}

	return []string{
		"CREATE CONSTRAINT IF NOT EXISTS FOR (n:File) REQUIRE n.id IS UNIQUE",
		"CREATE CONSTRAINT IF NOT EXISTS FOR (n:Function) REQUIRE n.id IS UNIQUE",
		"CREATE CONSTRAINT IF NOT EXISTS FOR (n:Class) REQUIRE n.id IS UNIQUE",
		"CREATE CONSTRAINT IF NOT EXISTS FOR (n:CodeElement) REQUIRE n.id IS UNIQUE",
		"CREATE INDEX IF NOT EXISTS FOR (n:Function) ON (n.name)",
		"CREATE INDEX IF NOT EXISTS FOR (n:File) ON (n.file)",
		// Vector Indexes (restored from Node.js implementation)
		fmt.Sprintf(`CREATE VECTOR INDEX feature_embeddings IF NOT EXISTS
		FOR (n:Feature) ON (n.embedding)
		OPTIONS {indexConfig: {
			`+"`vector.dimensions`"+`: %d,
			`+"`vector.similarity_function`"+`: 'cosine'
		}}`, dims),
		fmt.Sprintf(`CREATE VECTOR INDEX function_embeddings IF NOT EXISTS
		FOR (n:Function) ON (n.embedding)
		OPTIONS {indexConfig: {
			`+"`vector.dimensions`"+`: %d,
			`+"`vector.similarity_function`"+`: 'cosine'
		}}`, dims),
	}
}

func sanitizeLabel(label string) string {
	return strings.ReplaceAll(label, "`", "")
}
