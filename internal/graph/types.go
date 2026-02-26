// Package graph provides the core knowledge graph data model for campaigns.
//
// It defines node types (projects, festivals, intents, etc.) and edge types
// (contains, depends_on, relates_to, etc.) that represent the relationships
// between campaign artifacts.
package graph

import "time"

// NodeType identifies the kind of campaign artifact a node represents.
type NodeType string

const (
	NodeProject    NodeType = "project"
	NodeFestival   NodeType = "festival"
	NodeChain      NodeType = "chain"
	NodePhase      NodeType = "phase"
	NodeSequence   NodeType = "sequence"
	NodeTask       NodeType = "task"
	NodeIntent     NodeType = "intent"
	NodeDesignDoc  NodeType = "design_doc"
	NodeExploreDoc NodeType = "explore_doc"
	NodeFile       NodeType = "file"
	NodeFunction   NodeType = "function"
	NodeTypeDef    NodeType = "type_def"
	NodePackage    NodeType = "package"
)

// EdgeType identifies the kind of relationship between two nodes.
type EdgeType string

const (
	EdgeContains    EdgeType = "contains"
	EdgeChainMember EdgeType = "chain_member"
	EdgeDependsOn   EdgeType = "depends_on"
	EdgeLinksTo     EdgeType = "links_to"
	EdgeRelatesTo   EdgeType = "relates_to"
	EdgeGatheredFrom EdgeType = "gathered_from"
	EdgeReferences  EdgeType = "references"
	EdgeSimilarTo   EdgeType = "similar_to"
	EdgeDefines     EdgeType = "defines"
	EdgeCalls       EdgeType = "calls"
	EdgeImports     EdgeType = "imports"
	EdgeModifies    EdgeType = "modifies"
)

// ConfidenceSource indicates how an edge was discovered.
type ConfidenceSource string

const (
	SourceExplicit   ConfidenceSource = "explicit"
	SourceStructural ConfidenceSource = "structural"
	SourceInferred   ConfidenceSource = "inferred"
)

// Node represents a single campaign artifact in the knowledge graph.
type Node struct {
	ID        string            `json:"id"`
	Type      NodeType          `json:"type"`
	Name      string            `json:"name"`
	Path      string            `json:"path"`
	Status    string            `json:"status,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

// Edge represents a directed relationship between two nodes.
type Edge struct {
	FromID     string           `json:"from_id"`
	ToID       string           `json:"to_id"`
	Type       EdgeType         `json:"type"`
	Confidence float64          `json:"confidence"`
	Source     ConfidenceSource `json:"source"`
	Subtype    string           `json:"subtype,omitempty"`
	Note       string           `json:"note,omitempty"`
	CreatedAt  time.Time        `json:"created_at"`
}
