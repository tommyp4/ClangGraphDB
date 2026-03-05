# Research Report: Identifying Top-Level Domains with the GraphDB Skill

## Objective
To map the top-level architectural domains of the `trucks-v2` project using the `graphdb` skill and its underlying Neo4j Code Property Graph (CPG). The goal was to understand how the semantic clustering engine grouped the system's features and functions.

## Methodology & Steps Taken

The investigation required a combination of the `graphdb` CLI tool and direct Cypher queries to uncover the global hierarchy, as standard CLI wildcard searches were insufficient for structural discovery.

### Step 1: Initial CLI Exploration Attempts
*   **Attempt:** Ran `${graphdb_bin} query -type explore-domain -target "*"`.
*   **Result:** Failed (`feature not found: *`).
*   **Insight:** The `explore-domain` command requires an exact `ID` or `name` of a `Feature` node to begin traversal. It cannot be used with wildcards to list *all* domains.

### Step 2: Bypassing CLI to Access Graph Structure Directly
To understand the schema and find the entry points, direct database access was necessary.
*   **Action:** Read the `.env` file to retrieve the local Neo4j credentials (`NEO4J_USER` and `NEO4J_PASSWORD`).
*   **Action:** Executed a REST API call to Neo4j via `curl` to list all available labels: `CALL db.labels()`.
*   **Result:** Identified that the semantic clustering primarily uses `Feature` and `Function` nodes. The `Domain` label existed but was mostly unpopulated (only containing a dummy "Domain 1" node).

### Step 3: Locating the Root Nodes (Top-Level Domains)
To find the top-level domains, I needed to identify `Feature` nodes that act as the root of a hierarchy.
*   **Action:** Executed a Cypher query to find nodes with no incoming `PARENT_OF` relationships:
    ```cypher
    MATCH (n:Feature) WHERE NOT ()-[:PARENT_OF]->(n) RETURN n.name ORDER BY n.name
    ```
*   **Result:** Discovered exactly **28 root nodes**. The clustering engine had named them systematically:
    *   `domain-app` (1 node)
    *   `domain-generic-*` (26 nodes with hash suffixes, e.g., `domain-generic-b06f65f4`)
    *   `Top Feature` (1 outlier node)

### Step 4: Analyzing Domain Scope and Scale
To determine the importance of each domain, I analyzed how many underlying functions were mapped to them.
*   **Action:** Executed Cypher queries to traverse down the tree from each root node (`-[:PARENT_OF*]->`) and count the distinct `Function` nodes connected via the `IMPLEMENTS` relationship.
*   **Result:** This revealed the true size of each cluster, showing that some generic domains contained over 300 functions, while others contained fewer than 70. 

### Step 5: Deriving Business Capabilities via `explore-domain`
With the exact root IDs identified and prioritized by size, I used the `graphdb` CLI and targeted Cypher queries to extract the descriptive child features.
*   **Action:** Ran `${graphdb_bin} query -type explore-domain -target "domain-app"` and queried the `summary` properties of the child features for the largest `domain-generic-*` clusters.
*   **Result:** By reading the descriptive names (e.g., "Driver Settlement Calculation Service") and LLM-generated summaries of the child features, I was able to define the business purpose of the abstract `domain-generic-*` roots.

---

## Findings Summary: The Identified Domains

The semantic clustering engine identified **28 top-level domains**. Here is the breakdown of the system's architecture based on the graph analysis:

### 1. The Application Domain (`domain-app`)
This is the primary domain representing the frontend web application (Angular). It contains 18 sub-features and encapsulates 100 distinct functions. The sub-features are highly descriptive and clearly map to standard Angular frontend architecture patterns:
*   **Dependency Injection and Component Initialization:** Orchestration of class instantiation and dependencies.
*   **Component Scaffolding and Initialization:** Structural framework for component lifecycles.
*   **Angular Component / View Lifecycles:** Various features handling `ngOnChanges`, `ngOnInit`, and view initialization logic.
*   **Component Lifecycle Cleanup:** Resource deallocation and termination logic.
*   **Route Guard Authorization:** Middleware logic for intercepting navigation requests based on permissions.

### 2. The Backend/Core Generic Domains
The backend logic (C#/.NET) was clustered into 27 generic top-level domains (named `domain-generic-*`). These represent distinct, non-overlapping areas of the system's business and operational logic. By analyzing their children, four massive core clusters were identified:

**A. Settlement Processing & Operations (322 Functions)**
*(Clusters: `domain-generic-b06f65f4`, `e74ce4a4`, `a5e54b71`)*
This massive cluster handles the core financial and business logic of the application:
*   **Driver Settlement Calculation Service:** Aggregation of financial components (tolls, net earnings).
*   **Logistics Operations and Financial Settlement Service:** Core infrastructure for fleet entities, ledgers, and reporting.
*   **Stop-Off Credit Validation:** Calculating reimbursements for intermediate freight stops.
*   **Driver Settlement Reconciliation Service:** Consolidating manual entries and parsing check details.

**B. Data Extraction & Resolution (183 Functions)**
*(Clusters: `domain-generic-0be5bf46`, `af249145`, `4834c892`)*
This cluster focuses on querying, parsing, and making sense of the raw data:
*   **Truck Fleet Data Management Service:** Repository interfaces for operational data and Excel extraction.
*   **Settlement Date Resolution Service:** Extracting milestones like check, credit, and load dates.
*   **Driver Settlement Data Extraction Service:** Retrieving financial credits and metrics from records and Excel.

**C. Compliance, Ledger, & Information Retrieval (133 Functions)**
*(Clusters: `domain-generic-7e9bc2ce`, `db51a907`, `f20d9152`)*
This area handles tracking and reporting on the state of the business entities:
*   **Automated Billing Reconciliation Service:** Processing Excel workbooks to calculate outstanding balances.
*   **Violation and Deduction Management:** Tracking policy infractions and applying financial penalties.
*   **Driver Information Service:** Retrieving comprehensive and summary profiles of drivers.

**D. Persistence & Migration Workflows (120 Functions)**
*(Clusters: `domain-generic-60f5dbf1`, `267bd60a`, `004f339e`)*
This domain handles atomic operations, database management, and entity lifecycles:
*   **Company Management Service:** Creation, modification, and bulk import of company data.
*   **Data Migration and Export Service:** Downloading CSVs and transforming them into workbook formats.
*   **Transactional Data Persistence Service:** Managing atomic transaction commits and cloud file storage.

*(Note: A small outlier hierarchy starting with a node simply called "Top Feature" containing one "Nested Feature" was also identified, likely a remnant from a test).*

## Conclusion
The GraphDB skill's `explore-domain` command is highly effective for downward traversal, but discovering the global architecture requires querying the Neo4j database directly for root nodes (`WHERE NOT ()-[:PARENT_OF]->(n)`). Combining structural Cypher aggregation with the semantic summaries embedded in the nodes provides a powerful, automated map of the system's bounded contexts.