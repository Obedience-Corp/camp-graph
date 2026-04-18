package scanner_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/Obedience-Corp/camp-graph/internal/scanner"
)

func TestGenerateCandidates_SameFolder(t *testing.T) {
	root := resolvePath(t, t.TempDir())
	writeFile(t, filepath.Join(root, "Work/a.md"), "# a\n")
	writeFile(t, filepath.Join(root, "Work/b.md"), "# b\n")
	writeFile(t, filepath.Join(root, "Other/c.md"), "# c\n")

	g := scanNotesFixture(t, root)
	cs := scanner.GenerateCandidates(t.Context(), g, scanner.CandidateBudget{})
	// Expect same-folder pair for Work (a,b) but not cross-folder (a,c) or (b,c).
	same := countSource(cs.Pairs, scanner.CandidateSameFolder)
	if same < 1 {
		t.Errorf("expected at least one same_folder pair; got %d", same)
	}
	for _, p := range cs.Pairs {
		if p.Source != scanner.CandidateSameFolder {
			continue
		}
		if hasCrossFolder(p) {
			t.Errorf("same_folder pair crosses folders: %+v", p)
		}
	}
}

func TestGenerateCandidates_TagPostingList(t *testing.T) {
	root := resolvePath(t, t.TempDir())
	writeFile(t, filepath.Join(root, "A/a.md"), "Body #planning.\n")
	writeFile(t, filepath.Join(root, "B/b.md"), "Body #planning.\n")
	writeFile(t, filepath.Join(root, "C/c.md"), "Body #cooking.\n")

	g := scanNotesFixture(t, root)
	cs := scanner.GenerateCandidates(t.Context(), g, scanner.CandidateBudget{})

	tagPairs := 0
	for _, p := range cs.Pairs {
		if p.Source != scanner.CandidateSharedTag {
			continue
		}
		tagPairs++
		if p.Value != "planning" && p.Value != "cooking" {
			t.Errorf("unexpected shared_tag value: %q", p.Value)
		}
	}
	if tagPairs == 0 {
		t.Error("expected at least one shared_tag pair")
	}
}

func TestGenerateCandidates_FrontmatterType(t *testing.T) {
	root := resolvePath(t, t.TempDir())
	writeFile(t, filepath.Join(root, "A/a.md"), "---\ntype: daily\n---\n# a\n")
	writeFile(t, filepath.Join(root, "B/b.md"), "---\ntype: daily\n---\n# b\n")
	writeFile(t, filepath.Join(root, "C/c.md"), "---\ntype: reference\n---\n# c\n")

	g := scanNotesFixture(t, root)
	cs := scanner.GenerateCandidates(t.Context(), g, scanner.CandidateBudget{})

	found := false
	for _, p := range cs.Pairs {
		if p.Source == scanner.CandidateSharedFrontmatter && p.Value == "type=daily" {
			found = true
		}
	}
	if !found {
		t.Error("expected shared_frontmatter type=daily pair")
	}
}

func TestGenerateCandidates_BudgetTruncates(t *testing.T) {
	root := resolvePath(t, t.TempDir())
	// 10 notes in one folder -> 45 unordered pairs, far above tight cap.
	for i := 0; i < 10; i++ {
		writeFile(t, filepath.Join(root, fmt.Sprintf("Work/n%02d.md", i)), "# n\n")
	}

	g := scanNotesFixture(t, root)
	cs := scanner.GenerateCandidates(t.Context(), g, scanner.CandidateBudget{
		MaxMembersPerGroup: 4,
		MaxPairs:           50,
	})
	// With MaxMembersPerGroup=4, same-folder emits only C(4,2)=6 pairs.
	sameCount := countSource(cs.Pairs, scanner.CandidateSameFolder)
	if sameCount > 6 {
		t.Errorf("expected bounded same_folder count <= 6, got %d", sameCount)
	}
	if !cs.Truncated {
		t.Error("expected Truncated=true when group exceeds MaxMembersPerGroup")
	}
}

func TestGenerateCandidates_NoAllPairsExplosion(t *testing.T) {
	root := resolvePath(t, t.TempDir())
	// Put many notes under different folders with no shared tags or
	// frontmatter so inference should NOT produce a global all-pairs
	// candidate set.
	for i := 0; i < 20; i++ {
		dir := fmt.Sprintf("Folder%d", i)
		writeFile(t, filepath.Join(root, dir, "note.md"), "# n\n")
	}

	g := scanNotesFixture(t, root)
	cs := scanner.GenerateCandidates(t.Context(), g, scanner.CandidateBudget{})

	// With 20 singleton folders and no shared tags/frontmatter, only
	// shared_repo_root (all sharing the campaign root) could pair them.
	// That's the only legitimate source we expect at scale. Validate
	// none of the candidate sources that require structural affinity
	// leaked out.
	for _, p := range cs.Pairs {
		if p.Source == scanner.CandidateSameFolder {
			t.Errorf("unexpected same_folder pair: %+v", p)
		}
		if p.Source == scanner.CandidateSharedTag {
			t.Errorf("unexpected shared_tag pair: %+v", p)
		}
	}
	// Because all 20 notes share the same repo root (campaign root),
	// shared_repo_root pairs should be emitted but still bounded.
	repoRoot := countSource(cs.Pairs, scanner.CandidateSharedRepoRoot)
	if repoRoot == 0 {
		t.Error("expected shared_repo_root pairs for notes under same campaign root")
	}
}

func TestGenerateCandidates_CountsBySource(t *testing.T) {
	root := resolvePath(t, t.TempDir())
	writeFile(t, filepath.Join(root, "A/x.md"), "---\ntype: daily\ntags: [life]\n---\n\n# x\n")
	writeFile(t, filepath.Join(root, "A/y.md"), "---\ntype: daily\n---\n\n# y\n")

	g := scanNotesFixture(t, root)
	cs := scanner.GenerateCandidates(t.Context(), g, scanner.CandidateBudget{})
	if cs.CountsBySource[scanner.CandidateSameFolder] == 0 {
		t.Error("CountsBySource missing same_folder")
	}
	if cs.CountsBySource[scanner.CandidateSharedFrontmatter] == 0 {
		t.Error("CountsBySource missing shared_frontmatter")
	}
}

func countSource(pairs []scanner.CandidatePair, source scanner.CandidateSource) int {
	c := 0
	for _, p := range pairs {
		if p.Source == source {
			c++
		}
	}
	return c
}

func hasCrossFolder(p scanner.CandidatePair) bool {
	a := filepath.Dir(afterColon(p.FromID))
	b := filepath.Dir(afterColon(p.ToID))
	return a != b
}

func afterColon(id string) string {
	for i := 0; i < len(id); i++ {
		if id[i] == ':' {
			return id[i+1:]
		}
	}
	return id
}
