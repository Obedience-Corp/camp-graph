package ledger

import (
	"fmt"
	"time"

	"github.com/Obedience-Corp/camp/pkg/ledgerkit"

	"github.com/Obedience-Corp/camp-graph/internal/graph"
)

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
