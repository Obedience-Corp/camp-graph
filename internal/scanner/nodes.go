package scanner

import "github.com/Obedience-Corp/camp-graph/internal/graph"

func newProjectNode(name, path string) *graph.Node {
	return graph.NewNode("project:"+name, graph.NodeProject, name, path)
}

func newFestivalNode(name, path, status string) *graph.Node {
	n := graph.NewNode("festival:"+name, graph.NodeFestival, name, path)
	n.Status = status
	return n
}

func newPhaseNode(name, path, festivalID string) *graph.Node {
	return graph.NewNode(festivalID+"/"+name, graph.NodePhase, name, path)
}

func newSequenceNode(name, path, phaseID string) *graph.Node {
	return graph.NewNode(phaseID+"/"+name, graph.NodeSequence, name, path)
}

func newTaskNode(name, path, sequenceID string) *graph.Node {
	return graph.NewNode(sequenceID+"/"+name, graph.NodeTask, name, path)
}

func newIntentNode(name, path, status string) *graph.Node {
	n := graph.NewNode("intent:"+name, graph.NodeIntent, name, path)
	n.Status = status
	return n
}

func newDesignDocNode(name, path string) *graph.Node {
	return graph.NewNode("design_doc:"+name, graph.NodeDesignDoc, name, path)
}

func newExploreDocNode(name, path string) *graph.Node {
	return graph.NewNode("explore_doc:"+name, graph.NodeExploreDoc, name, path)
}

// newFolderNode creates a folder scope node. The ID is "folder:<rel>"
// where rel is the forward-slashed path relative to the campaign root.
// The campaign root uses "folder:." as its stable ID.
func newFolderNode(relFromRoot, absPath, scopeKind string) *graph.Node {
	id := "folder:" + relFromRoot
	n := graph.NewNode(id, graph.NodeFolder, relFromRoot, absPath)
	n.Metadata[graph.MetaScopeKind] = scopeKind
	return n
}

// newNoteNode creates a workspace note node with a path-stable ID of
// the form "note:<relative-path>". relFromRoot uses forward slashes and
// preserves the original filename so the ID round-trips to the
// on-disk path.
func newNoteNode(relFromRoot, absPath string) *graph.Node {
	id := "note:" + relFromRoot
	return graph.NewNode(id, graph.NodeNote, relFromRoot, absPath)
}

// newCanvasNode creates a canvas node with a path-stable ID of the form
// "canvas:<relative-path>".
func newCanvasNode(relFromRoot, absPath string) *graph.Node {
	return graph.NewNode("canvas:"+relFromRoot, graph.NodeCanvas, relFromRoot, absPath)
}

// newAttachmentNode creates an attachment node with a path-stable ID
// of the form "attachment:<relative-path>".
func newAttachmentNode(relFromRoot, absPath string) *graph.Node {
	return graph.NewNode("attachment:"+relFromRoot, graph.NodeAttachment, relFromRoot, absPath)
}

// newTagNode creates a tag node with an ID of the form "tag:<name>".
// Tag names are normalized to lower case with no leading # so that tag
// references are deduplicated case-insensitively.
func newTagNode(name string) *graph.Node {
	return graph.NewNode("tag:"+name, graph.NodeTag, name, "")
}
