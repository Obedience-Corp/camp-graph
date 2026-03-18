#!/usr/bin/env just --justfile
# camp-graph - Knowledge graph visualization for campaigns

set dotenv-load := true

# Configuration
BUILDTOOL := "go run ./internal/buildutil"
binary_name := "camp-graph"
bin_dir := "bin"
gobin := env_var_or_default("GOBIN", `go env GOPATH` + "/bin")

# Modules
[doc('Cross-platform builds')]
mod xbuild '.justfiles/build.just'

[doc('Testing (unit, coverage, benchmarks, integration)')]
mod test '.justfiles/test.just'

[doc('Release and versioning')]
mod release '.justfiles/release.just'

[private]
default:
    @echo "camp-graph - Knowledge Graph Visualization for Campaigns"
    @echo ""
    @just --list --unsorted

# Build camp-graph binary (with dashboard)
[no-cd]
build:
    @{{BUILDTOOL}} build

# Build binary only (fast, no vet)
[no-cd]
build-only:
    @{{BUILDTOOL}} build-only

# Format Go code
[no-cd]
fmt:
    go fmt ./...

# Run go vet
[no-cd]
vet:
    go vet ./...

# Run formatting and vetting
[no-cd]
lint: fmt vet
    @echo "Linting complete"

# Clean build artifacts
[no-cd]
clean:
    @{{BUILDTOOL}} clean

# Update and tidy dependencies
[no-cd]
deps:
    go get -u ./...
    go mod tidy

# Tidy dependencies
[no-cd]
tidy:
    go mod tidy

# Install camp-graph to $GOBIN (makes it discoverable by camp)
install: build
    #!/usr/bin/env bash
    set -euo pipefail
    echo "Installing camp-graph..."
    mkdir -p {{gobin}}
    cp {{bin_dir}}/{{binary_name}} {{gobin}}/{{binary_name}}
    if [[ "$(uname)" == "Darwin" ]]; then
        codesign -f -s - {{gobin}}/{{binary_name}} 2>/dev/null || true
    fi
    echo "camp-graph installed to {{gobin}}/{{binary_name}}"
    echo "  camp will now discover 'camp graph' automatically"

# Uninstall camp-graph from $GOBIN
uninstall:
    @echo "Uninstalling camp-graph..."
    @rm -f {{gobin}}/{{binary_name}}
    @echo "camp-graph uninstalled"

# Run camp-graph (for development)
run *ARGS:
    go run ./cmd/camp-graph {{ARGS}}
