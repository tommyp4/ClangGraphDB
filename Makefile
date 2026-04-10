.PHONY: build build-linux build-windows build-mocks test

VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.Version=$(VERSION)"

# Ensure local zig is in PATH for cross-compilation
LOCAL_ZIG := $(abspath .gemini/tools/zig-linux-x86_64-0.13.0)
export PATH := $(LOCAL_ZIG):$(PATH)

build: build-linux

build-all: build-linux build-windows

build-linux:
	@echo "Building for Linux (requires Zig)..."
	env CGO_ENABLED=1 \
    GOOS=linux \
    GOARCH=amd64 \
    CC="zig cc -target x86_64-linux-gnu.2.28" \
    CXX="zig c++ -target x86_64-linux-gnu.2.28" \
    go build $(LDFLAGS) -o .gemini/skills/graphdb/scripts/graphdb ./cmd/graphdb

build-windows:
	@echo "Building for Windows (requires Zig)..."
	env CGO_ENABLED=1 \
    GOOS=windows \
    GOARCH=amd64 \
    CC="zig cc -target x86_64-windows-gnu" \
    CXX="zig c++ -target x86_64-windows-gnu" \
    go build $(LDFLAGS) -o .gemini/skills/graphdb/scripts/graphdb-win.exe ./cmd/graphdb

build-mocks:
	go build -tags test_mocks -o .gemini/skills/graphdb/scripts/graphdb_test ./cmd/graphdb

test:
	go test -count=1 ./...

test-integration:
	@echo "Running integration tests (requires Neo4j)..."
	go test -count=1 -tags=integration ./internal/query/...
