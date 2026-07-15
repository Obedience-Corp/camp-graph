// Package ledger ingests the campaign event ledger into an in-memory
// knowledge graph per decision D008 (camp-graph ledger ingestion).
//
// The ledger is a deterministic scan source: every build/refresh that
// rebuilds re-reads the full merged ledger and emits the same nodes
// and edges for the same ledger content (stable ULID-derived ids and
// event timestamps, never wall-clock).
package ledger

import (
	"context"
	"fmt"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/Obedience-Corp/camp/pkg/ledgerkit"

	graphErrors "github.com/Obedience-Corp/camp-graph/internal/errors"
	"github.com/Obedience-Corp/camp-graph/internal/graph"
)

// Report summarizes one ingest pass for operator visibility (F6).
type Report struct {
	EventsRead    int `json:"events_read"`
	EventsApplied int `json:"events_applied"`
	EventsSkipped int `json:"events_skipped"`
	UnknownKinds  int `json:"unknown_kinds"`
	NodesAdded    int `json:"nodes_added"`
	EdgesAdded    int `json:"edges_added"`
	ReadSkipped   int `json:"read_skipped_lines"`
}

// Ingest reads the full campaign ledger and merges ledger-derived nodes
// and edges into g. Unknown kinds are skipped gracefully (A3). A missing
// events directory is not an error: ingest is a no-op on campaigns with
// no ledger yet.
func Ingest(ctx context.Context, campaignRoot string, g *graph.Graph) (*Report, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if g == nil {
		return nil, graphErrors.New("ledger ingest: nil graph")
	}
	if campaignRoot == "" {
		return nil, graphErrors.New("ledger ingest: empty campaign root")
	}

	reader, err := ledgerkit.NewReader(campaignRoot)
	if err != nil {
		return nil, graphErrors.Wrap(err, "open ledger reader")
	}
	events, readReport, err := reader.Read(ctx)
	if err != nil {
		return nil, graphErrors.Wrap(err, "read campaign ledger")
	}

	rep := &Report{EventsRead: len(events)}
	if readReport != nil {
		rep.ReadSkipped = len(readReport.Skipped)
	}

	wi := buildWorkitemIndex(campaignRoot, g)
	state := &ingestState{
		g:             g,
		campaignRoot:  campaignRoot,
		workitems:     wi,
		edgeSeen:      make(map[string]struct{}),
		actionNodes:   make(map[string]*graph.Node),
		decisionNodes: make(map[string]*graph.Node),
	}

	for _, ev := range events {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if ev == nil {
			rep.EventsSkipped++
			continue
		}
		applied, unknown, err := state.apply(ev)
		if err != nil {
			return nil, err
		}
		if unknown {
			rep.UnknownKinds++
			rep.EventsSkipped++
			continue
		}
		if applied {
			rep.EventsApplied++
		} else {
			rep.EventsSkipped++
		}
	}
	rep.NodesAdded = state.nodesAdded
	rep.EdgesAdded = state.edgesAdded
	return rep, nil
}

type ingestState struct {
	g             *graph.Graph
	campaignRoot  string
	workitems     workitemIndex
	edgeSeen      map[string]struct{}
	actionNodes   map[string]*graph.Node
	decisionNodes map[string]*graph.Node
	nodesAdded    int
	edgesAdded    int
}

func (s *ingestState) apply(ev *ledgerkit.Event) (applied, unknown bool, err error) {
	ts := parseEventTS(ev.TS)
	src, conf := sourceAndConfidence(ev.Source)

	switch ev.Kind {
	case ledgerkit.KindCreated:
		s.annotateArtifact(ev, ts, src, conf, "created", "")
		s.maybePromotedTo(ev, ts, src, conf)
		return true, false, nil

	case ledgerkit.KindTransitioned:
		note := transitionNote(ev)
		s.annotateArtifact(ev, ts, src, conf, "transitioned", note)
		s.maybePromotedTo(ev, ts, src, conf)
		return true, false, nil

	case ledgerkit.KindCompleted:
		s.annotateArtifact(ev, ts, src, conf, "completed", "completed")
		if id := s.resolveScopeArtifact(ev.Scope); id != "" {
			if n := s.g.Node(id); n != nil {
				n.Status = "completed"
				n.UpdatedAt = ts
			}
		}
		return true, false, nil

	case ledgerkit.KindDecided:
		s.applyDecided(ev, ts, src, conf)
		return true, false, nil

	case ledgerkit.KindEvidenceAttached:
		s.applyEvidence(ev, ts, src, conf)
		return true, false, nil

	case ledgerkit.KindReconciled:
		// Same treatment as the kind it reconciles would get, but with
		// inferred source and reduced confidence already set above.
		// Reconciled events still carry a Kind payload-equivalent via
		// payload.kind when present; otherwise treat as annotation.
		if inner := payloadString(ev.Payload, "kind"); inner != "" {
			innerEv := *ev
			innerEv.Kind = ledgerkit.Kind(inner)
			innerEv.Source = ledgerkit.SourceReconciled
			return s.apply(&innerEv)
		}
		s.annotateArtifact(ev, ts, src, conf, "reconciled", payloadString(ev.Payload, "note"))
		return true, false, nil

	case ledgerkit.KindRepaired:
		s.applyRepaired(ev, ts, src, conf)
		return true, false, nil

	default:
		// Unknown kinds skipped gracefully (A3 / future envelope versions).
		return false, true, nil
	}
}

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
		// Bare festival directory name or short id.
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
	if s.g.AddNode(n) {
		// Replaced existing — still count only first creation for report
		// stability; determinism holds either way.
	} else {
		s.nodesAdded++
	}
	s.decisionNodes[ev.ID] = n

	scopeID := s.resolveScopeArtifact(ev.Scope)
	if scopeID != "" {
		e := graph.NewEdge(scopeID, id, graph.EdgeContains, conf, src)
		e.CreatedAt = ts
		s.addEdge(e)
	}

	// prior decisions: payload.because_of / prior_decision / priors[]
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
			// Lightweight file node so level-2 path resolution works
			// even when the scanner never indexed this path.
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
	// Raise confidence / append note on the related action when present.
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
				e.Confidence = conf
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

func (s *ingestState) resolveRepoNode(repo, scopeID string) string {
	if repo != "" && repo != "campaign-root" && repo != "." {
		// Prefer an existing repo: node for the relative root.
		rel := filepath.ToSlash(repo)
		if strings.HasPrefix(rel, "projects/") {
			id := "repo:" + rel
			if s.g.Node(id) != nil {
				return id
			}
			// Also try without projects/ prefix variants.
		}
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
	if s.g.Node("folder:.") == nil {
		n := graph.NewNode("folder:.", graph.NodeFolder, ".", s.campaignRoot)
		n.Metadata[graph.MetaScopeKind] = graph.ScopeKindCampaignRoot
		s.g.AddNode(n)
		s.nodesAdded++
	}
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
			// Scan for a task node whose name matches.
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
		// Fall back to design_doc / explore_doc / bare name.
		for _, prefix := range []string{"design_doc:", "explore_doc:"} {
			id := prefix + scope.Workitem
			if s.g.Node(id) != nil {
				return id
			}
		}
	}
	if scope.Quest != "" {
		// Quests are not first-class graph nodes today; attach to folder if present.
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
		// task/seq/phase: parent is everything before the last segment
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
	// Already a full node id.
	if strings.HasPrefix(ref, "festival:") {
		if s.g.Node(ref) != nil {
			return ref
		}
		ref = strings.TrimPrefix(ref, "festival:")
	}
	// Exact directory name match.
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
		// fest.yaml-style id embedded in directory: ends with upper id.
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
	// Also skip if an identical structural edge already exists from scan.
	for _, existing := range s.g.EdgesFrom(e.FromID) {
		if existing.ToID == e.ToID && existing.Type == e.Type {
			// Prefer ledger annotation fields when the scan already
			// produced the structural edge: update subtype/note/ts.
			if e.Subtype != "" && existing.Subtype == "" {
				existing.Subtype = e.Subtype
			}
			if e.Note != "" && existing.Note == "" {
				existing.Note = e.Note
			}
			if e.Source == graph.SourceLedger || e.Source == graph.SourceExplicit {
				// Keep the richer ledger provenance on the existing edge.
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

func sourceAndConfidence(src ledgerkit.Source) (graph.ConfidenceSource, float64) {
	switch src {
	case ledgerkit.SourceReconciled, ledgerkit.SourceBackfill:
		return graph.SourceInferred, 0.7
	case ledgerkit.SourceExplicit:
		return graph.SourceExplicit, 1.0
	default:
		// command and anything else: ledger-sourced explicit truth.
		return graph.SourceLedger, 1.0
	}
}

func parseEventTS(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t.UTC()
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC()
	}
	return time.Time{}
}

func payloadString(payload map[string]any, key string) string {
	if payload == nil {
		return ""
	}
	v, ok := payload[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case fmt.Stringer:
		return t.String()
	default:
		return fmt.Sprint(t)
	}
}

func priorDecisionIDs(payload map[string]any) []string {
	if payload == nil {
		return nil
	}
	var out []string
	if s := payloadString(payload, "because_of"); s != "" {
		out = append(out, s)
	}
	if s := payloadString(payload, "prior_decision"); s != "" {
		out = append(out, s)
	}
	if raw, ok := payload["priors"]; ok {
		switch t := raw.(type) {
		case []any:
			for _, item := range t {
				if s, ok := item.(string); ok && s != "" {
					out = append(out, s)
				}
			}
		case []string:
			out = append(out, t...)
		}
	}
	return out
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func transitionNote(ev *ledgerkit.Event) string {
	from := payloadString(ev.Payload, "from")
	to := payloadString(ev.Payload, "to")
	if from == "" && to == "" {
		return ""
	}
	return from + "->" + to
}
