# Architectural Seams Research Report
**Date:** March 2, 2026
**Source:** GraphDB Analysis (Neo4j Code Property Graph)

## Executive Summary
Based on structural dependency mapping and risk analysis, the primary target for modernization and decoupling is the **Domain-Infrastructure Seam** within the `DriverSettlementFactory`. The current architecture exhibits high coupling between domain logic and data access, specifically within factory and manager classes.

---

## Identified Seams for Modernization

### 1. The Domain-Infrastructure Seam: `DriverSettlementFactory`
*   **Location:** `trucks/Trucks.App/Service/DriverSettlementFactory.cs`
*   **Analysis:** This class functions as a domain factory but is heavily "contaminated" by infrastructure. It directly depends on five distinct repositories (`IDriverRepository`, `ISettlementRepository`, `IDriverSettlementRepository`, `IFuelRepository`, and `ITollRepository`).
*   **Modernization Goal:** Transition to a **Pure Domain Factory**. Data-fetching logic should be moved to an Application Service/Use Case. The factory should accept hydrated domain objects or primitives, making it 100% testable without database mocks.

### 2. The Orchestration Seam: `SettlementManager`
*   **Location:** `trucks/Trucks.App/SettlementManager.cs`
*   **Analysis:** Identified as a "God Service." It bridges UI Controllers directly to nearly every infrastructure component (Repositories, File Systems, Toll Clients, Parsers). It violates the Single Responsibility Principle by mixing orchestration, I/O, and business rules.
*   **Modernization Goal:** Apply the **Gather-Calculate-Scatter** pattern. Decompose this manager into focused Command Handlers.

### 3. The Controller-Repository Leak: `ISettlementRepository`
*   **Location:** `trucks/Trucks.Core/ISettlementRepository.cs`
*   **Analysis:** This interface is directly injected into UI Controllers (`DriverController`, `SettlementsController`). This indicates that the presentation layer is manipulating database entities directly, bypassing business boundaries.
*   **Modernization Goal:** Introduce an **Application Layer** (Use Cases) to decouple Controllers from Repositories.

---

## Methodology: Step-by-Step GraphDB Analysis

The following sequence of `graphdb` commands was used to derive these conclusions:

### Phase 1: Global Vulnerability Scan
First, we attempted to find high-level architectural hotspots and automated seams.
1.  **Seam Discovery:** `graphdb query -type seams -layer all`
    *   *Finding:* Returned empty, indicating diffuse contamination (UI calling DB directly across the board) rather than isolated, clean boundaries.
2.  **Hotspot Identification:** `graphdb query -type hotspots -module "^server|trucks"`
    *   *Finding:* Scoped hotspots were low, shifting the focus from "frequently changed code" to "structurally bottlenecked code."

### Phase 2: Structural Dependency Mapping
We pivoted to investigating core domain concepts to find where the architecture was "pinched."
3.  **Repository Surface Area:** `graphdb query -type neighbors -target "ISettlementRepository" -depth 1`
    *   *Finding:* Revealed 100+ dependencies, proving `ISettlementRepository` is a massive dependency leak into the UI and Helper layers.
4.  **Manager Complexity:** `graphdb query -type hybrid-context -target "SettlementManager" -depth 1`
    *   *Finding:* Confirmed `SettlementManager` as the central orchestrator coupled to all infrastructure.
5.  **Factory Contamination:** `graphdb query -type hybrid-context -target "DriverSettlementFactory" -depth 1`
    *   *Finding:* The "smoking gun." The constructor revealed it was fetching its own state via five different repositories, making it the ideal candidate for domain decoupling.

### Phase 3: Validation & Impact Assessment
Finally, we verified if refactoring the Factory was safe.
6.  **Impact Analysis:** `graphdb query -type impact -target "DriverSettlementFactory" -depth 1`
    *   *Finding:* Limited callers (`CreateDriverSettlementSplit`). This confirmed the Factory is an isolated seam that can be refactored with minimal regression risk to the rest of the application.
7.  **What-If Simulation:** `graphdb query -type what-if -target "DriverSettlementFactory" -target2 "ISettlementRepository"`
    *   *Finding:* No hidden cross-boundary calls were identified, validating the feasibility of the decoupling.

---

## Recommended Starting Point
**Target:** `DriverSettlementFactory`
**Rationale:** High domain value, low structural resistance, and clear path to a "Pure Function" refactor.
