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

func newIntentNode(name, path string) *graph.Node {
	return graph.NewNode("intent:"+name, graph.NodeIntent, name, path)
}

func newDesignDocNode(name, path string) *graph.Node {
	return graph.NewNode("design_doc:"+name, graph.NodeDesignDoc, name, path)
}

func newExploreDocNode(name, path string) *graph.Node {
	return graph.NewNode("explore_doc:"+name, graph.NodeExploreDoc, name, path)
}
