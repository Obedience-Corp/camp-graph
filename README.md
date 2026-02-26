# camp-graph

Knowledge graph visualization plugin for [camp](https://github.com/Obedience-Corp/camp).

Builds and visualizes knowledge graphs from campaign artifacts — projects, festivals, intents, design docs, chains, and code.

## Install

```bash
go install github.com/Obedience-Corp/camp-graph/cmd/camp-graph@latest
```

Or with just:

```bash
just install
```

Once installed on `$PATH`, camp discovers it automatically:

```bash
camp graph build              # Build graph from campaign filesystem
camp graph browse             # TUI graph browser
camp graph query "auth"       # Search nodes
camp graph context HF0001     # Show relationships for a festival
camp graph render --svg       # Static graph image
```

## Development

```bash
just          # Show available commands
just build    # Build binary
just test all # Run tests
just lint     # Format + vet
just install  # Install to $GOBIN
```

## Architecture

`camp-graph` is a **separate binary** that plugs into camp via the git-style plugin pattern. When you run `camp graph`, camp discovers `camp-graph` on `$PATH` and delegates to it. Zero coupling, independent release cycles.

See `workflow/explore/knowledge_graph/` in obey-campaign for full research and design docs.
