// Package scanner walks the campaign filesystem and builds graph nodes and edges
// from the directory structure and metadata files.
package scanner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Obedience-Corp/camp-graph/internal/graph"
)

// Scanner walks a campaign directory and produces graph nodes and edges.
type Scanner struct {
	root string
}

// New creates a scanner rooted at the given campaign directory.
func New(root string) *Scanner {
	return &Scanner{root: root}
}

// Scan walks the campaign filesystem and returns a populated graph.
func (s *Scanner) Scan(ctx context.Context) (*graph.Graph, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	g := graph.New()

	if err := s.scanProjects(ctx, g); err != nil {
		return nil, fmt.Errorf("scan projects: %w", err)
	}
	if err := s.scanFestivals(ctx, g); err != nil {
		return nil, fmt.Errorf("scan festivals: %w", err)
	}
	if err := s.scanWorkflow(ctx, g); err != nil {
		return nil, fmt.Errorf("scan workflow: %w", err)
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

// scanWorkflow discovers intent, design, and explore nodes under workflow/.
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

	if err := scanDir("intents", newIntentNode); err != nil {
		return fmt.Errorf("scan intents: %w", err)
	}
	if err := scanDir("design", newDesignDocNode); err != nil {
		return fmt.Errorf("scan design: %w", err)
	}
	if err := scanDir("explore", newExploreDocNode); err != nil {
		return fmt.Errorf("scan explore: %w", err)
	}
	return nil
}
