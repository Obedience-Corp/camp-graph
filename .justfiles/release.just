# Release recipes

[private]
default:
    @just --list --unsorted --justfile {{source_file()}}

# Show current version
[no-cd]
version:
    @go run ./cmd/camp-graph version

# Tag a release (usage: just release tag v0.1.0)
[no-cd]
tag VERSION:
    #!/usr/bin/env bash
    set -euo pipefail
    if [[ "{{VERSION}}" != v* ]]; then
        echo "Version must start with 'v' (e.g., v0.1.0)"
        exit 1
    fi
    echo "Tagging {{VERSION}}..."
    git tag -a "{{VERSION}}" -m "Release {{VERSION}}"
    echo "Tagged {{VERSION}}. Push with: git push origin {{VERSION}}"
