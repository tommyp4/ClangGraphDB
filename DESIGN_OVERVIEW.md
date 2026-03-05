# The GraphDB Mental Model: A Guide for UI/UX Design

At its core, the GraphDB is a tool for **Software Architecture Archeology**. It takes a massive, tangled web of legacy code and maps it out so engineers can understand it, untangle it, and modernize it. 

It does this by operating on two distinct "Layers" that the UI needs to visualize and bridge:

## 1. The Physical Layer (What the code *is*)
This is the literal, structural reality of the code on the hard drive. 

*   **The Entities (Nodes):**
    *   📄 **Files:** The physical text files on disk.
    *   📦 **Classes/Structs:** The containers of logic.
    *   ⚙️ **Functions/Methods:** The actual blocks of executable code.
    *   🌍 **Global State/Variables:** Data that lives outside of functions (often the source of bugs and tangled dependencies).
*   **The Relationships (Edges):**
    *   `CALLS`: Function A executes Function B.
    *   `HAS_METHOD`: Class A owns Function B.
    *   `USES_GLOBAL`: Function A modifies Global Variable Z.
    *   `INHERITS`: Class A extends Class B.

*🎨 **Design Translation:** The Physical layer is rigid, hierarchical, and often messy. Visualizing this looks like a traditional dependency tree or network graph. It's about lines connecting boxes.*

---

## 2. The Semantic "Intent" Layer (What the code *means*)
This is where the AI/LLM magic happens. The database reads the code and groups it by *purpose*, regardless of which file or folder it lives in.

*   **The Entities (Nodes):**
    *   🧠 **Domains / Categories:** High-level business concepts (e.g., "E-Commerce", "User Management").
    *   ✨ **Features:** Specific capabilities (e.g., "Password Reset", "Calculate Shopping Cart Tax").
*   **The Relationships (Edges):**
    *   `PARENT_OF`: "User Management" contains "Password Reset".
    *   `IMPLEMENTS`: *This is the magic bridge.* It links a physical **Function** to a semantic **Feature**.

*🎨 **Design Translation:** The Intent layer is fluid and conceptual. Visualizing this looks like clustered bubbles, Venn diagrams, or grouped regions. It allows a user to search for "How does billing work?" and see a cluster of code light up, even if that code is scattered across 50 different files.*

---

## 3. Key UI Use Cases to Mock Up

When designing the interface (like the D3 Visualizer), the designer should think about these 4 core interactions:

### A. Semantic Search & Discovery
*   **The Action:** A user types a natural language query into a search bar: *"Where do we validate credit cards?"*
*   **The Visual:** The graph shouldn't just show a list of files. It should pan/zoom to a **Semantic Cluster** (a grouped bubble of nodes) and highlight the specific functions that execute that intent.

### B. Blast Radius (Impact Analysis)
*   **The Action:** A user selects a specific Function or Database connection and asks: *"If I change this, what else breaks?"*
*   **The Visual:** The selected node glows red. A ripple effect or highlighted path traces *upwards* through the `CALLS` edges, showing every other feature and function that relies on the selected node.

### C. The "Pinch Point" (Bottleneck Discovery)
*   **The Action:** The user wants to see architectural flaws.
*   **The Visual:** The UI highlights "Pinch Points"—nodes that have massive numbers of incoming arrows (everyone relies on them) and outgoing arrows (they rely on everything). These nodes should visually stand out (larger size, warning colors) because they represent high risk.

### D. The "Strangler Fig" (Extraction Planning)
*   **The Action:** A user wants to rip a feature (like "Billing") out of the legacy codebase and move it to a new microservice.
*   **The Visual:** The user selects a Semantic Domain bubble. The UI highlights the `IMPLEMENTS` edges connecting to the physical code, and explicitly flags the "Seams"—the exact `CALLS` edges that cross the boundary between the "Billing" bubble and the rest of the application. These are the wires the engineer will have to cut.
