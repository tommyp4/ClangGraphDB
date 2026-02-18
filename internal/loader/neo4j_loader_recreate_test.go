package loader

import (
	"context"
	"strings"
	"testing"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// Mock Driver
type mockDriver struct {
	neo4j.DriverWithContext
	recordedSessions []*mockSession
}

func (d *mockDriver) NewSession(ctx context.Context, config neo4j.SessionConfig) neo4j.SessionWithContext {
	s := &mockSession{config: config}
	d.recordedSessions = append(d.recordedSessions, s)
	return s
}

// Mock Session
type mockSession struct {
	neo4j.SessionWithContext
	config          neo4j.SessionConfig
	executedQueries []string
	closed          bool
}

func (s *mockSession) ExecuteWrite(ctx context.Context, work neo4j.ManagedTransactionWork, config ...func(*neo4j.TransactionConfig)) (any, error) {
	tx := &mockTransaction{session: s}
	return work(tx)
}

func (s *mockSession) ExecuteRead(ctx context.Context, work neo4j.ManagedTransactionWork, config ...func(*neo4j.TransactionConfig)) (any, error) {
	tx := &mockTransaction{session: s}
	return work(tx)
}

func (s *mockSession) Close(ctx context.Context) error {
	s.closed = true
	return nil
}

// Mock Transaction
type mockTransaction struct {
	neo4j.ManagedTransaction
	session *mockSession
}

func (t *mockTransaction) Run(ctx context.Context, cypher string, params map[string]any) (neo4j.ResultWithContext, error) {
	t.session.executedQueries = append(t.session.executedQueries, cypher)
	res := &mockResult{}
	if strings.Contains(cypher, "SHOW DATABASES") {
		// Return a dummy record to simulate online status
		// We don't need actual values for Next() to return true, unless the code checks values.
		// The code checks Next(), so we need at least one record.
		res.records = []*neo4j.Record{{Values: []any{"testdb", "online"}}}
	}
	if strings.Contains(cypher, "CALL dbms.components()") {
		// Simulate Enterprise Edition response - return only edition
		res.records = []*neo4j.Record{{Values: []any{"enterprise"}}}
	}
	return res, nil
}

// Mock Result
type mockResult struct {
	neo4j.ResultWithContext
	records []*neo4j.Record
	current int
}

func (r *mockResult) Next(ctx context.Context) bool {
	if r.current < len(r.records) {
		r.current++
		return true
	}
	return false
}

func (r *mockResult) Record() *neo4j.Record {
	if r.current > 0 && r.current <= len(r.records) {
		return r.records[r.current-1]
	}
	return nil
}

func (r *mockResult) Collect(ctx context.Context) ([]*neo4j.Record, error) {
	return r.records, nil
}

func (r *mockResult) Consume(ctx context.Context) (neo4j.ResultSummary, error) {
	return nil, nil
}

// TestRecreateDatabase verifies that RecreateDatabase sends the correct commands to the system database.
func TestRecreateDatabase(t *testing.T) {
	mockDrv := &mockDriver{}
	loader := NewNeo4jLoader(mockDrv, "testdb", 768)

	// Since RecreateDatabase is currently a stub, this should just pass without errors, but fail validation
	err := loader.RecreateDatabase(context.Background())
	if err != nil {
		t.Fatalf("RecreateDatabase failed: %v", err)
	}

	// Validate interactions
	if len(mockDrv.recordedSessions) == 0 {
		t.Fatal("No session created")
	}

	session := mockDrv.recordedSessions[0]
	if session.config.DatabaseName != "system" {
		t.Errorf("Expected session to connect to 'system', got '%s'", session.config.DatabaseName)
	}

	expectedCommands := []string{
		"CALL dbms.components() YIELD edition RETURN edition",
		"STOP DATABASE testdb",
		"DROP DATABASE testdb IF EXISTS",
		"CREATE DATABASE testdb",
		"START DATABASE testdb",
		"SHOW DATABASES YIELD name, currentStatus",
	}

	if len(session.executedQueries) != len(expectedCommands) {
		t.Errorf("Expected %d queries, got %d", len(expectedCommands), len(session.executedQueries))
	}

	for i, cmd := range expectedCommands {
		if i < len(session.executedQueries) && !strings.Contains(session.executedQueries[i], cmd) {
			t.Errorf("Query %d mismatch. Want '%s', got '%s'", i, cmd, session.executedQueries[i])
		}
	}
}

// TestRecreateDatabaseCommunityEdition verifies that RecreateDatabase handles Community Edition correctly.
func TestRecreateDatabaseCommunityEdition(t *testing.T) {
	mockDrv := &mockDriverCommunity{}
	loader := NewNeo4jLoader(mockDrv, "testdb", 768)

	err := loader.RecreateDatabase(context.Background())
	if err != nil {
		t.Fatalf("RecreateDatabase failed: %v", err)
	}

	// Validate that it detected Community Edition and used Wipe+DropSchema
	if len(mockDrv.recordedSessions) < 2 {
		t.Fatal("Expected at least 2 sessions (system check, then testdb operations)")
	}

	// First session should be system to check edition
	systemSession := mockDrv.recordedSessions[0]
	if systemSession.config.DatabaseName != "system" {
		t.Errorf("Expected first session to connect to 'system', got '%s'", systemSession.config.DatabaseName)
	}

	// Should have called dbms.components() to check edition
	if len(systemSession.executedQueries) == 0 || !strings.Contains(systemSession.executedQueries[0], "CALL dbms.components()") {
		t.Errorf("Expected CALL dbms.components() as first query")
	}

	// Second session should be testdb for Wipe and DropSchema
	dbSession := mockDrv.recordedSessions[1]
	if dbSession.config.DatabaseName != "testdb" {
		t.Errorf("Expected second session to connect to 'testdb', got '%s'", dbSession.config.DatabaseName)
	}

	// Should have called MATCH (n) DETACH DELETE n
	foundWipe := false
	for _, query := range dbSession.executedQueries {
		if strings.Contains(query, "MATCH (n) DETACH DELETE n") {
			foundWipe = true
			break
		}
	}
	if !foundWipe {
		t.Errorf("Expected Wipe() to be called (MATCH (n) DETACH DELETE n)")
	}
}

// Mock Driver for Community Edition
type mockDriverCommunity struct {
	neo4j.DriverWithContext
	recordedSessions []*mockSessionCommunity
}

func (d *mockDriverCommunity) NewSession(ctx context.Context, config neo4j.SessionConfig) neo4j.SessionWithContext {
	s := &mockSessionCommunity{config: config}
	d.recordedSessions = append(d.recordedSessions, s)
	return s
}

// Mock Session for Community Edition
type mockSessionCommunity struct {
	neo4j.SessionWithContext
	config          neo4j.SessionConfig
	executedQueries []string
	closed          bool
}

func (s *mockSessionCommunity) ExecuteWrite(ctx context.Context, work neo4j.ManagedTransactionWork, config ...func(*neo4j.TransactionConfig)) (any, error) {
	tx := &mockTransactionCommunity{session: s}
	return work(tx)
}

func (s *mockSessionCommunity) ExecuteRead(ctx context.Context, work neo4j.ManagedTransactionWork, config ...func(*neo4j.TransactionConfig)) (any, error) {
	tx := &mockTransactionCommunity{session: s}
	return work(tx)
}

func (s *mockSessionCommunity) Close(ctx context.Context) error {
	s.closed = true
	return nil
}

// Mock Transaction for Community Edition
type mockTransactionCommunity struct {
	neo4j.ManagedTransaction
	session *mockSessionCommunity
}

func (t *mockTransactionCommunity) Run(ctx context.Context, cypher string, params map[string]any) (neo4j.ResultWithContext, error) {
	t.session.executedQueries = append(t.session.executedQueries, cypher)
	res := &mockResult{}
	if strings.Contains(cypher, "CALL dbms.components()") {
		// Simulate Community Edition response - return only edition in the first value
		res.records = []*neo4j.Record{{Values: []any{"community"}}}
	}
	if strings.Contains(cypher, "SHOW CONSTRAINTS") || strings.Contains(cypher, "SHOW INDEXES") {
		// Return empty results for schema operations
		res.records = []*neo4j.Record{}
	}
	return res, nil
}
