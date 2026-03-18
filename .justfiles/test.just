# Test recipes

[private]
default:
    @just --list --unsorted --justfile {{source_file()}}

# Run all tests
[no-cd]
all:
    go test -race -cover ./...

# Run unit tests only
[no-cd]
unit:
    go test -short -timeout 30s ./...

# Run tests with verbose output
[no-cd]
verbose:
    go test -race -cover -v ./...

# Run tests with coverage report
[no-cd]
coverage:
    @mkdir -p out
    go test -race -coverprofile=out/coverage.out ./...
    go tool cover -html=out/coverage.out -o out/coverage.html
    @echo "Coverage report: out/coverage.html"

# Run short tests only
[no-cd]
short:
    go test -short ./...

# Run benchmarks
[no-cd]
bench:
    go test -bench=. -benchmem ./...

# ============================================
# Integration Tests (require Docker/Colima)
# ============================================

# Run integration tests with verbose output
[no-cd]
integration:
    #!/usr/bin/env bash
    set -euo pipefail
    echo "Building Linux binary for containers..."
    mkdir -p bin/linux
    GOOS=linux GOARCH=$(go env GOARCH) go build -o bin/linux/camp-graph ./cmd/camp-graph
    echo "Running container-based integration tests..."
    DOCKER_HOST="unix://${HOME}/.colima/default/docker.sock" \
    TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE=/var/run/docker.sock \
    go test -v -count=1 -tags=integration ./tests/integration/... -timeout 5m 2>&1

# Run a specific integration test
[no-cd]
integration-run TEST:
    #!/usr/bin/env bash
    set -euo pipefail
    echo "Building Linux binary for containers..."
    mkdir -p bin/linux
    GOOS=linux GOARCH=$(go env GOARCH) go build -o bin/linux/camp-graph ./cmd/camp-graph
    echo "Running integration test: {{TEST}}"
    DOCKER_HOST="unix://${HOME}/.colima/default/docker.sock" \
    TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE=/var/run/docker.sock \
    go test -v -count=1 -tags=integration ./tests/integration/... -run "{{TEST}}" -timeout 5m

# Build Linux binary for integration tests (useful for debugging)
[no-cd]
integration-build:
    @echo "Building Linux binary for containers..."
    @mkdir -p bin/linux
    GOOS=linux GOARCH=$(go env GOARCH) go build -o bin/linux/camp-graph ./cmd/camp-graph
    @echo "Built bin/linux/camp-graph"
