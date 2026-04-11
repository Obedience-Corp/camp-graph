# camp-graph

Knowledge graph visualization plugin for [camp](https://github.com/Obedience-Corp/camp).

Builds and visualizes knowledge graphs from campaign artifacts — projects, festivals, intents, design docs, chains, and code.

## Install

### Homebrew

```bash
brew install --cask Obedience-Corp/tap/camp-graph
```

### Arch Linux

```bash
yay -S camp-graph-bin
```

### Manual installer

```bash
curl -fsSL https://raw.githubusercontent.com/Obedience-Corp/camp-graph/main/install.sh | bash
```

### From source

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
just test unit
just lint     # Format + vet
just install  # Install to $GOBIN
just release check
```

## Intent Graph

Intents are raw ideas — observations, feature requests, research topics, or maintenance chores captured before they're formalized. In the knowledge graph, intents are first-class nodes that connect to everything they eventually become.

### How Intents Feed the System

- **Workflow items** — An intent triaged as urgent becomes a workflow task for immediate action
- **Design documents** — An intent that needs deeper exploration spawns a design doc in `workflow/design/`
- **Festivals** — An intent (or group of related intents) ready for structured execution gets promoted to a festival
- **Other intents** — Intents can reference each other, forming clusters around a common theme

### Intent Lifecycle

```
capture → triage → promote → track
```

1. **Capture** — Record the raw idea with minimal friction (`camp intent add`)
2. **Triage** — Evaluate priority, feasibility, and category (`inbox/` → `active/`)
3. **Promote** — Convert to a workflow item, design doc, or festival (`active/` → `ready/` → promoted)
4. **Track** — The graph links the original intent to its promoted artifact, so you can trace any deliverable back to the idea that started it

### Graph Representation

```bash
camp graph query "dark-mode"     # Find the intent node and all connected artifacts
camp graph context INTENT-001    # Show relationships: which festival, design doc, or workflow item it became
```

Intents appear as nodes with edges to their promoted artifacts. This makes the knowledge graph a complete record of how ideas flow through the system — from first capture to final delivery.

## Architecture

`camp-graph` is a **separate binary** that plugs into camp via the git-style plugin pattern. When you run `camp graph`, camp discovers `camp-graph` on `$PATH` and delegates to it. Zero coupling, independent release cycles.

See `workflow/explore/knowledge_graph/` in obey-campaign for full research and design docs.

## Release process

Tagged releases publish GitHub assets plus package-manager updates for
Homebrew and AUR. See [docs/releasing.md](docs/releasing.md) for the
one-time setup and the reusable pattern for future `camp-*` plugin repos.
