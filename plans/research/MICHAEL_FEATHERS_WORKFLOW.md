# Working Effectively with Legacy Code using GraphDB

This document explores how Michael Feathers' core methodologies from his seminal book, *Working Effectively with Legacy Code*, map to the capabilities of the GraphDB CLI, and identifies potential areas for future enhancement.

Feathers’ foundational rule is: **Legacy code is code without tests.** To safely change it, you must get it under test. To get it under test, you must break dependencies. Here is how his manual algorithms translate to automated GraphDB workflows.

## The Feathers Workflow via GraphDB

### Step 1: Identifying the Target (Hotspot Analysis)
Before modernizing a large codebase, Feathers is pragmatic: you shouldn't write tests for everything, but for the parts of the code that are both **dangerous** and **frequently changing**.
*   **Feathers' Manual Approach:** Relying on bug reports, feature requests, or developer intuition to identify fragile areas.
*   **GraphDB Approach:** **Hotspot Analysis**.
    *   `graphdb query -type hotspots`
    *   This query combines structural risk (fan-in, fan-out, and proximity to volatile systems calculated during `enrich-contamination`) with temporal risk (git churn/frequency of change calculated during `enrich-history`). It returns a ranked "hit-list" of the top 20 functions that are most fragile and most frequently modified, providing an exact, data-driven starting point for modernization.

### Step 2: Orientation and the "Effect Sketch" (Impact Analysis)
Once a target is identified, Feathers needs to understand its architectural context and how a change will ripple outward.
*   **Feathers' Manual Approach:** Guessing at architectural boundaries, then manually tracing `Ctrl+Click` up the call stack and drawing boxes on a whiteboard.
*   **GraphDB Approach:** 
    1.  **Orientation:** He would start with `graphdb query -type overview` or the UI visualizer. This immediately returns the top-level Semantic Domains (e.g., "Authentication", "Tax Calculation", "Inventory") discovered by the RPG engine, giving him an instant, high-level map of the system's true architecture, regardless of directory structure.
    2.  **Impact Analysis:** Once he focuses on his target (e.g., "CalculateLegacyTax"), he runs `graphdb query -type impact -target "CalculateLegacyTax"`. This instantly traces the `CALLS` edges upward, showing exactly which upstream systems (UI, APIs, Batch Jobs) will break if the target function is modified. This determines the bounding box of necessary test coverage.

### Step 3: Finding the "Pinch Point"
Feathers defines a **Pinch Point** as a narrow place in a graph of calls where you can write a few tests to cover a massive amount of internal behavior, acting as a chokepoint before the code hits a volatile dependency (like a database).
*   **GraphDB Approach:** **Structural Seams** query.
    *   `graphdb query -type seams`
    *   This query mathematically calculates exactly what Feathers looks for: `Internal Fan-In * Volatile Fan-Out`. It returns a ranked list of the exact functions where a mock object or interface should be introduced, identifying where testing will yield the highest return on investment.

### Step 4: Sensing and Separation (Dealing with Hidden State)
To get a class under test, you must be able to instantiate it. Legacy classes often secretly instantiate database connections or rely on invisible global variables.
*   **GraphDB Approach:** **Neighbors and Globals** queries.
    *   `graphdb query -type neighbors -target "OrderProcessor"` 
    *   `graphdb query -type globals`
    *   GraphDB exposes exactly what globals a class touches (`USES_GLOBAL`) and what structural dependencies it owns (`DEFINES`). This generates a concrete checklist of dependencies that must be Parameterized (via Dependency Injection) or mocked out before instantiation in a test harness is possible.

### Step 5: Breaking God Classes (Sprout/Wrap Prep)
When code is too tangled to test, Feathers recommends the "Sprout Class" or "Wrap Class" techniques—extracting behavior rather than adding to the mess.
*   **GraphDB Approach:** **Semantic Seams** query.
    *   `graphdb query -type semantic-seams`
    *   Instead of guessing where to split a 5,000-line class, GraphDB uses vector embeddings to highlight exactly where the Single Responsibility Principle (SRP) breaks down. If two functions in the same file have a very low semantic similarity score (e.g., `< 0.5`), they belong in different architectural domains, clearly identifying the target for a "Sprout Class" extraction.

---

## Gap Analysis: What is GraphDB Missing?

While GraphDB is an incredible reconnaissance tool, it currently lacks a few elements required for a complete Feathers workflow:

### 1. "Object Seam" Discovery (Extract Interface Candidates)
Feathers relies heavily on "Object Seams" (using polymorphism to swap implementations at runtime). GraphDB lacks a dedicated query to identify classes that are heavily depended upon but lack interfaces.

*   **Feasibility:** **High / Easy**. The structural graph already contains `IMPLEMENTS` and `EXTENDS` edges. We could easily expose a new `graphdb query -type object-seams` query in the backend:
    ```cypher
    MATCH (c:Class)
    // 1. Find classes that are heavily depended upon
    OPTIONAL MATCH (caller)-[:DEPENDS_ON|CALLS]->(c)
    WITH c, count(caller) as incoming_refs
    WHERE incoming_refs > 5

    // 2. Filter for classes that do NOT implement an interface
    OPTIONAL MATCH (c)-[:IMPLEMENTS]->(i:Interface)
    WITH c, incoming_refs, count(i) as interfaces_implemented
    WHERE interfaces_implemented = 0

    RETURN c.name, incoming_refs
    ```

### 2. Local Variable Data-Flow (The "Parameterize Method" refactoring)
GraphDB operates at the *structural* level. It cannot easily track if a specific local variable `x` eventually flows into a database call `db.Save(x)`.

*   **Feasibility:** **Low / Hard**. Currently, the Tree-sitter queries are optimized for top-level structural boundaries and intentionally ignore local variables (`variable_declarator`) to keep the graph size manageable.
    Implementing this would require:
    1. Updating Tree-sitter queries to capture local variables.
    2. Adding `DATA_FLOWS_TO` edges.
    3. Building a complex lexical scoping engine on top of Tree-sitter (which only understands syntax, not semantics) to resolve variable shadowing across all supported languages. This pushes GraphDB toward becoming a full static analysis compiler.

### 3. "Link Seams" and "Preprocessing Seams"
Feathers discusses bypassing dependencies via the compiler/linker (e.g., swapping `.dll` files, overriding `#include` macros). GraphDB analyzes raw text on disk via Tree-sitter, not the build system, so it cannot model compiler-level linking.

### 4. Refactoring Automation (Mutation)
GraphDB is strictly a **read-only reconnaissance tool**. It plans the attack, but the actual refactoring (e.g., "Extract Method") must still be performed manually (or via an orchestration layer like the Gemini Engineer Agent).
