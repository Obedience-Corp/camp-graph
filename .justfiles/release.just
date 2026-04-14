# Release recipes

[private]
default:
    #!/usr/bin/env bash
    echo "camp-graph release commands"
    echo ""
    echo "Primary:"
    echo "  just release stable [patch|minor|major]"
    echo ""
    echo "Support:"
    echo "  just release current"
    echo "  just release check"
    echo "  just release snapshot"
    echo "  just release tag v0.1.0"

goreleaser := "go run github.com/goreleaser/goreleaser/v2@v2.15.2"
repo := "Obedience-Corp/camp-graph"

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
    #!/usr/bin/env bash
    set -euo pipefail
    git fetch --prune --prune-tags origin '+refs/tags/*:refs/tags/*' >/dev/null 2>&1 || true
    latest=$(git tag -l --sort=-version:refname | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' | head -1 || true)
    echo "${latest:-none}"

# Create and push the next stable release tag (usage: just release stable [patch|minor|major])
[no-cd]
stable level="patch":
    #!/usr/bin/env bash
    set -euo pipefail

    level="{{level}}"
    case "$level" in
        patch|minor|major) ;;
        *)
            echo "Unknown level: $level (use patch, minor, or major)"
            exit 1
            ;;
    esac

    if [ -n "$(git status --porcelain)" ]; then
        echo "ERROR: repo has uncommitted changes"
        git status --short
        exit 1
    fi

    branch="$(git rev-parse --abbrev-ref HEAD)"
    if [ "$branch" != "main" ]; then
        echo "ERROR: releases must be cut from main (current: $branch)"
        exit 1
    fi

    git fetch --prune --prune-tags origin '+refs/tags/*:refs/tags/*' '+refs/heads/main:refs/remotes/origin/main'

    if [ "$(git rev-parse HEAD)" != "$(git rev-parse origin/main)" ]; then
        echo "ERROR: local main is not in sync with origin/main"
        echo "Run: git pull --ff-only origin main"
        exit 1
    fi

    just --justfile {{source_file()}} check

    latest=$(git tag -l --sort=-version:refname | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' | head -1 || true)

    if [ -z "$latest" ]; then
        next="v0.1.0"
    else
        nums="${latest#v}"
        IFS='.' read -r major minor patch <<< "$nums"

        case "$level" in
            major) major=$((major + 1)); minor=0; patch=0 ;;
            minor) minor=$((minor + 1)); patch=0 ;;
            patch) patch=$((patch + 1)) ;;
        esac

        next="v${major}.${minor}.${patch}"
    fi

    if git tag -l "$next" | grep -q .; then
        echo "ERROR: tag $next already exists locally"
        exit 1
    fi

    if [ -n "$(git ls-remote --tags origin "refs/tags/${next}")" ]; then
        echo "ERROR: tag $next already exists on origin"
        exit 1
    fi

    echo "Creating stable release: $next"
    git tag -a "$next" -m "Release $next"
    git push origin "$next"
    echo "Pushed $next"
    echo "Release workflow should now start on GitHub."

# Create and push an explicit release tag (usage: just release tag v0.1.0)
[no-cd]
tag VERSION:
    #!/usr/bin/env bash
    set -euo pipefail
    if [[ "{{VERSION}}" != v* ]]; then
        echo "Version must start with 'v' (e.g., v0.1.0)"
        exit 1
    fi
    if [ -n "$(git status --porcelain)" ]; then
        echo "ERROR: repo has uncommitted changes"
        git status --short
        exit 1
    fi
    just --justfile {{source_file()}} check
    echo "Creating release tag {{VERSION}}..."
    git tag -a "{{VERSION}}" -m "Release {{VERSION}}"
    git push origin "{{VERSION}}"
    echo "Pushed {{VERSION}}"
    echo "Release workflow should now start on GitHub."
