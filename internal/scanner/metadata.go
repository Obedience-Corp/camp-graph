package scanner

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/Obedience-Corp/camp-graph/internal/graph"
	"gopkg.in/yaml.v3"
)

// festYAML represents the relevant fields from a fest.yaml file.
type festYAML struct {
	Metadata struct {
		Chain string `yaml:"chain"`
	} `yaml:"metadata"`
	ProjectPath string `yaml:"project_path"`
}

// chainYAML represents a chain definition file.
type chainYAML struct {
	Festivals []string `yaml:"festivals"`
	Edges     []struct {
		From string `yaml:"from"`
		To   string `yaml:"to"`
		Type string `yaml:"type"`
	} `yaml:"edges"`
}

// intentFrontmatter represents YAML frontmatter in intent documents.
type intentFrontmatter struct {
	GatheredFrom    []string `yaml:"gathered_from"`
	Concept         string   `yaml:"concept"`
	RelatedProjects []string `yaml:"related_projects"`
}

// extractFestivalMetadata reads fest.yaml and creates relationship edges.
func extractFestivalMetadata(_ context.Context, g *graph.Graph, festID, festPath string) {
	data, err := os.ReadFile(filepath.Join(festPath, "fest.yaml"))
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		log.Printf("warn: read fest.yaml for %s: %v", festID, err)
		return
	}
	var fy festYAML
	if err := yaml.Unmarshal(data, &fy); err != nil {
		log.Printf("warn: parse fest.yaml for %s: %v", festID, err)
		return
	}
	if fy.Metadata.Chain != "" {
		chainID := "chain:" + fy.Metadata.Chain
		if g.Node(chainID) != nil {
			g.AddEdge(graph.NewEdge(festID, chainID, graph.EdgeChainMember, 1.0, graph.SourceExplicit))
		}
	}
	if fy.ProjectPath != "" {
		projName := filepath.Base(fy.ProjectPath)
		projID := "project:" + projName
		if g.Node(projID) != nil {
			g.AddEdge(graph.NewEdge(festID, projID, graph.EdgeLinksTo, 1.0, graph.SourceExplicit))
		}
	}
}

// extractChainMetadata reads a chain YAML file and creates edges.
func extractChainMetadata(_ context.Context, g *graph.Graph, chainID, chainPath string) {
	data, err := os.ReadFile(chainPath)
	if err != nil {
		log.Printf("warn: read chain %s: %v", chainID, err)
		return
	}
	var cy chainYAML
	if err := yaml.Unmarshal(data, &cy); err != nil {
		log.Printf("warn: parse chain %s: %v", chainID, err)
		return
	}
	for _, festName := range cy.Festivals {
		festID := "festival:" + festName
		if g.Node(festID) != nil {
			g.AddEdge(graph.NewEdge(chainID, festID, graph.EdgeChainMember, 1.0, graph.SourceExplicit))
		}
	}
	for _, edge := range cy.Edges {
		fromID := "festival:" + edge.From
		toID := "festival:" + edge.To
		if g.Node(fromID) != nil && g.Node(toID) != nil {
			confidence := 1.0
			subtype := "hard"
			if edge.Type == "soft" {
				confidence = 0.8
				subtype = "soft"
			}
			e := graph.NewEdge(fromID, toID, graph.EdgeDependsOn, confidence, graph.SourceExplicit)
			e.Subtype = subtype
			g.AddEdge(e)
		}
	}
}

// extractIntentMetadata reads intent YAML frontmatter and creates edges.
// intentPath is the path to the intent .md file itself.
func extractIntentMetadata(_ context.Context, g *graph.Graph, intentID, intentPath string) {
	data, err := os.ReadFile(intentPath)
	if err != nil {
		return
	}
	fm, err := parseYAMLFrontmatter(data)
	if err != nil {
		return
	}
	for _, source := range fm.GatheredFrom {
		sourceID := "project:" + source
		if g.Node(sourceID) != nil {
			g.AddEdge(graph.NewEdge(intentID, sourceID, graph.EdgeGatheredFrom, 1.0, graph.SourceExplicit))
		}
	}
	for _, proj := range fm.RelatedProjects {
		projID := "project:" + proj
		if g.Node(projID) != nil {
			g.AddEdge(graph.NewEdge(intentID, projID, graph.EdgeRelatesTo, 0.8, graph.SourceExplicit))
		}
	}
}

// parseYAMLFrontmatter extracts YAML between --- delimiters.
func parseYAMLFrontmatter(data []byte) (*intentFrontmatter, error) {
	content := string(data)
	if !strings.HasPrefix(content, "---\n") {
		return nil, fmt.Errorf("no frontmatter")
	}
	end := strings.Index(content[4:], "\n---")
	if end < 0 {
		return nil, fmt.Errorf("no closing frontmatter delimiter")
	}
	var fm intentFrontmatter
	if err := yaml.Unmarshal([]byte(content[4:4+end]), &fm); err != nil {
		return nil, err
	}
	return &fm, nil
}
