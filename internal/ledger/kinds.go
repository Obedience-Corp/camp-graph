package ledger

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/Obedience-Corp/camp/pkg/ledgerkit"

	"github.com/Obedience-Corp/camp-graph/internal/graph"
)

func (s *ingestState) annotateArtifact(ev *ledgerkit.Event, ts time.Time, src graph.ConfidenceSource, conf float64, subtype, note string) {
	artifactID := s.resolveScopeArtifact(ev.Scope)
	if artifactID == "" {
		return
	}
	n := s.g.Node(artifactID)
	if n == nil {
		return
	}
	if status := payloadString(ev.Payload, "status"); status != "" {
		n.Status = status
	}
	if target := payloadString(ev.Payload, "target"); target != "" && subtype == "transitioned" {
		n.Status = target
	}
	if to := payloadString(ev.Payload, "to"); to != "" && n.Status == "" {
		// Prefer the trailing path segment as a coarse status when no
		// explicit target/status is present.
		n.Status = path.Base(to)
	}
	n.UpdatedAt = ts
	if n.CreatedAt.IsZero() || n.CreatedAt.After(ts) {
		n.CreatedAt = ts
	}

	parentID := s.resolveParentScope(ev.Scope, artifactID)
	if parentID == "" || parentID == artifactID {
		return
	}
	e := graph.NewEdge(parentID, artifactID, graph.EdgeContains, conf, src)
	e.Subtype = subtype
	e.Note = note
	e.CreatedAt = ts
	s.addEdge(e)
}

func (s *ingestState) maybePromotedTo(ev *ledgerkit.Event, ts time.Time, src graph.ConfidenceSource, conf float64) {
	promoted := payloadString(ev.Payload, "promoted_to")
	if promoted == "" {
		return
	}
	intentID := ""
	if ev.Scope.Intent != "" {
		intentID = "intent:" + ev.Scope.Intent
	}
	if intentID == "" || s.g.Node(intentID) == nil {
		// Workitem promotions also use promoted_to; prefer intent when
		// present, else resolve the scope artifact as the source.
		intentID = s.resolveScopeArtifact(ev.Scope)
	}
	if intentID == "" {
		return
	}
	festID := s.resolveFestivalFromPath(promoted)
	if festID == "" {
		festID = s.resolveFestival(promoted)
	}
	if festID == "" {
		return
	}
	e := graph.NewEdge(intentID, festID, graph.EdgePromotedTo, conf, src)
	e.CreatedAt = ts
	s.addEdge(e)
}

func (s *ingestState) applyDecided(ev *ledgerkit.Event, ts time.Time, src graph.ConfidenceSource, conf float64) {
	id := "decision:" + ev.ID
	title := payloadString(ev.Payload, "title")
	if title == "" {
		title = firstNonEmpty(ev.Why, ev.ID)
	}
	pathStr := ""
	for _, evd := range ev.Evidence {
		if evd.Type == ledgerkit.EvidencePath && evd.Path != "" {
			pathStr = evd.Path
			break
		}
	}
	if pathStr == "" {
		pathStr = payloadString(ev.Payload, "path")
	}

	n := graph.NewNode(id, graph.NodeDecision, title, pathStr)
	n.CreatedAt = ts
	n.UpdatedAt = ts
	if ev.Why != "" {
		n.Metadata["why"] = ev.Why
	}
	n.Metadata["event_id"] = ev.ID
	if !s.g.AddNode(n) {
		s.nodesAdded++
	}
	s.decisionNodes[ev.ID] = n

	scopeID := s.resolveScopeArtifact(ev.Scope)
	if scopeID != "" {
		e := graph.NewEdge(scopeID, id, graph.EdgeContains, conf, src)
		e.CreatedAt = ts
		s.addEdge(e)
	}

	for _, prior := range priorDecisionIDs(ev.Payload) {
		priorID := prior
		if !strings.HasPrefix(priorID, "decision:") {
			priorID = "decision:" + prior
		}
		e := graph.NewEdge(id, priorID, graph.EdgeBecauseOf, conf, graph.SourceExplicit)
		e.CreatedAt = ts
		s.addEdge(e)
	}
}

func (s *ingestState) applyEvidence(ev *ledgerkit.Event, ts time.Time, src graph.ConfidenceSource, conf float64) {
	actionKey := ev.Action
	if actionKey == "" {
		actionKey = ev.ID
	}
	actionID := "action:" + actionKey

	n := s.actionNodes[actionKey]
	if n == nil {
		n = s.g.Node(actionID)
	}
	if n == nil {
		name := firstNonEmpty(ev.Why, actionKey)
		n = graph.NewNode(actionID, graph.NodeAction, name, "")
		n.CreatedAt = ts
		n.UpdatedAt = ts
		n.Metadata["action_id"] = actionKey
		if ev.Why != "" {
			n.Metadata["why"] = ev.Why
		}
		s.g.AddNode(n)
		s.nodesAdded++
	} else {
		n.UpdatedAt = ts
		if ev.Why != "" && n.Metadata["why"] == "" {
			n.Metadata["why"] = ev.Why
		}
	}
	s.actionNodes[actionKey] = n

	scopeID := s.resolveScopeArtifact(ev.Scope)
	if scopeID != "" {
		e := graph.NewEdge(scopeID, actionID, graph.EdgeRelatesTo, conf, src)
		e.CreatedAt = ts
		s.addEdge(e)
	}

	for _, evd := range ev.Evidence {
		s.emitProduced(actionID, evd, ts, src, conf, scopeID)
	}
}

func (s *ingestState) emitProduced(actionID string, evd ledgerkit.Evidence, ts time.Time, src graph.ConfidenceSource, conf float64, scopeID string) {
	switch evd.Type {
	case ledgerkit.EvidencePath:
		rel := filepath.ToSlash(evd.Path)
		if rel == "" {
			return
		}
		fileID := "file:" + rel
		if s.g.Node(fileID) == nil {
			fn := graph.NewNode(fileID, graph.NodeFile, rel, filepath.Join(s.campaignRoot, filepath.FromSlash(rel)))
			fn.CreatedAt = ts
			fn.UpdatedAt = ts
			s.g.AddNode(fn)
			s.nodesAdded++
		}
		e := graph.NewEdge(actionID, fileID, graph.EdgeProduced, conf, src)
		e.Subtype = "path"
		e.Note = rel
		e.CreatedAt = ts
		s.addEdge(e)

	case ledgerkit.EvidenceCommit:
		toID := s.resolveRepoNode(evd.Repo, scopeID)
		e := graph.NewEdge(actionID, toID, graph.EdgeProduced, conf, src)
		e.Subtype = "commit"
		e.Note = fmt.Sprintf("%s@%s", firstNonEmpty(evd.Repo, "repo"), evd.SHA)
		e.CreatedAt = ts
		s.addEdge(e)

	case ledgerkit.EvidenceURL:
		toID := firstNonEmpty(scopeID, "folder:.")
		e := graph.NewEdge(actionID, toID, graph.EdgeProduced, conf, src)
		e.Subtype = "url"
		e.Note = evd.URL
		e.CreatedAt = ts
		s.addEdge(e)
	}
}

func (s *ingestState) applyRepaired(ev *ledgerkit.Event, ts time.Time, src graph.ConfidenceSource, conf float64) {
	if ev.Action != "" {
		actionID := "action:" + ev.Action
		if n := s.g.Node(actionID); n != nil {
			n.UpdatedAt = ts
			if note := payloadString(ev.Payload, "note"); note != "" {
				prev := n.Metadata["repair_note"]
				if prev == "" {
					n.Metadata["repair_note"] = note
				} else {
					n.Metadata["repair_note"] = prev + "; " + note
				}
			}
			n.Metadata["repaired"] = "true"
		}
		for _, e := range s.g.EdgesFrom(actionID) {
			if e.Type == graph.EdgeProduced {
				// Repaired evidence is raised to full confidence.
				if conf > e.Confidence {
					e.Confidence = conf
				}
				if e.Confidence < 1.0 {
					e.Confidence = 1.0
				}
				if e.Note != "" && !strings.Contains(e.Note, "repaired") {
					e.Note = e.Note + " (repaired)"
				}
			}
		}
	}
	s.annotateArtifact(ev, ts, src, conf, "repaired", payloadString(ev.Payload, "note"))
}
