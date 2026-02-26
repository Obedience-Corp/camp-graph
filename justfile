#!/usr/bin/env just --justfile
# camp-graph - Knowledge graph visualization for campaigns

set dotenv-load := true

# Configuration
binary_name := "camp-graph"
bin_dir := "bin"
gobin := env_var_or_default("GOBIN", `go env GOPATH` + "/bin")
version_pkg := "github.com/Obedience-Corp/camp-graph/internal/version"
version := env_var_or_default("VERSION", "dev")
commit := `git rev-parse --short HEAD 2>/dev/null || echo "unknown"`
build_date := `date -u +"%Y-%m-%dT%H:%M:%SZ"`
ldflags := "-X " + version_pkg + ".Version=" + version + " -X " + version_pkg + ".Commit=" + commit + " -X " + version_pkg + ".BuildDate=" + build_date

# Modules
[doc('Cross-platform builds')]
mod xbuild '.justfiles/build.just'

[doc('Testing (unit, coverage, benchmarks)')]
mod test '.justfiles/test.just'

[doc('Release and versioning')]
mod release '.justfiles/release.just'

[private]
default:
    @echo "camp-graph - Knowledge Graph Visualization for Campaigns"
    @echo ""
    @just --list --unsorted

# Build camp-graph binary
build:
    @echo "Building camp-graph..."
    @mkdir -p {{bin_dir}}
    go build -ldflags '{{ldflags}}' -o {{bin_dir}}/{{binary_name}} ./cmd/camp-graph
    @echo "Built {{bin_dir}}/{{binary_name}}"

# Format Go code
fmt:
    go fmt ./...

# Run go vet
vet:
    go vet ./...

# Run formatting and vetting
lint: fmt vet
    @echo "Linting complete"

# Clean build artifacts
clean:
    rm -rf {{bin_dir}}
    rm -f coverage.out coverage.html
    @echo "Cleaned build artifacts"

# Update and tidy dependencies
deps:
    go get -u ./...
    go mod tidy

# Tidy dependencies
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
