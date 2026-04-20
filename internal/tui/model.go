// Package tui provides a BubbleTea-based terminal graph browser.
package tui

import (
	"context"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Obedience-Corp/camp-graph/internal/graph"
	"github.com/Obedience-Corp/camp-graph/internal/search"
)

type viewMode int

const (
	modeList viewMode = iota
	modeMicrograph
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
	querier *search.Querier
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
	return &Model{
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
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return tea.WindowSize()
}
