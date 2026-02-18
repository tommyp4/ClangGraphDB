package loader

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// --- Mocks for Constraints Test ---

type mockConstraintDriver struct {
	neo4j.DriverWithContext
	failQuery string // If set, queries containing this string will fail
	executed  []string
}

func (d *mockConstraintDriver) NewSession(ctx context.Context, config neo4j.SessionConfig) neo4j.SessionWithContext {
	return &mockConstraintSession{driver: d}
}

type mockConstraintSession struct {
	neo4j.SessionWithContext
	driver *mockConstraintDriver
}

func (s *mockConstraintSession) ExecuteWrite(ctx context.Context, work neo4j.ManagedTransactionWork, config ...func(*neo4j.TransactionConfig)) (any, error) {
	tx := &mockConstraintTransaction{session: s}
	return work(tx)
}

func (s *mockConstraintSession) Close(ctx context.Context) error {
	return nil
}

type mockConstraintTransaction struct {
	neo4j.ManagedTransaction
	session *mockConstraintSession
}

func (t *mockConstraintTransaction) Run(ctx context.Context, cypher string, params map[string]any) (neo4j.ResultWithContext, error) {
	t.session.driver.executed = append(t.session.driver.executed, cypher)
	
	if t.session.driver.failQuery != "" && strings.Contains(cypher, t.session.driver.failQuery) {
		return nil, fmt.Errorf("mock error for query: %s", cypher)
	}
	
	return &mockConstraintResult{}, nil
}

// Reuse mockResult from other test or define simple one
type mockConstraintResult struct {
	neo4j.ResultWithContext
}

func (r *mockConstraintResult) Consume(ctx context.Context) (neo4j.ResultSummary, error) {
	return nil, nil
}

// --- Test ---

func TestApplyConstraints_ContinuesAfterFailure(t *testing.T) {
	// Setup mock driver that fails on the first unique constraint
	mockDrv := &mockConstraintDriver{
		failQuery: "CREATE CONSTRAINT IF NOT EXISTS FOR (n:File) REQUIRE n.id IS UNIQUE",
	}
	
	loader := NewNeo4jLoader(mockDrv, "testdb", 768)
	
	// Call ApplyConstraints
	err := loader.ApplyConstraints(context.Background())
	
	// Verification
	
	// 1. It should return an error because one constraint failed.
	if err == nil {
		t.Error("Expected error from ApplyConstraints, got nil")
	}
	
	// 2. It should have attempted ALL constraints.
	// We know getConstraints returns ~8 queries.
	expectedCount := len(loader.getConstraints())
	if len(mockDrv.executed) != expectedCount {
		t.Errorf("Expected to execute all %d constraints, but only executed %d", expectedCount, len(mockDrv.executed))
	}
	
	// 3. Verify Vector Indexes were attempted (they are last in the list)
	foundVectorIndex := false
	for _, q := range mockDrv.executed {
		if strings.Contains(q, "CREATE VECTOR INDEX feature_embeddings") {
			foundVectorIndex = true
			break
		}
	}
	
	if !foundVectorIndex {
		t.Error("Expected vector index creation to be attempted despite earlier failure")
	}
}
