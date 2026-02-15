# Plan: Add end_line property to graph nodes

## Goal
Update C#, Java, TypeScript, C++, and SQL parsers to include `end_line` in the node properties.

## Steps
- [x] Verify existing tests pass for all 5 languages
- [x] C# (`internal/analysis/csharp.go`)
    - [x] Update test `internal/analysis/csharp_test.go` to assert `end_line`
    - [x] Update implementation to add `end_line` and remove `content` if present
    - [x] Verify test passes
- [x] Java (`internal/analysis/java.go`)
    - [x] Update test `internal/analysis/java_test.go` to assert `end_line`
    - [x] Update implementation to add `end_line` and remove `content` if present
    - [x] Verify test passes
- [x] TypeScript (`internal/analysis/typescript.go`)
    - [x] Update test `internal/analysis/typescript_test.go` to assert `end_line`
    - [x] Update implementation to add `end_line` and remove `content` if present
    - [x] Verify test passes
- [x] C++ (`internal/analysis/cpp.go`)
    - [x] Update test `internal/analysis/cpp_test.go` to assert `end_line`
    - [x] Update implementation to add `end_line` and remove `content` if present
    - [x] Verify test passes
- [x] SQL (`internal/analysis/sql.go`)
    - [x] Update test `internal/analysis/sql_test.go` to assert `end_line`
    - [x] Update implementation to add `end_line` and remove `content` if present
    - [x] Verify test passes
