# Release recipes

[private]
default:
    @cd .. && go run ./internal/releaseutil help

goreleaser := "go run github.com/goreleaser/goreleaser/v2@v2.15.2"
releaseutil := "go run ./internal/releaseutil"

# Validate GoReleaser configuration
check:
    cd .. && {{goreleaser}} check

# Build a local snapshot release
snapshot:
    cd .. && {{goreleaser}} release --snapshot --clean

# Show current version
version:
    @cd .. && go run ./cmd/camp-graph version

# Show current stable release tag
current:
    @cd .. && {{releaseutil}} current

# Create and push the next stable release tag (usage: just release stable [patch|minor|major])
stable level="patch":
    cd .. && {{releaseutil}} stable --level {{level}}

# Create and push an explicit release tag (usage: just release tag v0.1.0)
tag VERSION:
    cd .. && {{releaseutil}} tag --version {{VERSION}}
