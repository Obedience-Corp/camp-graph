# Release recipes
project_root := justfile_directory()

[private]
default:
    @cd "{{project_root}}" && go run ./internal/releaseutil help

goreleaser := "go run github.com/goreleaser/goreleaser/v2@v2.15.2"
releaseutil := "go run ./internal/releaseutil"

# Validate GoReleaser configuration
check:
    cd "{{project_root}}" && {{goreleaser}} check

# Build a local snapshot release
snapshot:
    cd "{{project_root}}" && {{goreleaser}} release --snapshot --clean

# Show current version
version:
    @cd "{{project_root}}" && go run ./cmd/camp-graph version

# Show current stable release tag
current:
    @cd "{{project_root}}" && {{releaseutil}} current

# Create and push the next stable release tag (usage: just release stable [patch|minor|major])
stable level="patch":
    cd "{{project_root}}" && {{releaseutil}} stable --level {{level}}

# Create and push an explicit release tag (usage: just release tag v0.1.0)
tag VERSION:
    cd "{{project_root}}" && {{releaseutil}} tag --version {{VERSION}}
