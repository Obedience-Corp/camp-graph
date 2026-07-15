package ledger

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/Obedience-Corp/camp-graph/internal/graph"
	"gopkg.in/yaml.v3"
)

// workitemIndex maps any workitem identifier (slug, ref WI-xxx, stable
// id) to a single graph node id so camp's heterogeneous scope.workitem
// values collapse to one node (D008 workitem normalization).
type workitemIndex map[string]string

func (idx workitemIndex) resolve(key string) string {
	if idx == nil {
		return ""
	}
	return idx[key]
}

func buildWorkitemIndex(campaignRoot string, g *graph.Graph) workitemIndex {
	idx := make(workitemIndex)

	// Seed from design/explore nodes already in the scanned graph: their
	// names are the directory slugs camp capture often emits.
	for _, n := range g.Nodes() {
		switch n.Type {
		case graph.NodeDesignDoc, graph.NodeExploreDoc:
			name := n.Name
			idx[name] = n.ID
			// Also map without a common type prefix if present.
			if strings.Contains(name, "/") {
				idx[filepath.Base(name)] = n.ID
			}
		}
	}

	// Walk workflow/ for .workitem markers to learn ref + stable id.
	// Skip VCS/tooling dirs and bound depth so dungeon archives cannot
	// dominate cold start on huge campaigns.
	const maxWorkitemWalkDepth = 8 // workflow/<type>/... up to nested design packs
	workflowRoot := filepath.Join(campaignRoot, "workflow")
	_ = filepath.WalkDir(workflowRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			base := d.Name()
			if base == ".git" || base == "node_modules" {
				return filepath.SkipDir
			}
			rel, relErr := filepath.Rel(workflowRoot, path)
			if relErr == nil && rel != "." {
				depth := 1 + strings.Count(filepath.ToSlash(rel), "/")
				if depth > maxWorkitemWalkDepth {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if d.Name() != ".workitem" {
			return nil
		}
		meta, err := readWorkitemMeta(path)
		if err != nil {
			return nil
		}
		dir := filepath.Dir(path)
		slug := filepath.Base(dir)
		nodeID := resolveWorkitemNodeID(g, slug, dir, campaignRoot)
		if nodeID == "" {
			return nil
		}
		idx[slug] = nodeID
		if meta.Ref != "" {
			idx[meta.Ref] = nodeID
		}
		if meta.ID != "" {
			idx[meta.ID] = nodeID
		}
		return nil
	})
	return idx
}

type workitemMeta struct {
	ID  string `yaml:"id"`
	Ref string `yaml:"ref"`
}

func readWorkitemMeta(path string) (workitemMeta, error) {
	var m workitemMeta
	data, err := os.ReadFile(path)
	if err != nil {
		return m, err
	}
	if err := yaml.Unmarshal(data, &m); err != nil {
		return m, err
	}
	return m, nil
}

func resolveWorkitemNodeID(g *graph.Graph, slug, absDir, campaignRoot string) string {
	for _, prefix := range []string{"design_doc:", "explore_doc:"} {
		if n := g.Node(prefix + slug); n != nil {
			return n.ID
		}
	}
	// Match by absolute or relative path against design/explore nodes.
	rel, err := filepath.Rel(campaignRoot, absDir)
	if err == nil {
		rel = filepath.ToSlash(rel)
		for _, n := range g.Nodes() {
			if n.Type != graph.NodeDesignDoc && n.Type != graph.NodeExploreDoc {
				continue
			}
			if n.Name == slug {
				return n.ID
			}
			nRel := n.Path
			if filepath.IsAbs(n.Path) {
				if r, err := filepath.Rel(campaignRoot, n.Path); err == nil {
					nRel = filepath.ToSlash(r)
				}
			}
			if nRel == rel || strings.HasSuffix(nRel, "/"+slug) {
				return n.ID
			}
		}
	}
	return ""
}
