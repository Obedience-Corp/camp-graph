// Package tui provides a BubbleTea-based terminal graph browser.
package tui

import (
	"context"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Obedience-Corp/camp-graph/internal/graph"
	"github.com/Obedience-Corp/camp-graph/internal/search"
	"github.com/Obedience-Corp/camp-graph/internal/tui/chips"
)

// chipBar groups the filter chips displayed above the result list.
type chipBar struct {
	Type    chips.Chip
	Tracked chips.Chip
	Mode    chips.Chip
}

// nodeTypeOptions is the authoritative list of NodeType strings the
// Type chip exposes. Kept in sync with internal/graph NodeType
// constants; UX order favors campaign artifacts above code types.
var nodeTypeOptions = []string{
	string(graph.NodeProject),
	string(graph.NodeFestival),
	string(graph.NodeChain),
	string(graph.NodePhase),
	string(graph.NodeSequence),
	string(graph.NodeTask),
	string(graph.NodeIntent),
	string(graph.NodeDesignDoc),
	string(graph.NodeExploreDoc),
	string(graph.NodeNote),
	string(graph.NodeCanvas),
	string(graph.NodeTag),
	string(graph.NodeAttachment),
	string(graph.NodeRepo),
	string(graph.NodeFolder),
	string(graph.NodePackage),
	string(graph.NodeTypeDef),
	string(graph.NodeFunction),
	string(graph.NodeFile),
}

type viewMode int

const (
	modeList viewMode = iota
	modeMicrograph
)

// focusMode identifies which UI element currently owns keyboard focus.
// focusSearch aliases the legacy m.searching bool so the two stay
// consistent; chip focus states route keys to the respective chip's
// Update method.
type focusMode int

const (
	focusList focusMode = iota
	focusSearch
	focusTypeChip
	focusTrackedChip
	focusModeChip
	focusScopePicker
	focusPreview
	focusHelp
)

type neighborEntry struct {
	node      *graph.Node
	edge      *graph.Edge
	direction string
}

// RelationMode controls which edge-source classes are shown in the
// micrograph and neighbor list. The default is RelationHybrid; pressing
// tab cycles hybrid -> structural -> explicit -> semantic.
type RelationMode int

const (
	RelationHybrid RelationMode = iota
	RelationStructural
	RelationExplicit
	RelationSemantic
)

// String returns the human-readable name of the relation mode.
func (r RelationMode) String() string {
	switch r {
	case RelationStructural:
		return "structural"
	case RelationExplicit:
		return "explicit"
	case RelationSemantic:
		return "semantic"
	default:
		return "hybrid"
	}
}

// Cycle returns the next relation mode in the tab order.
func (r RelationMode) Cycle() RelationMode {
	switch r {
	case RelationHybrid:
		return RelationStructural
	case RelationStructural:
		return RelationExplicit
	case RelationExplicit:
		return RelationSemantic
	default:
		return RelationHybrid
	}
}

// Model is the BubbleTea model for the graph browser.
type Model struct {
	ctx     context.Context
	store   *graph.Store
	querier querierIface
	graph   *graph.Graph
	// scopeAnchors is the scope-first default list shown when browse
	// opens. It contains the campaign-root folder plus every
	// campaign-bucket and repo-root folder. Users widen from here.
	scopeAnchors []*graph.Node
	nodes        []*graph.Node
	filtered     []*graph.Node
	cursor       int
	search       textinput.Model
	searching    bool
	width        int
	height       int
	mode         viewMode
	focusNode    *graph.Node
	neighbors    []*neighborEntry
	microCursor  int
	history      []*graph.Node
	relationMode RelationMode
	// showingAnchors is true when the list displays scopeAnchors;
	// becomes false once a scope is opened or the user widens to all
	// nodes via `a`.
	showingAnchors bool

	// Query churn control. queryGen tags each issued query so stale
	// results can be dropped; queryCancel cancels the in-flight Cmd
	// context when a newer query supersedes it.
	queryGen    uint64
	queryCancel context.CancelFunc
	results     []search.QueryResult
	groups      []resultGroup

	// Empty-query fallback state. filteredAnchors is scopeAnchors
	// after client-side chip/scope filtering; scope is the current
	// scope-prefix filter ("" for unrestricted). Sequence 03 wires
	// chips; sequence 04 wires scope.
	filteredAnchors []*graph.Node
	scope           string

	chips       chipBar
	focus       focusMode
	prevFocus   focusMode
	scopePicker scopePickerModel

	// countBuf accumulates leading digits as a vim-style count prefix
	// for list navigation. Cleared by consumeCount or on any non-digit
	// key in focusList.
	countBuf string

	// pendingG is true after a lone g keystroke; a second g triggers
	// the "jump to top" action and any other key cancels the pending
	// state.
	pendingG bool

	// Cached layout geometry (recomputed on tea.WindowSizeMsg).
	layout   layoutMode
	listW    int
	previewW int
	listH    int

	// Preview pane state.
	previewFocusID string
	previewCancel  context.CancelFunc
	previewNode    *graph.Node
	previewEdges   previewEdges
	previewRelated []search.RelatedItem
	previewScroll  int
	previewFetcher previewFetcher
}

// New creates a new TUI model from a populated graph. The browser
// opens on scope anchors rather than every node so users see campaign
// buckets, repo roots, and user-authored top-level scopes first.
func New(ctx context.Context, store *graph.Store, g *graph.Graph) *Model {
	ti := textinput.New()
	ti.Placeholder = "search scopes/nodes..."
	ti.CharLimit = 64

	nodes := g.Nodes()
	anchors := collectScopeAnchors(g)
	m := &Model{
		ctx:            ctx,
		store:          store,
		querier:        search.NewQuerier(store.DB()),
		graph:          g,
		nodes:          nodes,
		scopeAnchors:   anchors,
		filtered:       anchors,
		search:         ti,
		relationMode:   RelationHybrid,
		showingAnchors: true,
		chips: chipBar{
			Type:    chips.NewTypeChip(nodeTypeOptions),
			Tracked: chips.NewTrackedChip(),
			Mode:    chips.NewModeChip(),
		},
		scopePicker: newScopePicker(anchors),
	}
	m.filteredAnchors = filterAnchors(m.scopeAnchors, chipTypeValue(*m), chipTrackedValue(*m), m.scope)
	return m
}

// focusedRowID returns the ID of the row under the list cursor, or ""
// when no row is focused. Mirrors the row-selection logic used by
// renderList: grouped FTS results (m.groups) win when present, then
// m.filteredAnchors for the empty-query fallback, then m.filtered.
func (m Model) focusedRowID() string {
	if len(m.groups) > 0 {
		cursor := m.cursor
		for _, grp := range m.groups {
			if cursor < len(grp.Rows) {
				return grp.Rows[cursor].NodeID
			}
			cursor -= len(grp.Rows)
		}
		return ""
	}
	rows := m.filtered
	if m.filteredAnchors != nil {
		rows = m.filteredAnchors
	}
	if m.cursor < 0 || m.cursor >= len(rows) {
		return ""
	}
	return rows[m.cursor].ID
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return tea.WindowSize()
}
