# Release recipes

[private]
[no-cd]
default:
    @go run ./internal/releaseutil help

goreleaser := "go run github.com/goreleaser/goreleaser/v2@v2.15.2"
releaseutil := "go run ./internal/releaseutil"

# Validate GoReleaser configuration
[no-cd]
check:
    {{goreleaser}} check

# Build a local snapshot release
[no-cd]
snapshot:
    {{goreleaser}} release --snapshot --clean

# Show current version
[no-cd]
version:
    @go run ./cmd/camp-graph version

# Show current stable release tag
[no-cd]
current:
    @{{releaseutil}} current

# Create and push the next stable release tag (usage: just release stable [patch|minor|major])
[no-cd]
stable level="patch":
    {{releaseutil}} stable --level {{level}}

# Create and push an explicit release tag (usage: just release tag v0.1.0)
[no-cd]
tag VERSION:
    {{releaseutil}} tag --version {{VERSION}}
