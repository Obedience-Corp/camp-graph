package scanner

import (
	"context"
	"encoding/json"
	"math"
	"path"
	"sort"
	"strings"

	"github.com/Obedience-Corp/camp-graph/internal/graph"
)

// Weights applied per candidate source when aggregating evidence. These
// weights are starting values chosen to match the design package; they
// are kept in code so behavior is testable and tunable.
var inferenceWeights = map[CandidateSource]float64{
	CandidateSameFolder:        0.35,
	CandidateSharedAncestor:    0.20,
	CandidateSharedTag:         0.30,
	CandidateSharedFrontmatter: 0.25,
	CandidateSharedRepoRoot:    0.10,
}

// tokenOverlapWeight is applied when two notes share at least three
// significant filename or title tokens. This is computed at
// aggregation time rather than in candidate generation because it does
// not need a posting list of its own for already-shortlisted pairs.
const tokenOverlapWeight = 0.20

// artifactOwnedWeight is applied when both notes live under the same
// artifact-owned scope (a project or festival subtree). It provides the
// "artifact path ownership" signal from the design.
const artifactOwnedWeight = 0.25

// inferenceEdgeType chooses the EdgeType to emit for the aggregated
// inferred relationship. Pairs driven by token or alias signals use
// EdgeSimilarTo; everything else uses EdgeRelatesTo.
func inferenceEdgeType(reasons []evidenceReason) graph.EdgeType {
	for _, r := range reasons {
		if r.Kind == "shared_tokens" || r.Kind == "alias_match" {
			return graph.EdgeSimilarTo
		}
	}
	return graph.EdgeRelatesTo
}

// evidenceReason is the serialized shape of a single signal stored
// inside Edge.Note. Kinds map to CandidateSource values plus two
// aggregation-time additions (shared_tokens, artifact_owned).
type evidenceReason struct {
	Kind   string  `json:"kind"`
	Value  string  `json:"value,omitempty"`
	Weight float64 `json:"weight"`
}

// evidencePayload is the full JSON payload serialized into Edge.Note.
type evidencePayload struct {
	Reasons []evidenceReason `json:"reasons"`
	Score   float64          `json:"score"`
}

// minInferredConfidence filters out very weak relationships. Below
// this threshold aggregated evidence is considered too thin to justify
// an inferred edge. The value matches the same-folder baseline.
const minInferredConfidence = 0.30

// aggregateInferredEdges turns the CandidateSet into one inferred edge
// per (from_id, to_id) pair. Confidence is the saturated sum of
// per-signal weights. Reasons are stored as JSON in Edge.Note and the
// dominant reason kind is echoed into Edge.Subtype.
func (s *Scanner) aggregateInferredEdges(ctx context.Context, g *graph.Graph, cs *CandidateSet) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if cs == nil || len(cs.Pairs) == 0 {
		return nil
	}

	pairs := map[string]*pairAccumulator{}
	orderKey := func(a, b string) (string, string) {
		if a <= b {
			return a, b
		}
		return b, a
	}
	for _, p := range cs.Pairs {
		from, to := orderKey(p.FromID, p.ToID)
		key := from + "|" + to
		acc, ok := pairs[key]
		if !ok {
			acc = &pairAccumulator{FromID: from, ToID: to}
			pairs[key] = acc
		}
		acc.addSignal(string(p.Source), p.Value, inferenceWeights[p.Source])
	}

	// Token overlap and artifact-owned signals are computed over the
	// shortlisted pairs, not over every node combination, which keeps
	// the work bounded to |pairs|.
	for _, acc := range pairs {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		addTokenOverlapSignal(g, acc)
		addArtifactOwnedSignal(g, acc)
	}

	for _, acc := range pairs {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		confidence := saturate(acc.Score())
		if confidence < minInferredConfidence {
			continue
		}
		payload := evidencePayload{
			Reasons: acc.OrderedReasons(),
			Score:   round3(confidence),
		}
		notePayload, _ := json.Marshal(payload)

		edgeType := inferenceEdgeType(payload.Reasons)
		edge := graph.NewEdge(acc.FromID, acc.ToID, edgeType, confidence, graph.SourceInferred)
		edge.Subtype = dominantReasonKind(payload.Reasons)
		edge.Note = string(notePayload)
		g.AddEdge(edge)
	}
	return nil
}

// pairAccumulator collects per-pair evidence before aggregation.
type pairAccumulator struct {
	FromID  string
	ToID    string
	Reasons []evidenceReason
	// seen indexes reasons by kind so duplicate same-signal hits on a
	// pair (two different folder ancestors, two different tags) don't
	// double-count the weight.
	seen map[string]bool
}

// addSignal records one evidence entry unless the same kind has
// already been recorded for this pair. For kinds with per-instance
// values (shared_tag, shared_frontmatter) we allow at most one entry
// per value to keep the weight bounded.
func (a *pairAccumulator) addSignal(kind, value string, weight float64) {
	if a.seen == nil {
		a.seen = map[string]bool{}
	}
	sigKey := kind
	if value != "" {
		sigKey = kind + "#" + value
	}
	if a.seen[sigKey] {
		return
	}
	a.seen[sigKey] = true
	a.Reasons = append(a.Reasons, evidenceReason{
		Kind:   kind,
		Value:  value,
		Weight: weight,
	})
}

// Score returns the raw sum of reason weights. The caller saturates.
func (a *pairAccumulator) Score() float64 {
	var total float64
	for _, r := range a.Reasons {
		total += r.Weight
	}
	return total
}

// OrderedReasons returns reasons sorted by weight descending, then
// kind alphabetically for stability. Stable ordering makes edge notes
// diffable across runs.
func (a *pairAccumulator) OrderedReasons() []evidenceReason {
	out := append([]evidenceReason{}, a.Reasons...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Weight != out[j].Weight {
			return out[i].Weight > out[j].Weight
		}
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].Value < out[j].Value
	})
	// Round weights for stable JSON output.
	for i := range out {
		out[i].Weight = round3(out[i].Weight)
	}
	return out
}

// addTokenOverlapSignal compares the filename/title tokens of the two
// nodes and records a shared_tokens reason when the overlap crosses a
// meaningful threshold.
func addTokenOverlapSignal(g *graph.Graph, acc *pairAccumulator) {
	from := g.Node(acc.FromID)
	to := g.Node(acc.ToID)
	if from == nil || to == nil {
		return
	}
	shared := sharedTokens(from, to)
	if len(shared) < 3 {
		return
	}
	acc.addSignal("shared_tokens", strings.Join(shared, "+"), tokenOverlapWeight)
}

// addArtifactOwnedSignal promotes a pair when both sides live beneath
// the same artifact-owned scope (any "projects/<name>" or
// "festivals/<lifecycle>/<name>" subtree).
func addArtifactOwnedSignal(g *graph.Graph, acc *pairAccumulator) {
	from := g.Node(acc.FromID)
	to := g.Node(acc.ToID)
	if from == nil || to == nil {
		return
	}
	scope := sharedArtifactScope(from.Name, to.Name)
	if scope == "" {
		return
	}
	acc.addSignal("artifact_owned", scope, artifactOwnedWeight)
}

// sharedTokens returns the case-insensitive token intersection between
// two nodes' filename and title. Tokens shorter than 3 characters are
// dropped because they are usually stopwords.
func sharedTokens(a, b *graph.Node) []string {
	left := tokenSet(a)
	right := tokenSet(b)
	var shared []string
	for t := range left {
		if right[t] {
			shared = append(shared, t)
		}
	}
	sort.Strings(shared)
	return shared
}

func tokenSet(n *graph.Node) map[string]bool {
	set := map[string]bool{}
	addTokens := func(s string) {
		for _, t := range splitTokens(strings.ToLower(s)) {
			if len(t) < 3 {
				continue
			}
			set[t] = true
		}
	}
	addTokens(path.Base(strings.TrimSuffix(n.Name, path.Ext(n.Name))))
	if t := n.Metadata[graph.MetaNoteTitle]; t != "" {
		addTokens(t)
	}
	return set
}

// splitTokens breaks a string on common separators used in file names
// (space, dash, underscore, slash, dot, comma) and returns the
// non-empty tokens.
func splitTokens(s string) []string {
	out := strings.FieldsFunc(s, func(r rune) bool {
		switch r {
		case ' ', '-', '_', '/', '.', ',', '(', ')', '[', ']', ':':
			return true
		}
		return false
	})
	return out
}

// sharedArtifactScope returns the artifact subtree both relative paths
// live under, if any. Recognized shapes:
//   - projects/<name>/...
//   - festivals/<lifecycle>/<name>/...
func sharedArtifactScope(a, b string) string {
	aParts := strings.Split(a, "/")
	bParts := strings.Split(b, "/")
	if len(aParts) < 3 || len(bParts) < 3 {
		return ""
	}
	if aParts[0] == "projects" && bParts[0] == "projects" && aParts[1] == bParts[1] {
		return "projects/" + aParts[1]
	}
	if aParts[0] == "festivals" && bParts[0] == "festivals" && len(aParts) >= 4 && len(bParts) >= 4 &&
		aParts[1] == bParts[1] && aParts[2] == bParts[2] {
		return "festivals/" + aParts[1] + "/" + aParts[2]
	}
	return ""
}

// dominantReasonKind returns the reason kind with the highest weight.
// Used as Edge.Subtype so consumers can filter without parsing JSON.
func dominantReasonKind(reasons []evidenceReason) string {
	if len(reasons) == 0 {
		return ""
	}
	return reasons[0].Kind
}

// saturate maps a raw summed weight to a final [0, 1] confidence. The
// curve is 1 - (1 - min(1, raw))^2 so stacked mid-weight signals reach
// high confidence without dominating over a single very strong signal.
func saturate(raw float64) float64 {
	if raw <= 0 {
		return 0
	}
	if raw >= 1 {
		return 1
	}
	return 1 - math.Pow(1-raw, 2)
}

func round3(f float64) float64 {
	return math.Round(f*1000) / 1000
}
