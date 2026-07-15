package ledger

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Obedience-Corp/camp/pkg/ledgerkit"

	"github.com/Obedience-Corp/camp-graph/internal/graph"
)

func TestIngest_EmptyLedger(t *testing.T) {
	root := t.TempDir()
	// Minimal campaign shape (no events dir).
	if err := os.MkdirAll(filepath.Join(root, "projects"), 0o755); err != nil {
		t.Fatal(err)
	}
	g := graph.New()
	rep, err := Ingest(context.Background(), root, g)
	if err != nil {
		t.Fatalf("ingest empty: %v", err)
	}
	if rep.EventsRead != 0 || rep.NodesAdded != 0 {
		t.Fatalf("expected empty report, got %+v", rep)
	}
}

func TestIngest_DecisionActionPromotedDeterministic(t *testing.T) {
	root := t.TempDir()
	seedCampaign(t, root)

	g := baseGraph()
	writeEvents(t, root, []map[string]any{
		{
			"v": 1, "id": "dec-001", "ts": "2026-07-12T10:00:00Z",
			"kind": "decided",
			"scope": map[string]any{
				"campaign": "camp-1",
				"festival": "demo-fest-DF0001",
			},
			"actor":   map[string]any{"type": "human", "name": "tester"},
			"why":     "choose ledger scan source",
			"payload": map[string]any{"title": "D008 accepted"},
			"source":  "explicit",
		},
		{
			"v": 1, "id": "ev-001", "ts": "2026-07-12T11:00:00Z",
			"kind":   "evidence_attached",
			"action": "act-001",
			"scope": map[string]any{
				"campaign": "camp-1",
				"intent":   "demo-intent",
			},
			"actor": map[string]any{"type": "human", "name": "tester"},
			"why":   "land the implementation",
			"evidence": []map[string]any{
				{"type": "commit", "repo": "campaign-root", "sha": "abc1234"},
				{"type": "path", "path": "docs/note.md"},
			},
			"source": "command",
		},
		{
			"v": 1, "id": "tr-001", "ts": "2026-07-12T12:00:00Z",
			"kind": "transitioned",
			"scope": map[string]any{
				"campaign": "camp-1",
				"intent":   "demo-intent",
			},
			"actor": map[string]any{"type": "human", "name": "tester"},
			"payload": map[string]any{
				"from":        ".campaign/intents/inbox/demo-intent.md",
				"to":          "festivals/active/demo-fest-DF0001",
				"promoted_to": "festivals/active/demo-fest-DF0001",
				"target":      "festival",
			},
			"source": "command",
		},
		// Unknown kind must be skipped, not fail.
		{
			"v": 1, "id": "unk-001", "ts": "2026-07-12T13:00:00Z",
			"kind":   "future_kind_v9",
			"scope":  map[string]any{"campaign": "camp-1"},
			"actor":  map[string]any{"type": "unknown"},
			"source": "command",
		},
	})

	rep1, err := Ingest(context.Background(), root, g)
	if err != nil {
		t.Fatalf("ingest 1: %v", err)
	}
	if rep1.UnknownKinds != 1 {
		t.Fatalf("unknown kinds: got %d want 1", rep1.UnknownKinds)
	}

	dec := g.Node("decision:dec-001")
	if dec == nil || dec.Type != graph.NodeDecision {
		t.Fatalf("missing decision node: %+v", dec)
	}
	if !dec.CreatedAt.Equal(time.Date(2026, 7, 12, 10, 0, 0, 0, time.UTC)) {
		t.Fatalf("decision CreatedAt = %v, want event ts", dec.CreatedAt)
	}

	act := g.Node("action:act-001")
	if act == nil || act.Type != graph.NodeAction {
		t.Fatalf("missing action node: %+v", act)
	}

	// produced path + commit edges
	var producedPath, producedCommit, promoted bool
	for _, e := range g.Edges() {
		switch {
		case e.Type == graph.EdgeProduced && e.Subtype == "path" && e.ToID == "file:docs/note.md":
			producedPath = true
			if e.FromID != "action:act-001" {
				t.Fatalf("path produced from %s", e.FromID)
			}
		case e.Type == graph.EdgeProduced && e.Subtype == "commit":
			producedCommit = true
			if e.Note != "campaign-root@abc1234" {
				t.Fatalf("commit note: %q", e.Note)
			}
		case e.Type == graph.EdgePromotedTo && e.FromID == "intent:demo-intent" && e.ToID == "festival:demo-fest-DF0001":
			promoted = true
			if e.Source != graph.SourceLedger {
				t.Fatalf("promoted source: %s", e.Source)
			}
		}
	}
	if !producedPath || !producedCommit || !promoted {
		t.Fatalf("edges missing path=%v commit=%v promoted=%v", producedPath, producedCommit, promoted)
	}

	// Determinism: second ingest on a fresh base graph yields identical
	// decision/action ids and edge endpoint triples.
	g2 := baseGraph()
	rep2, err := Ingest(context.Background(), root, g2)
	if err != nil {
		t.Fatalf("ingest 2: %v", err)
	}
	if rep1.EventsApplied != rep2.EventsApplied {
		t.Fatalf("applied parity: %d vs %d", rep1.EventsApplied, rep2.EventsApplied)
	}
	if g.Node("decision:dec-001") == nil || g2.Node("decision:dec-001") == nil {
		t.Fatal("decision id not stable across replays")
	}
	if g.Node("action:act-001") == nil || g2.Node("action:act-001") == nil {
		t.Fatal("action id not stable across replays")
	}
	// Same edge endpoint set (type, from, to).
	sig := func(gr *graph.Graph) map[string]int {
		m := make(map[string]int)
		for _, e := range gr.Edges() {
			if e.Source == graph.SourceStructural {
				continue
			}
			key := string(e.Type) + "|" + e.FromID + "|" + e.ToID + "|" + e.Subtype
			m[key]++
		}
		return m
	}
	s1, s2 := sig(g), sig(g2)
	if len(s1) != len(s2) {
		t.Fatalf("edge signature size %d vs %d\n%v\n%v", len(s1), len(s2), s1, s2)
	}
	for k, v := range s1 {
		if s2[k] != v {
			t.Fatalf("edge %s count %d vs %d", k, v, s2[k])
		}
	}
}

func TestIngest_WorkitemNormalization(t *testing.T) {
	root := t.TempDir()
	seedCampaign(t, root)

	// design workitem on disk with .workitem marker
	wiDir := filepath.Join(root, "workflow", "design", "demo-workitem")
	if err := os.MkdirAll(wiDir, 0o755); err != nil {
		t.Fatal(err)
	}
	meta := "version: v1alpha6\nkind: workitem\nid: design-demo-workitem-2026-07-12\ntype: design\nref: WI-abc123\n"
	if err := os.WriteFile(filepath.Join(wiDir, ".workitem"), []byte(meta), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wiDir, "README.md"), []byte("# demo\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	g := baseGraph()
	// Scanner-equivalent design_doc node.
	g.AddNode(graph.NewNode("design_doc:demo-workitem", graph.NodeDesignDoc, "demo-workitem", wiDir))

	writeEvents(t, root, []map[string]any{
		{
			"v": 1, "id": "tr-wi-1", "ts": "2026-07-12T14:00:00Z",
			"kind": "transitioned",
			"scope": map[string]any{
				"campaign": "camp-1",
				"workitem": "WI-abc123", // ref form
			},
			"actor":   map[string]any{"type": "human"},
			"payload": map[string]any{"from": "a", "to": "b", "target": "active"},
			"source":  "command",
		},
		{
			"v": 1, "id": "tr-wi-2", "ts": "2026-07-12T15:00:00Z",
			"kind": "transitioned",
			"scope": map[string]any{
				"campaign": "camp-1",
				"workitem": "demo-workitem", // slug form
			},
			"actor":   map[string]any{"type": "human"},
			"payload": map[string]any{"from": "b", "to": "c", "target": "completed"},
			"source":  "command",
		},
	})

	if _, err := Ingest(context.Background(), root, g); err != nil {
		t.Fatal(err)
	}
	n := g.Node("design_doc:demo-workitem")
	if n == nil {
		t.Fatal("design doc missing")
	}
	if n.Status != "completed" {
		t.Fatalf("status after normalized transitions: got %q want completed", n.Status)
	}
}

func TestIngest_ContextCancel(t *testing.T) {
	root := t.TempDir()
	seedCampaign(t, root)
	writeEvents(t, root, []map[string]any{
		{
			"v": 1, "id": "x", "ts": "2026-07-12T10:00:00Z",
			"kind": "created", "scope": map[string]any{"campaign": "c", "intent": "demo-intent"},
			"actor": map[string]any{"type": "unknown"}, "source": "command",
		},
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := Ingest(ctx, root, baseGraph())
	if err == nil {
		t.Fatal("expected context error")
	}
}

func TestSourceAndConfidence(t *testing.T) {
	src, conf := sourceAndConfidence(ledgerkit.SourceBackfill)
	if src != graph.SourceInferred || conf >= 1.0 {
		t.Fatalf("backfill: src=%s conf=%v", src, conf)
	}
	src, conf = sourceAndConfidence(ledgerkit.SourceCommand)
	if src != graph.SourceLedger || conf != 1.0 {
		t.Fatalf("command: src=%s conf=%v", src, conf)
	}
}

func TestIngest_ReconciledSelfKindDoesNotLoop(t *testing.T) {
	root := t.TempDir()
	seedCampaign(t, root)
	g := baseGraph()
	writeEvents(t, root, []map[string]any{
		{
			"v": 1, "id": "rc-loop", "ts": "2026-07-12T16:00:00Z",
			"kind": "reconciled",
			"scope": map[string]any{
				"campaign": "camp-1",
				"intent":   "demo-intent",
			},
			"actor":   map[string]any{"type": "unknown"},
			"payload": map[string]any{"kind": "reconciled", "note": "self"},
			"source":  "reconciled",
		},
	})
	// Must return; a regression that re-dispatches reconciled→reconciled
	// would hang or stack overflow without the depth/self-kind guard.
	rep, err := Ingest(context.Background(), root, g)
	if err != nil {
		t.Fatal(err)
	}
	if rep.EventsApplied != 1 {
		t.Fatalf("applied=%d want 1", rep.EventsApplied)
	}
}

func baseGraph() *graph.Graph {
	g := graph.New()
	// Festival matches directory name form used by the scanner.
	fest := graph.NewNode("festival:demo-fest-DF0001", graph.NodeFestival, "demo-fest-DF0001", "/tmp/fest")
	fest.Status = "active"
	g.AddNode(fest)
	intent := graph.NewNode("intent:demo-intent", graph.NodeIntent, "demo-intent", "/tmp/intent.md")
	g.AddNode(intent)
	g.AddNode(graph.NewNode("folder:.", graph.NodeFolder, ".", "/tmp"))
	// Pre-existing structural contains edge (scan would emit this).
	g.AddEdge(graph.NewEdge("festival:demo-fest-DF0001", "intent:demo-intent", graph.EdgeContains, 1.0, graph.SourceStructural))
	return g
}

func seedCampaign(t *testing.T, root string) {
	t.Helper()
	for _, d := range []string{"projects", "festivals/active", "workflow/design", ".campaign"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

func writeEvents(t *testing.T, root string, events []map[string]any) {
	t.Helper()
	dir := filepath.Join(root, ".campaign", "events", "2026-07")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(filepath.Join(dir, "test.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	for _, ev := range events {
		if err := enc.Encode(ev); err != nil {
			t.Fatal(err)
		}
	}
}
