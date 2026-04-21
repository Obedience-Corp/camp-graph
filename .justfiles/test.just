# Test recipes
project_root := source_directory() / ".."
BUILDTOOL := "bin/.buildutil"

[private]
default:
    @just --list --unsorted --justfile {{source_file()}}

# Ensure buildutil binary is compiled
[private]
_buildtool:
    #!/usr/bin/env bash
    set -euo pipefail
    cd "{{project_root}}"
    test -f {{BUILDTOOL}} || (mkdir -p bin && go build -o {{BUILDTOOL}} ./internal/buildutil)

# Run all tests (unit + integration) with dashboard
all: _buildtool
    @cd "{{project_root}}" && {{BUILDTOOL}} test
    @cd "{{project_root}}" && {{BUILDTOOL}} integration

# Run unit tests with dashboard
unit: _buildtool
    @cd "{{project_root}}" && {{BUILDTOOL}} test

# Run tests with verbose output (standard go test)
verbose:
    cd "{{project_root}}" && go test -race -cover -v ./...

# Run tests with coverage report
coverage:
    @cd "{{project_root}}" && mkdir -p out
    cd "{{project_root}}" && go test -race -coverprofile=out/coverage.out ./...
    cd "{{project_root}}" && go tool cover -html=out/coverage.out -o out/coverage.html
    @echo "Coverage report: out/coverage.html"

# Run short tests only
short:
    cd "{{project_root}}" && go test -short ./...

# Run benchmarks
bench:
    cd "{{project_root}}" && go test -bench=. -benchmem ./...

# ============================================
# Integration Tests (require Docker/Colima)
# ============================================

# Run integration tests with dashboard
integration: _buildtool
    @cd "{{project_root}}" && {{BUILDTOOL}} integration

# Run integration tests with verbose output (standard go test)
integration-verbose:
    #!/usr/bin/env bash
    set -euo pipefail
    cd "{{project_root}}"
    echo "Building Linux binary for containers..."
    mkdir -p bin/linux
    GOOS=linux GOARCH=$(go env GOARCH) go build -o bin/linux/camp-graph ./cmd/camp-graph
    echo "Running container-based integration tests (verbose)..."
    DOCKER_HOST="unix://${HOME}/.colima/default/docker.sock" \
    TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE=/var/run/docker.sock \
    go test -v -count=1 -tags=integration ./tests/integration/... -timeout 5m 2>&1

# Run a specific integration test
integration-run TEST:
    #!/usr/bin/env bash
    set -euo pipefail
    cd "{{project_root}}"
    echo "Building Linux binary for containers..."
    mkdir -p bin/linux
    GOOS=linux GOARCH=$(go env GOARCH) go build -o bin/linux/camp-graph ./cmd/camp-graph
    echo "Running integration test: {{TEST}}"
    DOCKER_HOST="unix://${HOME}/.colima/default/docker.sock" \
    TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE=/var/run/docker.sock \
    go test -v -count=1 -tags=integration ./tests/integration/... -run "{{TEST}}" -timeout 5m

# Build Linux binary for integration tests (useful for debugging)
integration-build:
    #!/usr/bin/env bash
    set -euo pipefail
    cd "{{project_root}}"
    echo "Building Linux binary for containers..."
    mkdir -p bin/linux
    GOOS=linux GOARCH=$(go env GOARCH) go build -o bin/linux/camp-graph ./cmd/camp-graph
    echo "Built bin/linux/camp-graph"
