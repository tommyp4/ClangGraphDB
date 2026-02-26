# Feature Implementation Plan: Refactor Node IDs

## 📋 Todo Checklist
- [x] Define global `Label:FQN:Signature` ID standard.
- [x] Update C# Parser to new ID standard and add `fqn` property.
- [ ] Update Java Parser to new ID standard and add `fqn` property.
- [ ] Update TypeScript Parser to new ID standard and add `fqn` property.
- [ ] Update C++ Parser to new ID standard and add `fqn` property.
- [ ] Update SQL Parser to new ID standard and add `fqn` property.
- [ ] Refactor Neo4j Loader (`buildEdgeQuery`) to match edges on `id OR fqn`.
- [ ] Refactor Query Engine (`neo4j.go`) to support querying by `fqn`.
- [ ] Update `.gemini/skills/graphdb/SKILL.md` to reflect new ID standards and query flexibility.
- [ ] Final Review and Testing (Verify zero duplicate IDs and successful ingestion).

## 🔍 Analysis & Investigation
The Neo4j ingestion process fails with fatal `ConstraintValidationFailed` errors during `BatchLoadNodes` due to duplicate `id` properties on `CodeElement` nodes. Neo4j enforces a constraint: `CREATE CONSTRAINT IF NOT EXISTS FOR (n:CodeElement) REQUIRE n.id IS UNIQUE`. When two nodes with the identical `id` but different labels are batched, Neo4j attempts to create a second node, violating the constraint.

**Root Causes (Collisions):**
1.  **Cross-Label Collisions:** The C# parser generates the same ID for a Class and its Constructor. If they have the same fully qualified name, they clash between the `Class` and `Function` batches.
2.  **Field vs Method Collisions:** In TypeScript and Java, a field (e.g., `count`) and a method (e.g., `count()`) generate the identical ID within the same class, clashing between `Field` and `Function` batches.
3.  **Overloaded Methods:** Methods with the same name but different signatures generate the exact same ID (e.g., `Process` for both `Process(int)` and `Process(string)`).

**Requirements for the Solution:**
- **Deterministic Unique IDs:** IDs must be globally deterministic and prevent all collisions. We must use the format `Label:FQN:Signature`.
- **FQN Path Independence:** For statically typed languages like C# and Java, the Fully Qualified Name (FQN) **MUST NOT** include file paths because callers do not know the file path of external dependencies. For TypeScript and C++, file paths **MUST** be included because module resolution relies on paths.
- **Agent UX & Cypher Matching:** Because IDs will include complex signatures, LLM agents cannot easily guess them. Nodes must explicitly store an `fqn` property. The Query Engine and Edge Builder must gracefully fall back to matching on `fqn` when an exact `id` isn't found.

## 📝 Implementation Plan

### Prerequisites
- Go environment configured.
- Access to `internal/analysis/*.go`, `internal/loader/neo4j_loader.go`, and `internal/query/neo4j.go`.

### Step-by-Step Implementation

#### Phase 1: Global ID Standard & Test Harness
1.  **Step 1.A (The Harness):** Define the parser verification requirement.
    *   *Action:* Create `test/fixtures/IDCollisionTest.cs` (or Java) containing overloaded methods, constructors, and a field with the same name as a method.
    *   *Goal:* Assert that the generated IDs are unique and no duplicates exist. Assert `fqn` properties are correctly populated.
2.  **Step 1.B (The Implementation):** Implement a standardized ID generator function.
    *   *Action:* In `internal/analysis/parser.go` (or a similar shared location), create:
        ```go
        func GenerateNodeID(label string, fqn string, signature string) string {
            // E.g. "Function:MyApp.User.Process:(int, string)"
            // Fallback for empty signature: "Class:MyApp.User:"
            return fmt.Sprintf("%s:%s:%s", label, fqn, signature)
        }
        ```

#### Phase 2: Update Parsers
Update Node instantiation logic in each parser to use the new ID standard and inject the `fqn` property.

1.  **Step 2.A (C# Parser):**
    *   *Action:* Modify `internal/analysis/csharp.go`.
    *   *Detail:*
        - Build `fqn = Namespace.Class.Member` (NO file path).
        - For overloaded methods/constructors, extract the parameter types/names (or parameter count as a fallback) to form a `signature`.
        - Set `n.ID = GenerateNodeID(label, fqn, signature)`.
        - Add `n.Properties["fqn"] = fqn`.
        - Ensure edge `SourceID` uses the exact `n.ID`. The `TargetID` uses the resolved candidate's `FQN`.

2.  **Step 2.B (Java Parser):**
    *   *Action:* Modify `internal/analysis/java.go`.
    *   *Detail:*
        - Build `fqn = Package.Class.Member` (NO file path).
        - Generate unique `signature` from parameter types.
        - Add `n.Properties["fqn"] = fqn` and update `n.ID`.

3.  **Step 2.C (TypeScript Parser):**
    *   *Action:* Modify `internal/analysis/typescript.go`.
    *   *Detail:*
        - Build `fqn = FilePath:Class.Member` (File path required for TS).
        - Add `n.Properties["fqn"] = fqn` and update `n.ID`.

4.  **Step 2.D (C++ Parser):**
    *   *Action:* Modify `internal/analysis/cpp.go`.
    *   *Detail:*
        - Build `fqn = FilePath:Namespace.Class.Member`.
        - Add `fqn` property and update ID.

5.  **Step 2.E (SQL Parser):**
    *   *Action:* Modify `internal/analysis/sql.go`.
    *   *Detail:*
        - Build `fqn = Schema.Table.Column` or `Schema.Procedure`.
        - Add `fqn` property and update ID.

#### Phase 3: Edge Linking & Neo4j Loader
Because `TargetID` for edges will now often just be an `FQN` (as the exact signature may not be statically analyzable at the call site), the Neo4j Loader must link edges using `id OR fqn`. If multiple overloads match the FQN, it correctly links to all.

1.  **Step 3.A (The Harness):** Define verification for edge linking.
    *   *Action:* Add test in `internal/loader/neo4j_loader_test.go` to mock loading edges and confirm edges match target nodes via `fqn` when `id` isn't an exact match.
2.  **Step 3.B (The Implementation):** Refactor `BatchLoadEdges`.
    *   *Action:* Modify `internal/loader/neo4j_loader.go` within `buildEdgeQuery`.
    *   *Detail:* Change the Cypher query to support FQN fallback:
        ```cypher
        UNWIND $batch AS row
        MATCH (source:CodeElement) WHERE source.id = row.sourceId
        MATCH (target:CodeElement) WHERE target.id = row.targetId OR target.fqn = row.targetId
        MERGE (source)-[r:%s]->(target)
        ```

#### Phase 4: Query Engine & Agent UX
Agents will formulate queries using known structures (`name` or `fqn`). We must ensure all queries gracefully handle `fqn` as an entry point.

1.  **Step 4.A (The Harness):** Update query engine tests.
    *   *Action:* Modify `internal/query/neo4j_test.go` to test fetching nodes and their neighbors using `fqn`.
2.  **Step 4.B (The Implementation):** Update Cypher predicates.
    *   *Action:* Modify `internal/query/neo4j.go`.
    *   *Detail:* Locate queries in `GetNeighbors`, `Traverse`, `GetCallers`, `FetchSource`, `GetGlobals`, `GetImpact`, etc.
    *   *Refactor:* Change predicates from `MATCH (n) WHERE n.id = $id OR n.name = $id` to:
        ```cypher
        MATCH (n) WHERE n.id = $id OR n.fqn = $id OR n.name = $id
        ```

#### Phase 5: Update Skill Documentation
The `SKILL.md` file must be synchronized with the new architectural standards to ensure LLM agents utilize the graph correctly.

1.  **Step 5.A (The Implementation):** Update `.gemini/skills/graphdb/SKILL.md`.
    *   *Action:* Modify the documentation.
    *   *Detail:*
        - Update the **Supported Languages & FQN Formats** section. Explain that while IDs are internally complex (`Label:FQN:Signature`), the query engine is polymorphic and accepts `ID`, `fqn`, or `name`.
        - Clarify that for C# and Java, FQNs are **always** `Namespace.Class.Method`, while for TS/C++, they are `FilePath:Class.Method`.
        - Update the **Operational Guidelines** to instruct the agent to use `fqn` for structural queries to ensure robust matching across overloads and signatures.

### Testing Strategy
- Run `go test ./internal/analysis/...` to verify IDs are collision-free and format strictly as `Label:FQN:Signature`.
- Run `go test ./internal/query/...` to ensure Cypher queries remain functional.
- Execute a full, dry-run ingestion (`make build && ./bin/graphdb build-all -dir .`) and inspect the generated `nodes.jsonl` output for duplicate `id`s using bash `jq` or `awk`. Verify no `ConstraintValidationFailed` errors occur.

## 🎯 Success Criteria
- [ ] Nodes are written to `nodes.jsonl` with mathematically unique IDs across all labels and overloads.
- [ ] `BatchLoadNodes` completes successfully without Neo4j throwing constraint errors.
- [ ] All code elements possess a populated `fqn` property.
- [ ] Edges are correctly established between dependencies even if only the `FQN` is known at the call site.
- [ ] Agents can successfully execute graph structural queries (neighbors, impact, callers) using `FQN` inputs.