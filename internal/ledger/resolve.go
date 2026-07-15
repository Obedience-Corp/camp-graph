package ledger

import (
	"path"
	"path/filepath"
	"strings"

	"github.com/Obedience-Corp/camp/pkg/ledgerkit"

	"github.com/Obedience-Corp/camp-graph/internal/graph"
)

func (s *ingestState) resolveRepoNode(repo, scopeID string) string {
	if repo != "" && repo != "campaign-root" && repo != "." {
		rel := filepath.ToSlash(repo)
		id := "repo:" + rel
		if s.g.Node(id) != nil {
			return id
		}
		// Project name alone: projects/<name>
		if !strings.Contains(rel, "/") {
			projRepo := "repo:projects/" + rel
			if s.g.Node(projRepo) != nil {
				return projRepo
			}
			if s.g.Node("project:"+rel) != nil {
				return "project:" + rel
			}
		}
	}
	if scopeID != "" {
		return scopeID
	}
	if s.g.Node("folder:.") != nil {
		return "folder:."
	}
	// Ensure a stable campaign-root target exists for produced edges.
	n := graph.NewNode("folder:.", graph.NodeFolder, ".", s.campaignRoot)
	n.Metadata[graph.MetaScopeKind] = graph.ScopeKindCampaignRoot
	s.g.AddNode(n)
	s.nodesAdded++
	return "folder:."
}

func (s *ingestState) resolveScopeArtifact(scope ledgerkit.Scope) string {
	// Most-specific first.
	if scope.Task != "" && scope.Festival != "" {
		festID := s.resolveFestival(scope.Festival)
		if festID != "" {
			phase := scope.Phase
			seq := scope.Sequence
			task := scope.Task
			if phase != "" && seq != "" {
				id := festID + "/" + phase + "/" + seq + "/" + task
				if s.g.Node(id) != nil {
					return id
				}
			}
			for _, n := range s.g.NodesByType(graph.NodeTask) {
				if n.Name == task || strings.HasSuffix(n.ID, "/"+task) {
					if strings.HasPrefix(n.ID, festID+"/") {
						return n.ID
					}
				}
			}
		}
	}
	if scope.Sequence != "" && scope.Festival != "" {
		festID := s.resolveFestival(scope.Festival)
		if festID != "" && scope.Phase != "" {
			id := festID + "/" + scope.Phase + "/" + scope.Sequence
			if s.g.Node(id) != nil {
				return id
			}
		}
	}
	if scope.Phase != "" && scope.Festival != "" {
		festID := s.resolveFestival(scope.Festival)
		if festID != "" {
			id := festID + "/" + scope.Phase
			if s.g.Node(id) != nil {
				return id
			}
		}
	}
	if scope.Festival != "" {
		if id := s.resolveFestival(scope.Festival); id != "" {
			return id
		}
	}
	if scope.Intent != "" {
		id := "intent:" + scope.Intent
		if s.g.Node(id) != nil {
			return id
		}
	}
	if scope.Workitem != "" {
		if id := s.workitems.resolve(scope.Workitem); id != "" {
			return id
		}
		for _, prefix := range []string{"design_doc:", "explore_doc:"} {
			id := prefix + scope.Workitem
			if s.g.Node(id) != nil {
				return id
			}
		}
	}
	if scope.Quest != "" {
		id := "folder:.campaign/quests/" + scope.Quest
		if s.g.Node(id) != nil {
			return id
		}
	}
	return ""
}

func (s *ingestState) resolveParentScope(scope ledgerkit.Scope, artifactID string) string {
	if strings.HasPrefix(artifactID, "intent:") {
		if s.g.Node("folder:.campaign/intents") != nil {
			return "folder:.campaign/intents"
		}
		return "folder:."
	}
	if strings.Contains(artifactID, "/") {
		if i := strings.LastIndex(artifactID, "/"); i > 0 {
			parent := artifactID[:i]
			if s.g.Node(parent) != nil {
				return parent
			}
		}
	}
	if scope.Festival != "" {
		return s.resolveFestival(scope.Festival)
	}
	return "folder:."
}

func (s *ingestState) resolveFestival(ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ""
	}
	if strings.HasPrefix(ref, "festival:") {
		if s.g.Node(ref) != nil {
			return ref
		}
		ref = strings.TrimPrefix(ref, "festival:")
	}
	id := "festival:" + ref
	if s.g.Node(id) != nil {
		return id
	}
	// Short fest id (CA0002) or path suffix match against scanned festivals.
	upper := strings.ToUpper(ref)
	for _, n := range s.g.NodesByType(graph.NodeFestival) {
		name := n.Name
		if name == ref || strings.HasSuffix(name, "-"+ref) || strings.HasSuffix(strings.ToUpper(name), "-"+upper) {
			return n.ID
		}
		if len(ref) >= 4 && strings.HasSuffix(strings.ToUpper(name), strings.ToUpper(ref)) {
			return n.ID
		}
	}
	return ""
}

func (s *ingestState) resolveFestivalFromPath(p string) string {
	p = filepath.ToSlash(p)
	base := path.Base(p)
	if base == "" || base == "." || base == "/" {
		return ""
	}
	return s.resolveFestival(base)
}

func (s *ingestState) addEdge(e *graph.Edge) {
	key := e.FromID + "\x00" + e.ToID + "\x00" + string(e.Type)
	if _, ok := s.edgeSeen[key]; ok {
		return
	}
	// Prefer ledger annotation fields when the scan already produced
	// an identical structural edge.
	for _, existing := range s.g.EdgesFrom(e.FromID) {
		if existing.ToID == e.ToID && existing.Type == e.Type {
			if e.Subtype != "" && existing.Subtype == "" {
				existing.Subtype = e.Subtype
			}
			if e.Note != "" && existing.Note == "" {
				existing.Note = e.Note
			}
			if e.Source == graph.SourceLedger || e.Source == graph.SourceExplicit {
				if existing.Source == graph.SourceStructural {
					existing.Source = e.Source
					existing.Confidence = e.Confidence
					existing.CreatedAt = e.CreatedAt
				}
			}
			s.edgeSeen[key] = struct{}{}
			return
		}
	}
	s.edgeSeen[key] = struct{}{}
	s.g.AddEdge(e)
	s.edgesAdded++
}
