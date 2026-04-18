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
	// NodeFolder represents a workspace directory that carries
	// organizational meaning (a campaign bucket, repo root, or ancestor
	// of authored files). Scope-kind semantics live in Node.Metadata.
	NodeFolder NodeType = "folder"
	// NodeNote represents a markdown note discovered in the workspace.
	// Notes carry frontmatter fields (aliases, tags, type, status) on
	// Node.Metadata and use a path-stable ID of the form
	// "note:<relative-path>".
	NodeNote NodeType = "note"
	// NodeCanvas represents an Obsidian-style .canvas file that curates
	// relationships between other workspace nodes.
	NodeCanvas NodeType = "canvas"
	// NodeTag represents a user-authored tag (e.g. #research) that
	// workspace content shares. ID is "tag:<name>".
	NodeTag NodeType = "tag"
	// NodeAttachment represents a non-markdown resource (image, PDF,
	// etc.) that workspace content references. ID is
	// "attachment:<relative-path>".
	NodeAttachment NodeType = "attachment"
	// NodeRepo represents a nested repository (non-root git boundary,
	// including submodules) as an explicit graph slice anchor. ID is
	// "repo:<relative-repo-root>" per the implementation contract.
	NodeRepo NodeType = "repo"
)

// Scope-related metadata keys recorded on Node.Metadata for folder and
// content nodes. These keys are the stable contract for later passes that
// consume scope information.
const (
	MetaScopeKind   = "scope_kind"
	MetaRepoRoot   = "repo_root"
	MetaPathDepth  = "path_depth"
	MetaIsSubmodule = "is_submodule"
	MetaBoundaryRel = "boundary_rel"
)

// Scope-kind values enumerated for folder nodes. These are string values
// (not a typed enum) because they are serialized through Node.Metadata.
const (
	ScopeKindCampaignRoot   = "campaign_root"
	ScopeKindRepoRoot       = "repo_root"
	ScopeKindSubmoduleRoot  = "submodule_root"
	ScopeKindCampaignBucket = "campaign_bucket"
	ScopeKindArtifactScope  = "artifact_scope"
	ScopeKindUserScope      = "user_scope"
)

// EdgeType identifies the kind of relationship between two nodes.
type EdgeType string

const (
	EdgeContains     EdgeType = "contains"
	EdgeChainMember  EdgeType = "chain_member"
	EdgeDependsOn    EdgeType = "depends_on"
	EdgeLinksTo      EdgeType = "links_to"
	EdgeRelatesTo    EdgeType = "relates_to"
	EdgeGatheredFrom EdgeType = "gathered_from"
	EdgeReferences   EdgeType = "references"
	EdgeSimilarTo    EdgeType = "similar_to"
	EdgeDefines      EdgeType = "defines"
	EdgeCalls        EdgeType = "calls"
	EdgeImports      EdgeType = "imports"
	EdgeModifies     EdgeType = "modifies"
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

// String returns the display name for a NodeType.
func (t NodeType) String() string {
	switch t {
	case NodeProject:
		return "project"
	case NodeFestival:
		return "festival"
	case NodeChain:
		return "chain"
	case NodePhase:
		return "phase"
	case NodeSequence:
		return "sequence"
	case NodeTask:
		return "task"
	case NodeIntent:
		return "intent"
	case NodeDesignDoc:
		return "design_doc"
	case NodeExploreDoc:
		return "explore_doc"
	case NodeFile:
		return "file"
	case NodeFunction:
		return "function"
	case NodeTypeDef:
		return "type_def"
	case NodePackage:
		return "package"
	case NodeFolder:
		return "folder"
	case NodeNote:
		return "note"
	case NodeCanvas:
		return "canvas"
	case NodeTag:
		return "tag"
	case NodeAttachment:
		return "attachment"
	case NodeRepo:
		return "repo"
	default:
		return string(t)
	}
}

// Note-related metadata keys recorded on Node.Metadata for NodeNote
// entries. These are the stable contract for search, browse, and
// inference passes that consume note metadata.
const (
	MetaNoteTitle    = "title"
	MetaNoteAliases  = "aliases"
	MetaNoteTags     = "tags"
	MetaNoteType     = "type"
	MetaNoteStatus   = "status"
	MetaGitState     = "git_state"
)

// String returns the display name for an EdgeType.
func (t EdgeType) String() string {
	switch t {
	case EdgeContains:
		return "contains"
	case EdgeChainMember:
		return "chain_member"
	case EdgeDependsOn:
		return "depends_on"
	case EdgeLinksTo:
		return "links_to"
	case EdgeRelatesTo:
		return "relates_to"
	case EdgeGatheredFrom:
		return "gathered_from"
	case EdgeReferences:
		return "references"
	case EdgeSimilarTo:
		return "similar_to"
	case EdgeDefines:
		return "defines"
	case EdgeCalls:
		return "calls"
	case EdgeImports:
		return "imports"
	case EdgeModifies:
		return "modifies"
	default:
		return string(t)
	}
}

// NewNode creates a Node with initialized metadata and timestamps.
// Always prefer NewNode over struct literals to avoid nil Metadata panics
// and zero-value CreatedAt/UpdatedAt in JSON output.
func NewNode(id string, nodeType NodeType, name, path string) *Node {
	now := time.Now()
	return &Node{
		ID:        id,
		Type:      nodeType,
		Name:      name,
		Path:      path,
		Metadata:  make(map[string]string),
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// NewEdge creates an Edge with the creation timestamp set.
// Always prefer NewEdge over struct literals to avoid zero-value CreatedAt
// in JSON output.
func NewEdge(fromID, toID string, edgeType EdgeType, confidence float64, source ConfidenceSource) *Edge {
	return &Edge{
		FromID:     fromID,
		ToID:       toID,
		Type:       edgeType,
		Confidence: confidence,
		Source:     source,
		CreatedAt:  time.Now(),
	}
}
