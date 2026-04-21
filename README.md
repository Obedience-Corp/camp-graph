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

### Manual install from GitHub Releases

Download the matching archive from the
[GitHub Releases page](https://github.com/Obedience-Corp/camp-graph/releases),
extract `camp-graph`, and place it on your `PATH`.

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
# Build graph from campaign filesystem
camp graph build

# Content-backed lexical search (FTS5) with scope + mode filters
camp graph query "job search" --scope "Work/JobSearch" --limit 10 --json
camp graph query "auth" --type project --mode hybrid

# Workitem enrichment - scope-first related items with reasons and scores
camp graph related --path "Work/JobSearch/Action Plan.md" --limit 10 --json

# Incremental refresh (falls back to full rebuild on schema drift)
camp graph refresh --json

# Machine-readable status for freshness and FTS availability
camp graph status --json

# FTS-backed live TUI browser with chips, scope picker, and preview pane
camp graph browse

# Focused neighborhood and rendering
camp graph context HF0001
camp graph render -f svg
camp graph render --scope "Work/JobSearch" --mode structural -f dot
camp graph render -f html -o graph.html
```

### JSON envelopes

Machine-readable commands emit stable `schema_version` tags so downstream
tools can version-gate their parsers:

- `query --json` → `graph-query/v1alpha1`
- `related --json` → `graph-related/v1alpha1`
- `refresh --json` → `graph-refresh/v1alpha1`
- `status --json` → `graph-status/v1alpha1`

Graph-database schema: `graphdb/v2alpha1`. A mismatch forces `refresh`
to fall back to a full rebuild.

## TUI browser (`camp-graph browse`)

`camp-graph browse` is an FTS-backed terminal UI. With an empty query
it opens the scope-anchor explorer; as soon as you type, cycle a
chip, or pick a scope, the input routes through `internal/search`
(`Querier.Search`) as a live query with per-keystroke generation
cancellation so only the latest result set lands in the list.

### Filters

Three filter chips sit above the list:

- **Type** (`t`) narrows to a single NodeType (project, festival,
  task, intent, file, ...). Default is `All`.
- **Tracked** (`s`) toggles between `All`, `Tracked only`, and
  `Untracked only` (maps to `Querier.QueryOptions.Tracked` /
  `Untracked`).
- **Mode** (`m`) selects `hybrid`, `structural`, `explicit`, or
  `semantic` ranking.

A modal scope picker (`c`) restricts results to a path prefix; `C`
clears the scope. Non-default chip values and the active scope
render as pills in an active-filters row under the chip bar.

### Preview pane

The right pane shows node details for the row under the cursor:
title + `[type]` header, path, optional status, outgoing edges
(capped at 50, overflow collapses to `... +N more`), incoming edges
(same cap), and the top 3 related items. Cursor moves cancel any
in-flight fetch and issue a new one; `tab` toggles focus between
list and preview so `j`/`k` scroll the pane without moving the list
cursor.

### Layout

Three breakpoints adapt to terminal width:

- **narrow** (`< 80` columns): preview pane hidden, list fills the
  screen, row scope column suppressed.
- **normal** (`80`-`120`): 60/40 list/preview split.
- **wide** (`> 120`): 50/50 list/preview split.

### Help

`?` toggles a full-screen help overlay listing every binding;
`esc` or a second `?` restores the prior focus.

For the complete keybinding table see the `UX_SPEC.md` referenced
by the festival plan (`festivals/.../002_PLAN/UX_SPEC.md`).

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
