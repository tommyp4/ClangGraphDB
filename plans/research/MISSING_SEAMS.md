# GraphDB Skill Gap Analysis: Missing Architectural Seams

## Executive Summary
During a task to identify the best architectural seam within the "plating" module, the primary GraphDB queries (`query -type seams` and `query -type search-features`) returned empty results. This forced a fallback to legacy filesystem searches (`dir`, `grep`) to bridge the gap before the graph could be successfully utilized via `hybrid-context`. 

This document outlines the root causes for the failure of the initial graph queries and identifies gaps in both the database state and the query methodology.

## 1. Missing Database State: The Contamination Layer
**The Symptom:** `query -type seams -module ".*plating.*"` returned `[]`.
**The Root Cause:** The `enrich-contamination` pipeline step had not been executed (or failed silently) prior to the query.

### Evidence
A diagnostic traversal of a known core function (`CheckEC5PlateCapacity`) confirmed:
*   **Intent Layer Exists:** The node successfully mapped to `feature-9684556f` ("Eurocode 5 Plate Capacity Verification"). This confirms `enrich-features` completed successfully.
*   **Volatility Layer is Missing:** The function node lacked any properties related to `volatility`, `risk`, or `criticality`. There are no `CodeProperty` nodes of type `PinchPoint` in the graph. 

### Mechanism of Failure
The `seams` query is not a text search; it is a mathematical calculation that identifies "Pinch Points" by comparing internal fan-in against volatile fan-out. Because the `enrich-contamination` step was missing, the volatility baseline was zero across the graph, causing the seams algorithm to return an empty set regardless of the target module.

## 2. Query Methodology Gap: Feature Summaries vs. Code Semantics
**The Symptom:** `query -type search-features -target "plating"` returned `[]`.
**The Root Cause:** Misalignment between LLM-generated abstract summaries and specific domain terminology.

### Mechanism of Failure
The `search-features` command queries the *Intent Layer*. During `enrich-features`, the LLM abstracted the raw code into high-level features. For example, the plating logic was summarized as "Eurocode 5 Plate Capacity Verification" and "timber connector plates". Because the specific keyword "plating" wasn't highly weighted in the LLM summary, the vector search failed.

**The Correction:** `search-similar` should have been used. `search-similar` queries the function-level embeddings directly against the raw source code, docstrings, and signatures, which would have matched `EC5PlateCalc.cpp` immediately.

## 3. Tooling Gap: Silent Failures on Missing Prerequisites
**The Symptom:** The CLI returned an empty array `[]` instead of an error.
**The Root Cause:** The `graphdb` Go binary does not validate the existence of prerequisite data before executing complex analytical queries.

### Mechanism of Failure
When a user requests `query -type seams`, the tool currently executes the Cypher query against the database and returns whatever is found. Because the database was missing the contamination properties, it silently returned nothing. This leads the user/agent to falsely assume that the module has no seams, rather than realizing the graph is incomplete.

## Recommendations & Action Items

1.  **Immediate Remediation:** 
    Execute the contamination enrichment pipeline to populate the missing data:
    `graphdb enrich-contamination -module ".*"`
2.  **Agent Workflow Adjustment:** 
    Agents should prioritize `search-similar` with descriptive, context-heavy targets (e.g., "truss connector plate capacity calculation") over `search-features` when looking for specific implementation modules.
3.  **Tool Enhancement (GraphDB CLI):**
    Update the `graphdb query -type seams` and `query -type hotspots` commands to perform a pre-flight check. If the graph lacks contamination or git-history metadata, the CLI should return an explicit warning: `Warning: Contamination data missing. Please run 'enrich-contamination' first.` rather than returning an empty array.