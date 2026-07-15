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
		applied, unknown, err := state.apply(ev, 0)
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

// applyDepthLimit bounds reconciled re-dispatch so a malicious or
// malformed payload.kind chain cannot recurse unbounded.
const applyDepthLimit = 4

func (s *ingestState) apply(ev *ledgerkit.Event, depth int) (applied, unknown bool, err error) {
	if depth > applyDepthLimit {
		return false, true, nil
	}
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
		if inner := payloadString(ev.Payload, "kind"); inner != "" {
			innerKind := ledgerkit.Kind(inner)
			// Refuse self-kind re-dispatch (would loop forever without a guard).
			if innerKind != ledgerkit.KindReconciled {
				innerEv := *ev
				innerEv.Kind = innerKind
				innerEv.Source = ledgerkit.SourceReconciled
				return s.apply(&innerEv, depth+1)
			}
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
