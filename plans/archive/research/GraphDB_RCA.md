# Root Cause Analysis: GraphDB Pipeline Hang

## 1. Incident Summary
On March 10, 2026, the `graphdb` pipeline was initiated using the `build-all` command to generate a Code Property Graph (CPG) from scratch. The pipeline successfully completed Ingestion (Phase 1), Neo4j Import (Phase 2), and the heavy semantic clustering of Phase 3 (Enriching Features). 

However, at `03:35:56` on March 11, the process hung indefinitely while attempting to write the computed feature clusters back to the remote Neo4j database (hosted at `10.0.0.195`). The process remained deadlocked for approximately 14 hours until manually terminated.

## 2. Context and Timeline
* **Target Database:** Remote Neo4j instance at `bolt://10.0.0.195:7687`
* **Scale of Data:** 31,009 files, 2.70 GB of structural data.
* **Clustering Scale:** 227,933 function embeddings were successfully clustered.
* **Point of Failure:** The log ended abruptly with the following entry:
  `2026/03/11 03:35:56 Writing 44360 feature nodes and 272143 edges to database...`

## 3. System State at Time of Intervention
* **Process Status:** The `graphdb-win.exe` process was active but consuming exactly `0` CPU cycles over measured intervals. It was in a pure I/O Wait/Deadlock state.
* **Network Status:** A TCP connection test to `10.0.0.195:7687` succeeded, proving the remote VM and Neo4j service were still online and reachable.
* **Memory/CPU:** The process was holding memory (~371 MB) but doing no processing.

## 4. Root Cause Analysis
The deadlock is a classic symptom of a **TCP Half-Open Connection** or a **Silent Transaction Timeout** between the Go Bolt driver and the Neo4j server during a massive, long-running transaction.

**Detailed Breakdown:**
1. **Massive Transaction Size:** The tool attempted to write 44,360 nodes and 272,143 edges in what appears to be a single batch or a continuously streaming transaction.
2. **Neo4j Server Constraints:** When receiving a transaction of this magnitude, the Neo4j server must hold the entire transaction state in its heap memory before committing. If the server experienced heavy GC (Garbage Collection) pauses, or if the transaction took too long to process, the server may have silently dropped the connection, or a firewall/router between the host and `10.0.0.195` may have culled the idle TCP connection due to lack of keepalive packets during the long processing window.
3. **Driver Deadlock:** The Go Neo4j Bolt driver sent the data and was waiting for a `SUCCESS` acknowledgement from the server. Because the connection was dropped silently (no `RST` packet received by the client), the Go network socket remained open indefinitely, waiting for a response that would never come. This resulted in the 0% CPU usage deadlock.

## 5. Impact
Because the database write operation hung before completion, the "Intent Layer" (Feature nodes and edges) in the Neo4j database is in an inconsistent state. Neo4j transactions are ACID compliant, meaning if this was a single massive transaction, it was automatically rolled back by the server when the connection died. If it was written in smaller batches, the graph is partially enriched.

## 6. Recommendations & Next Steps
1. **Neo4j Server Logs:** Check the `neo4j.log` and `debug.log` on the `10.0.0.195` VM around `03:35 AM` to confirm if the server threw an `OutOfMemoryError` or forcefully terminated the transaction.
2. **Batching:** The `graphdb` tool may need to chunk its write operations for the Intent Layer. While extraction and embedding use `-batch-size`, the final write step appears to be executing synchronously en masse.
3. **Recovery:** The pipeline should be restarted. Since the structural data is massive and already ingested into intermediate files (if not cleaned up) or partially in the DB, running `build-all -clean` is recommended to ensure no corrupted or orphaned relationships remain.