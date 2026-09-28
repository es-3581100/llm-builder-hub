package workstation

import (
	"encoding/json"
	"strings"
	"testing"
)

// forbiddenPhase4ControlSurface is the Phase-4 mutation surface that must never
// reach a static projection: the uppercase control labels and the API paths
// that would let a read-only export offer a mutation.
var forbiddenPhase4ControlSurface = []string{
	"STAGE FILE",
	"UNSTAGE FILE",
	"COMMIT STAGED",
	"/api/stage-file",
	"/api/unstage-file",
	"/api/commit",
}

func requireNoPhase4ControlSurface(t *testing.T, label, body string) {
	t.Helper()
	for _, forbidden := range forbiddenPhase4ControlSurface {
		if strings.Contains(body, forbidden) {
			t.Fatalf("%s contains Phase-4 mutation surface %q", label, forbidden)
		}
	}
}

// phase4ExportSnapshot is a realistic dirty-repository snapshot: non-empty
// staged, unstaged, and untracked lists, a populated files slice, history, and
// diffs, so the export path that embeds repository JSON is genuinely exercised
// rather than short-circuited by an empty snapshot.
//
// Every value is deliberately free of the forbidden strings, case-insensitively.
// The forbidden set is uppercase control labels and API paths, while a real
// snapshot legitimately carries lowercase JSON keys such as "staged" and
// "unstaged" plus commit subjects; nothing here may contain a forbidden
// substring even in another case, so the guard cannot pass or fail on fixture
// noise.
func phase4ExportSnapshot() *RepositorySnapshot {
	return &RepositorySnapshot{
		Authority:      "GIT",
		RepositoryID:   strings.Repeat("1", 64),
		SnapshotSHA256: strings.Repeat("2", 64),
		Root:           "/srv/local-hub/workstation",
		GitDir:         "/srv/local-hub/workstation/.git",
		ObjectFormat:   "sha1",
		HeadCommit:     strings.Repeat("3", 40),
		HeadShort:      strings.Repeat("3", 12),
		Branch:         "phase4/local-git-commit-control",
		IndexSHA256:    strings.Repeat("4", 64),
		Detached:       false,
		Unborn:         false,
		Clean:          false,
		Staged: []GitChange{
			{Path: "alpha.txt", Status: "M", Kind: "modified"},
		},
		Unstaged: []GitChange{
			{Path: "notes.md", PreviousPath: "draft-notes.md", Status: "R", Kind: "renamed"},
		},
		Untracked:    []string{"docs/guide.md", "scratch/todo.txt"},
		StagedDiff:   "diff --git a/alpha.txt b/alpha.txt\nindex 1111111..2222222 100644\n--- a/alpha.txt\n+++ b/alpha.txt\n@@ -1 +1,2 @@\n alpha\n+alpha edited\n",
		UnstagedDiff: "diff --git a/draft-notes.md b/notes.md\nsimilarity index 92%\nrename from draft-notes.md\nrename to notes.md\n",
		History: []GitCommit{
			{
				Commit:  strings.Repeat("5", 40),
				Short:   strings.Repeat("5", 12),
				Date:    "2026-01-04T09:15:00Z",
				Author:  "Phase Four",
				Subject: "read-only repository inspection in the projection",
			},
			{
				Commit:  strings.Repeat("6", 40),
				Short:   strings.Repeat("6", 12),
				Date:    "2026-01-03T18:02:11Z",
				Author:  "Phase Four",
				Subject: "seed the local workstation fixture",
			},
		},
		Files: []SourceFileRelationship{
			{
				ID:               "src-11111111111111111111",
				Path:             "alpha.txt",
				Tracked:          true,
				Exists:           true,
				Regular:          true,
				Size:             18,
				StagedStatus:     "M",
				ContentAuthority: "GIT_WORKTREE",
				ContentSHA256:    strings.Repeat("7", 64),
				DocumentID:       "git-22222222222222222222",
			},
			{
				ID:               "src-33333333333333333333",
				Path:             "notes.md",
				PreviousPath:     "draft-notes.md",
				Tracked:          true,
				Exists:           true,
				Regular:          true,
				Size:             144,
				UnstagedStatus:   "R",
				ContentAuthority: "GIT_WORKTREE",
				ContentSHA256:    strings.Repeat("8", 64),
				DocumentID:       "git-44444444444444444444",
			},
			{
				ID:               "src-55555555555555555555",
				Path:             "docs/guide.md",
				Untracked:        true,
				Exists:           true,
				Regular:          true,
				Size:             96,
				ContentAuthority: "GIT_WORKTREE",
				ContentSHA256:    strings.Repeat("9", 64),
			},
			{
				ID:               "src-66666666666666666666",
				Path:             "scratch/todo.txt",
				Untracked:        true,
				Exists:           false,
				ContentAuthority: "GIT_WORKTREE",
			},
		},
		Documents: []RepositoryDocument{
			{
				ID:            "git-22222222222222222222",
				Path:          "alpha.txt",
				Kind:          "text",
				Title:         "alpha.txt",
				Content:       "alpha\nalpha edited\n",
				Size:          18,
				RepositoryID:  strings.Repeat("1", 64),
				Source:        "git_worktree",
				ContentSHA256: strings.Repeat("7", 64),
			},
			{
				ID:            "git-44444444444444444444",
				Path:          "notes.md",
				Kind:          "markdown",
				Title:         "notes.md",
				Content:       "# Notes\n\nLocal notes for the projection.\n",
				Size:          144,
				RepositoryID:  strings.Repeat("1", 64),
				Source:        "git_worktree",
				ContentSHA256: strings.Repeat("8", 64),
			},
			{
				ID:            "git-77777777777777777777",
				Path:          "docs/guide.md",
				Kind:          "markdown",
				Title:         "docs/guide.md",
				Content:       "# Guide\n\nUntracked working note.\n",
				Size:          96,
				RepositoryID:  strings.Repeat("1", 64),
				Source:        "git_worktree",
				ContentSHA256: strings.Repeat("9", 64),
			},
		},
		DocumentLimit:    defaultRepositoryDocumentLimit,
		MaxDocumentBytes: defaultRepositoryDocumentBytes,
	}
}

// requirePhase4FixtureCarriesNoForbiddenSurface fails if the fixture could
// produce a false positive, making the export guard provably non-vacuous.
func requirePhase4FixtureCarriesNoForbiddenSurface(t *testing.T, snapshot *RepositorySnapshot) {
	t.Helper()
	raw, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	lowered := strings.ToLower(string(raw))
	for _, forbidden := range forbiddenPhase4ControlSurface {
		if strings.Contains(lowered, strings.ToLower(forbidden)) {
			t.Fatalf("fixture itself contains %q (case-insensitively); the export guard would be a false positive", forbidden)
		}
	}
}

func TestPhase4StaticExportEmbedsRepositoryJSONWithoutAnyGitControlSurface(t *testing.T) {
	state := DefaultState()
	hash := strings.Repeat("a", 64)
	snapshot := phase4ExportSnapshot()
	requirePhase4FixtureCarriesNoForbiddenSurface(t, snapshot)

	body, err := ExportHTMLWithRepository(state, hash, snapshot)
	if err != nil {
		t.Fatalf("ExportHTMLWithRepository: %v", err)
	}
	export := string(body)

	// Non-vacuity: the repository snapshot really is embedded as JSON, so the
	// forbidden-string sweep below is inspecting the leak-prone payload and not
	// an export that simply omitted repository state.
	for _, want := range []string{
		`id="hub-repository"`,
		`"repository_id":"` + snapshot.RepositoryID + `"`,
		`"staged":[`,
		`"unstaged":[`,
		`"untracked":["docs/guide.md","scratch/todo.txt"]`,
		`"path":"alpha.txt"`,
		"alpha.txt",
		"notes.md",
		"docs/guide.md",
		"read-only repository inspection in the projection",
	} {
		if !strings.Contains(export, want) {
			t.Fatalf("export is missing %q; the repository payload was not embedded, so the guard below would be vacuous", want)
		}
	}
	if len(snapshot.Files) == 0 || len(snapshot.Staged) == 0 || len(snapshot.Unstaged) == 0 || len(snapshot.Untracked) == 0 {
		t.Fatalf("fixture is not realistic: files=%d staged=%d unstaged=%d untracked=%d",
			len(snapshot.Files), len(snapshot.Staged), len(snapshot.Unstaged), len(snapshot.Untracked))
	}

	requireNoPhase4ControlSurface(t, "ExportHTMLWithRepository", export)
}

func TestPhase4StaticExportWithoutRepositoryCarriesNoGitControlSurface(t *testing.T) {
	state := DefaultState()
	hash := strings.Repeat("a", 64)
	snapshot := phase4ExportSnapshot()

	body, err := ExportHTML(state, hash)
	if err != nil {
		t.Fatalf("ExportHTML: %v", err)
	}
	export := string(body)

	// The nil-repository path must stay repository-free, so the absence of
	// Phase-4 control surface here is a real property of the output.
	if strings.Contains(export, snapshot.RepositoryID) || strings.Contains(export, `id="hub-repository"`) {
		t.Fatalf("ExportHTML embedded repository state it was not given: %q", export)
	}
	if !strings.Contains(export, `id="hub-state"`) {
		t.Fatalf("export is missing the state payload: %q", export)
	}

	requireNoPhase4ControlSurface(t, "ExportHTML", export)
}
