# Feature Implementation Plan: Semantic Clustering Progress Bar Improvement

## 📋 Todo Checklist
- [ ] 1. **`internal/ui/progressbar.go`**: Add `UpdateDescription` method.
- [ ] 2. **`internal/rpg/builder.go`**: Refactor callbacks to `OnStepStart` and `OnStepEnd`.
- [ ] 3. **`cmd/graphdb/main.go`**: Update usage in `handleEnrichFeatures`.
- [ ] 4. **Verification**: Run tests and build.

## 🔍 Context
The previous implementation of the semantic clustering progress bar was a good start, but it lacked granularity and reliability. The user wants to see exactly which domain is being processed and ensure the progress bar updates correctly.

## 📝 Implementation Details

### 1. `internal/ui/progressbar.go`
*   Add `UpdateDescription(text string)` method to `ProgressBar`.
*   Ensure `UpdateDescription` acquires the lock and allows subsequent renders to use the new text.

### 2. `internal/rpg/builder.go`
*   Change `OnPhaseStep` field to `OnStepStart func(stepName string)` and `OnStepEnd func(stepName string)`.
*   In `Build` method:
    *   Call `OnStepStart(name)` *before* processing the domain.
    *   Call `OnStepEnd(name)` *after* processing the domain.

### 3. `cmd/graphdb/main.go`
*   Update `handleEnrichFeatures`:
    *   In `OnPhaseStart`:
        *   Create `ui.NewProgressBar`.
        *   Call `clusterPb.Add(0)` immediately to render the "0/N" state.
    *   Implement `OnStepStart`:
        *   Call `clusterPb.UpdateDescription(fmt.Sprintf("Clustering %s", stepName))`.
    *   Implement `OnStepEnd`:
        *   Call `clusterPb.Add(1)`.

### 4. Verification
*   Run `go test ./internal/rpg/...`.
*   Run `go build ./cmd/graphdb`.
