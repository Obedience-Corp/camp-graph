// Package scanner walks the campaign filesystem and builds graph nodes and edges
// from the directory structure and metadata files.
package scanner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	graphErrors "github.com/Obedience-Corp/camp-graph/internal/errors"
	"github.com/Obedience-Corp/camp-graph/internal/graph"
)

// Scanner walks a campaign directory and produces graph nodes and edges.
type Scanner struct {
	root             string
	inventory        *Inventory
	inventoryOptions InventoryOptions
}

// New creates a scanner rooted at the given campaign directory.
func New(root string) *Scanner {
	return &Scanner{root: root}
}

// SetInventoryOptions overrides the inventory-build options used by Scan.
// Intended for tests that supply a custom GitProbe or want to include
// ignored entries.
func (s *Scanner) SetInventoryOptions(opts InventoryOptions) {
	s.inventoryOptions = opts
}

// Inventory returns the inventory computed by the most recent Scan call,
// or nil if Scan has not been run.
func (s *Scanner) Inventory() *Inventory {
	return s.inventory
}

// Scan walks the campaign filesystem and returns a populated graph.
//
// Before any artifact or content pass runs, Scan builds a shared
// Inventory containing the campaign-root and nested-repo boundaries and
// the live-worktree entries inside them so later passes consume a single
// canonical view of repo scope.
func (s *Scanner) Scan(ctx context.Context) (*graph.Graph, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	inv, err := BuildInventory(ctx, s.root, s.inventoryOptions)
	if err != nil {
		return nil, graphErrors.Wrap(err, "build inventory")
	}
	s.inventory = inv

	g := graph.New()

	if err := s.scanScopes(ctx, g); err != nil {
		return nil, graphErrors.Wrap(err, "scan scopes")
	}
	if err := s.scanProjects(ctx, g); err != nil {
		return nil, fmt.Errorf("scan projects: %w", err)
	}
	if err := s.scanFestivals(ctx, g); err != nil {
		return nil, fmt.Errorf("scan festivals: %w", err)
	}
	if err := s.scanIntents(ctx, g); err != nil {
		return nil, fmt.Errorf("scan intents: %w", err)
	}
	if err := s.scanWorkflow(ctx, g); err != nil {
		return nil, fmt.Errorf("scan workflow: %w", err)
	}

	// Emit note nodes for markdown inventory entries that are not owned
	// by a dedicated artifact scanner. Note creation runs after artifact
	// scanning so shouldEmitNoteNode can defer to artifact IDs where
	// they already exist.
	if err := s.scanNotes(ctx, g); err != nil {
		return nil, graphErrors.Wrap(err, "scan notes")
	}

	// Parse explicit references (markdown links, wiki-links, canvas
	// connections, inline tags, embedded attachments) into explicit
	// edges. Must run after scanNotes so link targets can be resolved
	// to note IDs.
	if err := s.extractExplicitLinks(ctx, g); err != nil {
		return nil, graphErrors.Wrap(err, "extract explicit links")
	}

	// Bridge artifact nodes to the scope graph so the two layers share
	// a single structural spine.
	if err := s.bridgeArtifactsToScopes(ctx, g); err != nil {
		return nil, graphErrors.Wrap(err, "bridge artifacts to scopes")
	}

	// Code-aware slice extraction: emits NodeFile entries (and Go
	// NodePackage groupings) for nested repo boundaries only so we
	// keep code slices from flooding the campaign-root graph.
	if err := s.extractCodeSlices(ctx, g); err != nil {
		return nil, graphErrors.Wrap(err, "extract code slices")
	}

	// Deterministic inference: bounded candidate generation then
	// aggregation to one inferred edge per pair with evidence reasons.
	candidates := GenerateCandidates(g, DefaultCandidateBudget())
	if err := s.aggregateInferredEdges(ctx, g, candidates); err != nil {
		return nil, graphErrors.Wrap(err, "aggregate inferred edges")
	}

	// Pass 2: Extract metadata and create relationship edges
	for _, n := range g.Nodes() {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		switch n.Type {
		case graph.NodeFestival:
			extractFestivalMetadata(ctx, g, n.ID, n.Path)
		case graph.NodeIntent:
			extractIntentMetadata(ctx, g, n.ID, n.Path)
		}
	}

	return g, nil
}

// scanProjects discovers project nodes under projects/.
func (s *Scanner) scanProjects(ctx context.Context, g *graph.Graph) error {
	dir := filepath.Join(s.root, "projects")
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read projects dir: %w", err)
	}
	for _, e := range entries {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		node := newProjectNode(e.Name(), filepath.Join(dir, e.Name()))
		g.AddNode(node)
	}
	return nil
}

// scanFestivals discovers festival nodes across all lifecycle directories.
func (s *Scanner) scanFestivals(ctx context.Context, g *graph.Graph) error {
	festRoot := filepath.Join(s.root, "festivals")
	lifecycleDirs := map[string]string{
		"active":            "active",
		"planning":          "planning",
		"ready":             "ready",
		"ritual":            "ritual",
		"dungeon/completed": "completed",
		"dungeon/archived":  "archived",
		"dungeon/someday":   "someday",
	}
	for subdir, status := range lifecycleDirs {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		dir := filepath.Join(festRoot, subdir)
		entries, err := os.ReadDir(dir)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("read %s: %w", subdir, err)
		}
		for _, e := range entries {
			if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
				continue
			}
			festPath := filepath.Join(dir, e.Name())
			festNode := newFestivalNode(e.Name(), festPath, status)
			g.AddNode(festNode)
			s.scanFestivalHierarchy(ctx, g, festNode.ID, festPath)
		}
	}
	return nil
}

// scanFestivalHierarchy discovers phases, sequences, and tasks within a festival.
func (s *Scanner) scanFestivalHierarchy(ctx context.Context, g *graph.Graph, festID, festPath string) {
	entries, err := os.ReadDir(festPath)
	if err != nil {
		return
	}
	for _, e := range entries {
		if ctx.Err() != nil {
			return
		}
		if !e.IsDir() || !isPhaseDir(e.Name()) {
			continue
		}
		phasePath := filepath.Join(festPath, e.Name())
		phaseNode := newPhaseNode(e.Name(), phasePath, festID)
		g.AddNode(phaseNode)
		g.AddEdge(graph.NewEdge(festID, phaseNode.ID, graph.EdgeContains, 1.0, graph.SourceStructural))

		seqEntries, err := os.ReadDir(phasePath)
		if err != nil {
			continue
		}
		for _, se := range seqEntries {
			if !se.IsDir() || strings.HasPrefix(se.Name(), ".") {
				continue
			}
			seqPath := filepath.Join(phasePath, se.Name())
			seqNode := newSequenceNode(se.Name(), seqPath, phaseNode.ID)
			g.AddNode(seqNode)
			g.AddEdge(graph.NewEdge(phaseNode.ID, seqNode.ID, graph.EdgeContains, 1.0, graph.SourceStructural))

			taskEntries, err := os.ReadDir(seqPath)
			if err != nil {
				continue
			}
			for _, te := range taskEntries {
				if te.IsDir() || !strings.HasSuffix(te.Name(), ".md") {
					continue
				}
				if te.Name() == "SEQUENCE_GOAL.md" {
					continue
				}
				taskNode := newTaskNode(te.Name(), filepath.Join(seqPath, te.Name()), seqNode.ID)
				g.AddNode(taskNode)
				g.AddEdge(graph.NewEdge(seqNode.ID, taskNode.ID, graph.EdgeContains, 1.0, graph.SourceStructural))
			}
		}
	}
}

// isPhaseDir returns true if the directory name matches NNN_NAME phase pattern.
func isPhaseDir(name string) bool {
	if len(name) < 4 {
		return false
	}
	for i := 0; i < 3; i++ {
		if name[i] < '0' || name[i] > '9' {
			return false
		}
	}
	return name[3] == '_'
}

// scanIntents discovers intent nodes under .campaign/intents/.
// Intents are .md files organized into lifecycle sub-directories.
func (s *Scanner) scanIntents(ctx context.Context, g *graph.Graph) error {
	intentsRoot := filepath.Join(s.root, ".campaign", "intents")

	scanMDFiles := func(dir, status string) error {
		entries, err := os.ReadDir(dir)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		for _, e := range entries {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			name := strings.TrimSuffix(e.Name(), ".md")
			node := newIntentNode(name, filepath.Join(dir, e.Name()), status)
			g.AddNode(node)
		}
		return nil
	}

	// Direct lifecycle dirs
	for _, status := range []string{"inbox", "active", "ready"} {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err := scanMDFiles(filepath.Join(intentsRoot, status), status); err != nil {
			return fmt.Errorf("scan intents/%s: %w", status, err)
		}
	}

	// Dungeon sub-dirs
	dungeonDir := filepath.Join(intentsRoot, "dungeon")
	dungeonEntries, err := os.ReadDir(dungeonDir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read intents/dungeon: %w", err)
	}
	for _, de := range dungeonEntries {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if !de.IsDir() || strings.HasPrefix(de.Name(), ".") {
			continue
		}
		if err := scanMDFiles(filepath.Join(dungeonDir, de.Name()), de.Name()); err != nil {
			return fmt.Errorf("scan intents/dungeon/%s: %w", de.Name(), err)
		}
	}

	return nil
}

// scanWorkflow discovers design and explore nodes under workflow/.
func (s *Scanner) scanWorkflow(ctx context.Context, g *graph.Graph) error {
	wfRoot := filepath.Join(s.root, "workflow")
	scanDir := func(subdir string, makeFn func(string, string) *graph.Node) error {
		dir := filepath.Join(wfRoot, subdir)
		entries, err := os.ReadDir(dir)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		for _, e := range entries {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
				continue
			}
			node := makeFn(e.Name(), filepath.Join(dir, e.Name()))
			g.AddNode(node)
		}
		return nil
	}

	if err := scanDir("design", newDesignDocNode); err != nil {
		return fmt.Errorf("scan design: %w", err)
	}
	if err := scanDir("explore", newExploreDocNode); err != nil {
		return fmt.Errorf("scan explore: %w", err)
	}
	return nil
}
