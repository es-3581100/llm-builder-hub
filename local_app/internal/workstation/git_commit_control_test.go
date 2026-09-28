package workstation

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// phase4Fixture is a disposable repository used to prove guarded local Git
// control. Porcelain Git is used here on purpose: fixtures build and destroy
// repository state so the product code under test can be observed.
type phase4Fixture struct {
	dir    string
	remote string
	repo   *GitRepository
}

func newPhase4Fixture(t *testing.T) *phase4Fixture {
	t.Helper()
	dir := initRepo(t, "repo")
	remote := filepath.Join(t.TempDir(), "remote.git")
	if err := os.MkdirAll(remote, 0o755); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, remote, "init", "--bare", "-b", "main")
	gitCmd(t, dir, "remote", "add", "origin", remote)
	gitCmd(t, dir, "branch", "side-branch")
	gitCmd(t, dir, "tag", "phase4-tag")
	return &phase4Fixture{dir: dir, remote: remote, repo: NewGitRepository(dir)}
}

func (f *phase4Fixture) snapshot(t *testing.T) RepositorySnapshot {
	t.Helper()
	got, err := f.repo.Inspect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func (f *phase4Fixture) write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(f.dir, filepath.FromSlash(path)), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func (f *phase4Fixture) worktreeBytes(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(f.dir, filepath.FromSlash(path)))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func (f *phase4Fixture) indexMode(t *testing.T, path string) string {
	t.Helper()
	entry := gitCmd(t, f.dir, "ls-files", "-s", "--", path)
	fields := strings.Fields(strings.TrimSpace(entry))
	if len(fields) < 3 {
		t.Fatalf("no index entry for %q: %q", path, entry)
	}
	return fields[0]
}

func (f *phase4Fixture) refs(t *testing.T) map[string]string {
	t.Helper()
	out := gitCmd(t, f.dir, "for-each-ref", "--format=%(refname) %(objectname)")
	result := map[string]string{}
	for _, line := range nonEmptyLines(out) {
		name, value, ok := strings.Cut(line, " ")
		if !ok {
			t.Fatalf("unparsable ref line %q", line)
		}
		result[name] = value
	}
	return result
}

// treeEntries normalizes `git ls-tree -r` output into path -> "mode sha".
func (f *phase4Fixture) treeEntries(t *testing.T, treeish string) map[string]string {
	t.Helper()
	out := gitCmd(t, f.dir, "ls-tree", "-r", treeish)
	result := map[string]string{}
	for _, line := range nonEmptyLines(out) {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			t.Fatalf("unparsable ls-tree line %q", line)
		}
		tab := strings.Index(line, "\t")
		if tab < 0 {
			t.Fatalf("unparsable ls-tree line %q", line)
		}
		result[line[tab+1:]] = fields[0] + " " + fields[2]
	}
	return result
}

func (f *phase4Fixture) indexEntries(t *testing.T) map[string]string {
	t.Helper()
	out := gitCmd(t, f.dir, "ls-files", "-s")
	result := map[string]string{}
	for _, line := range nonEmptyLines(out) {
		fields := strings.Fields(line)
		tab := strings.Index(line, "\t")
		if tab < 0 {
			t.Fatalf("unparsable ls-files line %q", line)
		}
		result[line[tab+1:]] = fields[0] + " " + fields[1]
	}
	return result
}

func (f *phase4Fixture) head(t *testing.T) string {
	t.Helper()
	return strings.TrimSpace(gitCmd(t, f.dir, "rev-parse", "HEAD"))
}

// commitMessage returns the exact stored message bytes of a commit object,
// without the newline `git log --format` appends.
func (f *phase4Fixture) commitMessage(t *testing.T, commit string) string {
	t.Helper()
	raw, err := exec.Command("git", "-C", f.dir, "cat-file", "commit", commit).Output()
	if err != nil {
		t.Fatalf("cat-file commit %s: %v", commit, err)
	}
	boundary := bytes.Index(raw, []byte("\n\n"))
	if boundary < 0 {
		t.Fatalf("commit %s has no message boundary", commit)
	}
	return string(raw[boundary+2:])
}

func (f *phase4Fixture) stageRequest(t *testing.T, path string) StageFileRequest {
	t.Helper()
	snapshot := f.snapshot(t)
	rel, ok := findFile(snapshot, path)
	if !ok {
		t.Fatalf("path %q missing from repository view", path)
	}
	return StageFileRequest{
		RepositoryID:           snapshot.RepositoryID,
		Path:                   path,
		ExpectedHeadCommit:     snapshot.HeadCommit,
		ExpectedBranch:         snapshot.Branch,
		ExpectedIndexSHA256:    snapshot.IndexSHA256,
		ExpectedWorktreeSHA256: rel.ContentSHA256,
	}
}

func (f *phase4Fixture) unstageRequest(t *testing.T, path string) UnstageFileRequest {
	t.Helper()
	snapshot := f.snapshot(t)
	return UnstageFileRequest{
		RepositoryID:        snapshot.RepositoryID,
		Path:                path,
		ExpectedHeadCommit:  snapshot.HeadCommit,
		ExpectedBranch:      snapshot.Branch,
		ExpectedIndexSHA256: snapshot.IndexSHA256,
	}
}

func (f *phase4Fixture) commitRequest(t *testing.T, message string) CommitStagedRequest {
	t.Helper()
	snapshot := f.snapshot(t)
	return CommitStagedRequest{
		RepositoryID:        snapshot.RepositoryID,
		ExpectedHeadCommit:  snapshot.HeadCommit,
		ExpectedBranch:      snapshot.Branch,
		ExpectedIndexSHA256: snapshot.IndexSHA256,
		CommitMessage:       message,
	}
}

func requireErrorIs(t *testing.T, err error, target error) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected %v, got nil error", target)
	}
	if !errors.Is(err, target) {
		t.Fatalf("expected %v, got %v", target, err)
	}
}

func requireEqualStringMaps(t *testing.T, label string, want, got map[string]string) {
	t.Helper()
	if len(want) != len(got) {
		t.Fatalf("%s: entry count %d != %d\nwant=%v\ngot=%v", label, len(want), len(got), want, got)
	}
	for key, value := range want {
		other, ok := got[key]
		if !ok {
			t.Fatalf("%s: missing %q in %v", label, key, got)
		}
		if other != value {
			t.Fatalf("%s: %q is %q, want %q", label, key, other, value)
		}
	}
}

func installTripwireHooks(t *testing.T, repoDir, markerDir string, names ...string) {
	t.Helper()
	hooks := filepath.Join(repoDir, ".git", "hooks")
	if err := os.MkdirAll(hooks, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		script := "#!/bin/sh\n: > '" + filepath.Join(markerDir, name) + "'\nexit 0\n"
		if err := os.WriteFile(filepath.Join(hooks, name), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

func tripwireMarkers(t *testing.T, markerDir string) []string {
	t.Helper()
	entries, err := os.ReadDir(markerDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatal(err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	return names
}

func TestStageFileMutatesIndexOnlyAndLeavesWorktreeAndHeadUnchanged(t *testing.T) {
	f := newPhase4Fixture(t)
	f.write(t, "alpha.txt", "alpha staged\n")
	request := f.stageRequest(t, "alpha.txt")

	headBefore := f.head(t)
	worktreeBefore := f.worktreeBytes(t, "alpha.txt")
	modeBefore := f.indexMode(t, "alpha.txt")
	indexBefore := f.snapshot(t).IndexSHA256

	result, err := f.repo.StageFile(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if result.BeforeIndexSHA256 != indexBefore {
		t.Fatalf("before index identity %q != bound %q", result.BeforeIndexSHA256, indexBefore)
	}
	if result.AfterIndexSHA256 == result.BeforeIndexSHA256 {
		t.Fatal("STAGE FILE did not change index identity")
	}
	if result.AfterIndexSHA256 != result.Repository.IndexSHA256 {
		t.Fatalf("returned identity %q != fresh snapshot %q", result.AfterIndexSHA256, result.Repository.IndexSHA256)
	}
	if result.Path != "alpha.txt" || result.ExpectedHeadCommit != headBefore || result.HeadCommit != headBefore {
		t.Fatalf("unexpected result identity: %+v", result)
	}
	if result.StagedMode != modeBefore {
		t.Fatalf("staged mode %q != index mode %q", result.StagedMode, modeBefore)
	}
	if !validObjectID(result.StagedObjectID) {
		t.Fatalf("staged object id %q is not an object id", result.StagedObjectID)
	}
	if got := f.head(t); got != headBefore {
		t.Fatalf("HEAD moved on stage: %q -> %q", headBefore, got)
	}
	if got := f.worktreeBytes(t, "alpha.txt"); !bytes.Equal(got, worktreeBefore) {
		t.Fatalf("worktree changed by stage: %q -> %q", worktreeBefore, got)
	}
	if got := f.indexMode(t, "alpha.txt"); got != modeBefore {
		t.Fatalf("index mode changed by stage: %q -> %q", modeBefore, got)
	}
	after := f.snapshot(t)
	if len(after.Staged) != 1 || after.Staged[0].Path != "alpha.txt" || after.Staged[0].Kind != "modified" {
		t.Fatalf("staged view after stage: %+v", after.Staged)
	}
	if len(after.Unstaged) != 0 || len(after.Untracked) != 0 {
		t.Fatalf("stage left non-index state: unstaged=%+v untracked=%+v", after.Unstaged, after.Untracked)
	}
}

func TestStageFileRejectsStaleIdentity(t *testing.T) {
	f := newPhase4Fixture(t)
	f.write(t, "alpha.txt", "alpha staged\n")
	bound := f.stageRequest(t, "alpha.txt")

	t.Run("wrong repository", func(t *testing.T) {
		request := bound
		request.RepositoryID = strings.Repeat("a", 64)
		_, err := f.repo.StageFile(context.Background(), request)
		requireErrorIs(t, err, ErrGitRepositoryConflict)
	})
	t.Run("wrong head", func(t *testing.T) {
		request := bound
		request.ExpectedHeadCommit = strings.Repeat("b", 40)
		_, err := f.repo.StageFile(context.Background(), request)
		requireErrorIs(t, err, ErrGitHeadConflict)
	})
	t.Run("wrong branch", func(t *testing.T) {
		request := bound
		request.ExpectedBranch = "other-branch"
		_, err := f.repo.StageFile(context.Background(), request)
		requireErrorIs(t, err, ErrGitBranchConflict)
	})
	t.Run("wrong index", func(t *testing.T) {
		request := bound
		request.ExpectedIndexSHA256 = strings.Repeat("c", 64)
		_, err := f.repo.StageFile(context.Background(), request)
		requireErrorIs(t, err, ErrGitIndexConflict)
	})
	t.Run("wrong worktree", func(t *testing.T) {
		request := bound
		request.ExpectedWorktreeSHA256 = strings.Repeat("d", 64)
		_, err := f.repo.StageFile(context.Background(), request)
		requireErrorIs(t, err, ErrGitWorktreeConflict)
	})
	t.Run("adversarial index drift", func(t *testing.T) {
		gitCmd(t, f.dir, "add", "notes.md")
		_, err := f.repo.StageFile(context.Background(), bound)
		requireErrorIs(t, err, ErrGitIndexConflict)
		gitCmd(t, f.dir, "restore", "--staged", "notes.md")
	})
	t.Run("adversarial worktree drift", func(t *testing.T) {
		extra := f.stageRequest(t, "alpha.txt")
		f.write(t, "alpha.txt", "alpha changed again\n")
		_, err := f.repo.StageFile(context.Background(), extra)
		requireErrorIs(t, err, ErrGitWorktreeConflict)
		f.write(t, "alpha.txt", "alpha staged\n")
	})
	t.Run("adversarial head drift", func(t *testing.T) {
		extra := f.stageRequest(t, "alpha.txt")
		gitCmd(t, f.dir, "add", "alpha.txt")
		gitCmd(t, f.dir, "commit", "-m", "external commit", "--no-gpg-sign")
		_, err := f.repo.StageFile(context.Background(), extra)
		requireErrorIs(t, err, ErrGitHeadConflict)
	})
	t.Run("invalid request fields", func(t *testing.T) {
		cases := map[string]func(StageFileRequest) StageFileRequest{
			"missing repository": func(r StageFileRequest) StageFileRequest { r.RepositoryID = ""; return r },
			"missing head":       func(r StageFileRequest) StageFileRequest { r.ExpectedHeadCommit = ""; return r },
			"uppercase index": func(r StageFileRequest) StageFileRequest {
				r.ExpectedIndexSHA256 = strings.ToUpper(r.ExpectedIndexSHA256)
				return r
			},
			"short worktree hash": func(r StageFileRequest) StageFileRequest { r.ExpectedWorktreeSHA256 = "abc"; return r },
			"absolute path":       func(r StageFileRequest) StageFileRequest { r.Path = "/etc/passwd"; return r },
			"traversal path":      func(r StageFileRequest) StageFileRequest { r.Path = "../escape.txt"; return r },
			"non canonical path":  func(r StageFileRequest) StageFileRequest { r.Path = "./alpha.txt"; return r },
			"dash branch":         func(r StageFileRequest) StageFileRequest { r.ExpectedBranch = "-f"; return r },
			"head with spaces":    func(r StageFileRequest) StageFileRequest { r.ExpectedHeadCommit = "abc def"; return r },
		}
		for name, mutate := range cases {
			t.Run(name, func(t *testing.T) {
				_, err := f.repo.StageFile(context.Background(), mutate(bound))
				requireErrorIs(t, err, ErrGitInvalidRequest)
			})
		}
	})
}

func TestStageFileRejectsUnsupportedTargets(t *testing.T) {
	f := newPhase4Fixture(t)
	ctx := context.Background()

	t.Run("already staged", func(t *testing.T) {
		f.write(t, "alpha.txt", "staged then edited\n")
		request := f.stageRequest(t, "alpha.txt")
		gitCmd(t, f.dir, "add", "alpha.txt")
		f.write(t, "alpha.txt", "staged then edited again\n")
		request.ExpectedIndexSHA256 = f.snapshot(t).IndexSHA256
		request.ExpectedWorktreeSHA256 = contentSHA256(f.worktreeBytes(t, "alpha.txt"))
		_, err := f.repo.StageFile(ctx, request)
		requireErrorIs(t, err, ErrGitUnsupportedTarget)
		gitCmd(t, f.dir, "restore", "--staged", "--worktree", "alpha.txt")
	})

	t.Run("untracked", func(t *testing.T) {
		f.write(t, "untracked.txt", "brand new\n")
		request := StageFileRequest{
			RepositoryID:           f.snapshot(t).RepositoryID,
			Path:                   "untracked.txt",
			ExpectedHeadCommit:     f.snapshot(t).HeadCommit,
			ExpectedBranch:         f.snapshot(t).Branch,
			ExpectedIndexSHA256:    f.snapshot(t).IndexSHA256,
			ExpectedWorktreeSHA256: contentSHA256([]byte("brand new\n")),
		}
		_, err := f.repo.StageFile(ctx, request)
		requireErrorIs(t, err, ErrGitUnsupportedTarget)
	})

	t.Run("deleted", func(t *testing.T) {
		request := f.stageRequest(t, "notes.md")
		if err := os.Remove(filepath.Join(f.dir, "notes.md")); err != nil {
			t.Fatal(err)
		}
		_, err := f.repo.StageFile(ctx, request)
		requireErrorIs(t, err, ErrGitUnsupportedTarget)
		gitCmd(t, f.dir, "restore", "notes.md")
	})

	t.Run("symlink", func(t *testing.T) {
		if err := os.Remove(filepath.Join(f.dir, "alpha.txt")); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("notes.md", filepath.Join(f.dir, "alpha.txt")); err != nil {
			t.Fatal(err)
		}
		request := StageFileRequest{
			RepositoryID:           f.snapshot(t).RepositoryID,
			Path:                   "alpha.txt",
			ExpectedHeadCommit:     f.snapshot(t).HeadCommit,
			ExpectedBranch:         f.snapshot(t).Branch,
			ExpectedIndexSHA256:    f.snapshot(t).IndexSHA256,
			ExpectedWorktreeSHA256: contentSHA256([]byte("notes.md")),
		}
		_, err := f.repo.StageFile(ctx, request)
		requireErrorIs(t, err, ErrGitUnsupportedTarget)
		gitCmd(t, f.dir, "checkout", "--", "alpha.txt")
	})
}

func TestStageFileRejectsUnsafeGitAttributes(t *testing.T) {
	f := newPhase4Fixture(t)
	f.write(t, "cleaned.dat", "data v1\n")
	f.write(t, "encoded.txt", "text v1\n")
	f.write(t, "marked.txt", "marked v1\n")
	gitCmd(t, f.dir, "add", "cleaned.dat", "encoded.txt", "marked.txt")
	gitCmd(t, f.dir, "commit", "-m", "attribute targets", "--no-gpg-sign")

	// Tripwire the custom filter driver so any hidden clean/smudge execution
	// becomes observable as a marker file.
	markerDir := t.TempDir()
	for _, driver := range []string{"clean", "smudge"} {
		gitCmd(t, f.dir, "config", "filter.tripscript."+driver, "!touch '"+filepath.Join(markerDir, "filter-"+driver)+"'")
	}
	f.write(t, ".gitattributes", "cleaned.dat filter=tripscript\nencoded.txt working-tree-encoding=UTF-8\nmarked.txt filter\n")
	gitCmd(t, f.dir, "add", ".gitattributes")
	gitCmd(t, f.dir, "commit", "-m", "attribute rules", "--no-gpg-sign")

	for _, path := range []string{"cleaned.dat", "encoded.txt", "marked.txt"} {
		f.write(t, path, path+" v2\n")
		_, err := f.repo.StageFile(context.Background(), f.stageRequest(t, path))
		requireErrorIs(t, err, ErrGitUnsafeAttributes)
	}
	if got := tripwireMarkers(t, markerDir); len(got) != 0 {
		t.Fatalf("custom clean filter ran as a hidden execution path: %v", got)
	}
	if len(f.snapshot(t).Staged) != 0 {
		t.Fatal("rejected stage still wrote the index")
	}
}

func TestStageFileDoesNotStageModeOrTypeChanges(t *testing.T) {
	f := newPhase4Fixture(t)
	f.write(t, "alpha.txt", "alpha executable attempt\n")
	request := f.stageRequest(t, "alpha.txt")
	modeBefore := f.indexMode(t, "alpha.txt")

	if err := os.Chmod(filepath.Join(f.dir, "alpha.txt"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := f.repo.StageFile(context.Background(), request)
	requireErrorIs(t, err, ErrGitUnsupportedTarget)
	if got := f.indexMode(t, "alpha.txt"); got != modeBefore {
		t.Fatalf("rejected stage changed index mode %q -> %q", modeBefore, got)
	}
	if len(f.snapshot(t).Staged) != 0 {
		t.Fatal("rejected stage still wrote the index")
	}
	if err := os.Chmod(filepath.Join(f.dir, "alpha.txt"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestUnstageFileRestoresOnlyThatIndexEntry(t *testing.T) {
	f := newPhase4Fixture(t)
	f.write(t, "alpha.txt", "alpha staged\n")
	f.write(t, "notes.md", "# Notes edited\n")
	gitCmd(t, f.dir, "add", "alpha.txt", "notes.md")
	gitCmd(t, f.dir, "diff", "--cached", "--stat")

	alphaEntriesBefore := f.treeEntries(t, "HEAD")["alpha.txt"]
	notesEntriesBefore := f.treeEntries(t, "HEAD")["notes.md"]
	indexBefore := f.snapshot(t).IndexSHA256
	headBefore := f.head(t)
	worktreeBeforeAlpha := f.worktreeBytes(t, "alpha.txt")
	worktreeBeforeNotes := f.worktreeBytes(t, "notes.md")

	result, err := f.repo.UnstageFile(context.Background(), f.unstageRequest(t, "alpha.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if result.BeforeIndexSHA256 != indexBefore {
		t.Fatalf("before index identity %q != bound %q", result.BeforeIndexSHA256, indexBefore)
	}
	if result.AfterIndexSHA256 == result.BeforeIndexSHA256 {
		t.Fatal("UNSTAGE FILE did not change index identity")
	}
	if result.Path != "alpha.txt" || result.HeadCommit != headBefore {
		t.Fatalf("unexpected unstage identity: %+v", result)
	}
	if result.RestoredMode+" "+result.RestoredObjectID != alphaEntriesBefore {
		t.Fatalf("restored entry %q %q != HEAD entry %q", result.RestoredMode, result.RestoredObjectID, alphaEntriesBefore)
	}

	index := f.indexEntries(t)
	if index["alpha.txt"] != alphaEntriesBefore {
		t.Fatalf("alpha index entry %q != HEAD entry %q", index["alpha.txt"], alphaEntriesBefore)
	}
	if index["notes.md"] == notesEntriesBefore {
		t.Fatalf("unstage of alpha.txt also reverted notes.md: %q", index["notes.md"])
	}
	if got := f.worktreeBytes(t, "alpha.txt"); !bytes.Equal(got, worktreeBeforeAlpha) {
		t.Fatalf("worktree alpha.txt changed by unstage: %q -> %q", worktreeBeforeAlpha, got)
	}
	if got := f.worktreeBytes(t, "notes.md"); !bytes.Equal(got, worktreeBeforeNotes) {
		t.Fatalf("worktree notes.md changed by unstage: %q -> %q", worktreeBeforeNotes, got)
	}
	if got := f.head(t); got != headBefore {
		t.Fatalf("HEAD moved on unstage: %q -> %q", headBefore, got)
	}
	after := f.snapshot(t)
	if len(after.Staged) != 1 || after.Staged[0].Path != "notes.md" {
		t.Fatalf("staged view after unstage: %+v", after.Staged)
	}
	if len(after.Unstaged) != 1 || after.Unstaged[0].Path != "alpha.txt" {
		t.Fatalf("unstaged view after unstage: %+v", after.Unstaged)
	}
}

func TestUnstageFileRejectsStaleIdentityAndNonStagedTargets(t *testing.T) {
	f := newPhase4Fixture(t)
	f.write(t, "alpha.txt", "alpha staged\n")
	gitCmd(t, f.dir, "add", "alpha.txt")
	bound := f.unstageRequest(t, "alpha.txt")

	t.Run("wrong index", func(t *testing.T) {
		request := bound
		request.ExpectedIndexSHA256 = strings.Repeat("a", 64)
		_, err := f.repo.UnstageFile(context.Background(), request)
		requireErrorIs(t, err, ErrGitIndexConflict)
	})
	t.Run("wrong head", func(t *testing.T) {
		request := bound
		request.ExpectedHeadCommit = strings.Repeat("b", 40)
		_, err := f.repo.UnstageFile(context.Background(), request)
		requireErrorIs(t, err, ErrGitHeadConflict)
	})
	t.Run("wrong branch", func(t *testing.T) {
		request := bound
		request.ExpectedBranch = "side-branch"
		_, err := f.repo.UnstageFile(context.Background(), request)
		requireErrorIs(t, err, ErrGitBranchConflict)
	})
	t.Run("wrong repository", func(t *testing.T) {
		request := bound
		request.RepositoryID = strings.Repeat("c", 64)
		_, err := f.repo.UnstageFile(context.Background(), request)
		requireErrorIs(t, err, ErrGitRepositoryConflict)
	})
	t.Run("adversarial index drift", func(t *testing.T) {
		extra := f.unstageRequest(t, "alpha.txt")
		f.write(t, "notes.md", "# Notes drifted\n")
		gitCmd(t, f.dir, "add", "notes.md")
		_, err := f.repo.UnstageFile(context.Background(), extra)
		requireErrorIs(t, err, ErrGitIndexConflict)
		gitCmd(t, f.dir, "restore", "--staged", "notes.md")
	})
	t.Run("not staged", func(t *testing.T) {
		f.write(t, "notes.md", "# Notes plain edit\n")
		_, err := f.repo.UnstageFile(context.Background(), f.unstageRequest(t, "notes.md"))
		requireErrorIs(t, err, ErrGitUnsupportedTarget)
	})
}

func TestCommitStagedCreatesExactlyOneOneParentCommitFromBoundIndex(t *testing.T) {
	f := newPhase4Fixture(t)
	f.write(t, "alpha.txt", "alpha staged for commit\n")
	if _, err := f.repo.StageFile(context.Background(), f.stageRequest(t, "alpha.txt")); err != nil {
		t.Fatal(err)
	}
	request := f.commitRequest(t, "phase four local commit\n")
	headBefore := f.head(t)
	countBefore := strings.TrimSpace(gitCmd(t, f.dir, "rev-list", "--count", "HEAD"))
	indexBefore := request.ExpectedIndexSHA256
	boundIndex := f.indexEntries(t)

	result, err := f.repo.CommitStaged(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if result.ExpectedHeadCommit != headBefore {
		t.Fatalf("expected head %q != bound %q", result.ExpectedHeadCommit, headBefore)
	}
	if result.HeadCommit != result.NewHeadCommit || !validObjectID(result.NewHeadCommit) {
		t.Fatalf("unexpected new head: %+v", result)
	}
	if result.NewHeadCommit == headBefore {
		t.Fatal("COMMIT STAGED did not advance HEAD")
	}
	if result.BranchRef != "refs/heads/main" || result.Branch != "main" {
		t.Fatalf("unexpected branch identity: %+v", result)
	}
	if result.ParentCommit != headBefore {
		t.Fatalf("parent %q != expected head %q", result.ParentCommit, headBefore)
	}
	if !validObjectID(result.TreeID) {
		t.Fatalf("tree id %q is not an object id", result.TreeID)
	}
	if len(result.StagedPaths) != 1 || result.StagedPaths[0] != "alpha.txt" {
		t.Fatalf("staged paths %v", result.StagedPaths)
	}
	if result.BeforeIndexSHA256 != indexBefore {
		t.Fatalf("before index identity %q != bound %q", result.BeforeIndexSHA256, indexBefore)
	}
	if !validSHA256(result.CommitMessageSHA256) {
		t.Fatalf("commit message digest %q is not a sha256", result.CommitMessageSHA256)
	}

	if got := f.head(t); got != result.NewHeadCommit {
		t.Fatalf("HEAD is %q, result claimed %q", got, result.NewHeadCommit)
	}
	countAfter := strings.TrimSpace(gitCmd(t, f.dir, "rev-list", "--count", "HEAD"))
	if countBefore != "1" || countAfter != "2" {
		t.Fatalf("commit count %q -> %q, expected exactly one new commit", countBefore, countAfter)
	}
	body := gitCmd(t, f.dir, "cat-file", "commit", result.NewHeadCommit)
	parents := 0
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "parent ") {
			parents++
			if strings.TrimPrefix(line, "parent ") != headBefore {
				t.Fatalf("commit parent %q != %q", line, headBefore)
			}
		}
		if line == "" {
			break
		}
	}
	if parents != 1 {
		t.Fatalf("commit has %d parents, want 1", parents)
	}
	requireEqualStringMaps(t, "commit tree", boundIndex, f.treeEntries(t, result.NewHeadCommit))
	if got := f.commitMessage(t, result.NewHeadCommit); got != "phase four local commit\n" {
		t.Fatalf("commit message %q is not the exact supplied bytes", got)
	}
	after := f.snapshot(t)
	if len(after.Staged) != 0 {
		t.Fatalf("staged entries survived commit: %+v", after.Staged)
	}
	if !after.Clean {
		t.Fatalf("repository should be clean after commit: %+v", after.Unstaged)
	}
}

func TestCommitStagedAdvancesOnlyTheBoundBranch(t *testing.T) {
	f := newPhase4Fixture(t)
	f.write(t, "alpha.txt", "alpha staged for branch test\n")
	if _, err := f.repo.StageFile(context.Background(), f.stageRequest(t, "alpha.txt")); err != nil {
		t.Fatal(err)
	}
	refsBefore := f.refs(t)
	request := f.commitRequest(t, "branch bound commit\n")
	result, err := f.repo.CommitStaged(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	refsAfter := f.refs(t)
	changed := []string{}
	for name, value := range refsAfter {
		if refsBefore[name] != value {
			changed = append(changed, name)
		}
	}
	sort.Strings(changed)
	if len(changed) != 1 || changed[0] != "refs/heads/main" {
		t.Fatalf("changed refs %v", changed)
	}
	if refsAfter["refs/heads/main"] != result.NewHeadCommit {
		t.Fatalf("main is %q, result claimed %q", refsAfter["refs/heads/main"], result.NewHeadCommit)
	}
	if refsAfter["refs/heads/side-branch"] != refsBefore["refs/heads/side-branch"] {
		t.Fatal("side-branch moved")
	}
	if refsAfter["refs/tags/phase4-tag"] != refsBefore["refs/tags/phase4-tag"] {
		t.Fatal("tag moved")
	}
	if strings.TrimSpace(gitCmd(t, f.dir, "rev-parse", "side-branch")) != refsBefore["refs/heads/side-branch"] {
		t.Fatal("side-branch ref content changed")
	}
}

func TestCommitStagedRunsNoHooksEditorOrSigningPath(t *testing.T) {
	f := newPhase4Fixture(t)
	markerDir := t.TempDir()
	installTripwireHooks(t, f.dir, markerDir,
		"pre-commit", "commit-msg", "prepare-commit-msg", "post-commit", "pre-applypatch", "post-applypatch")
	gitCmd(t, f.dir, "config", "commit.gpgsign", "true")
	gitCmd(t, f.dir, "config", "gpg.format", "openpgp")
	gitCmd(t, f.dir, "config", "user.signingkey", "0000000000000000000000000000000000000000")

	editor := filepath.Join(t.TempDir(), "tripwire-editor.sh")
	if err := os.WriteFile(editor, []byte("#!/bin/sh\n: > '"+filepath.Join(markerDir, "editor")+"'\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_EDITOR", editor)
	t.Setenv("EDITOR", editor)
	t.Setenv("GIT_OPTIONAL_LOCKS", "0")

	f.write(t, "alpha.txt", "alpha staged for tripwire\n")
	if _, err := f.repo.StageFile(context.Background(), f.stageRequest(t, "alpha.txt")); err != nil {
		t.Fatal(err)
	}
	result, err := f.repo.CommitStaged(context.Background(), f.commitRequest(t, "no hooks no signing\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got := tripwireMarkers(t, markerDir); len(got) != 0 {
		t.Fatalf("hidden execution markers created: %v", got)
	}
	body := gitCmd(t, f.dir, "cat-file", "commit", result.NewHeadCommit)
	if strings.Contains(body, "gpgsig") {
		t.Fatalf("commit is signed:\n%s", body)
	}
	if strings.Contains(body, "gpgsig-sha256") {
		t.Fatalf("commit carries a signature header:\n%s", body)
	}
}

func TestMutatingGitRunnerEnvironmentNeverDisablesLocksOrInvokesAnEditor(t *testing.T) {
	env := mutateGitEnv([]string{
		"PATH=/usr/bin",
		"HOME=/home/example",
		"GIT_OPTIONAL_LOCKS=0",
		"GIT_EDITOR=sneaky-editor",
		"EDITOR=sneaky-editor",
		"GIT_PAGER=less",
		"PAGER=less",
		"GIT_TERMINAL_PROMPT=1",
		"LC_ALL=en_US.UTF-8",
	})
	joined := strings.Join(env, "\n")
	for _, forbidden := range []string{
		"GIT_OPTIONAL_LOCKS=0",
		"GIT_EDITOR=sneaky-editor",
		"EDITOR=sneaky-editor",
		"GIT_PAGER=less",
		"PAGER=less",
		"GIT_TERMINAL_PROMPT=1",
		"LC_ALL=en_US.UTF-8",
	} {
		if strings.Contains(joined, forbidden) {
			t.Fatalf("mutating environment kept inherited %q: %s", forbidden, joined)
		}
	}
	for _, required := range []string{
		"GIT_OPTIONAL_LOCKS=1",
		"GIT_TERMINAL_PROMPT=0",
		"GIT_PAGER=cat",
		"PAGER=cat",
		"LC_ALL=C",
		"GIT_EDITOR=:",
		"PATH=/usr/bin",
		"HOME=/home/example",
	} {
		if !strings.Contains(joined, required) {
			t.Fatalf("mutating environment missing %q: %s", required, joined)
		}
	}
	seen := map[string]bool{}
	for _, entry := range env {
		name, _, ok := strings.Cut(entry, "=")
		if !ok {
			t.Fatalf("unparsable environment entry %q", entry)
		}
		if seen[name] {
			t.Fatalf("duplicate environment key %q in %v", name, env)
		}
		seen[name] = true
	}
}

func TestCommitStagedRejectsUnsupportedGitStates(t *testing.T) {
	t.Run("detached", func(t *testing.T) {
		f := newPhase4Fixture(t)
		f.write(t, "alpha.txt", "alpha staged detached\n")
		gitCmd(t, f.dir, "add", "alpha.txt")
		gitCmd(t, f.dir, "checkout", "--detach", "HEAD")
		request := f.commitRequest(t, "detached\n")
		// A detached repository has no branch name to bind, so the browser can
		// only send the last branch it observed.
		request.ExpectedBranch = "main"
		_, err := f.repo.CommitStaged(context.Background(), request)
		requireErrorIs(t, err, ErrGitUnsupportedState)
	})
	t.Run("unborn", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "unborn")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		gitCmd(t, dir, "init", "-b", "main")
		gitCmd(t, dir, "config", "user.name", "Phase Four")
		gitCmd(t, dir, "config", "user.email", "phase4@example.invalid")
		if err := os.WriteFile(filepath.Join(dir, "alpha.txt"), []byte("alpha\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		gitCmd(t, dir, "add", "alpha.txt")
		repo := NewGitRepository(dir)
		snapshot, err := repo.Inspect(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if !snapshot.Unborn {
			t.Fatal("fixture is not unborn")
		}
		_, err = repo.CommitStaged(context.Background(), CommitStagedRequest{
			RepositoryID:        snapshot.RepositoryID,
			ExpectedHeadCommit:  strings.Repeat("a", 40),
			ExpectedBranch:      snapshot.Branch,
			ExpectedIndexSHA256: snapshot.IndexSHA256,
			CommitMessage:       "unborn\n",
		})
		requireErrorIs(t, err, ErrGitUnsupportedState)
	})
	t.Run("index unavailable", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "no index")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		gitCmd(t, dir, "init", "-b", "main")
		gitCmd(t, dir, "config", "user.name", "Phase Four")
		gitCmd(t, dir, "config", "user.email", "phase4@example.invalid")
		gitCmd(t, dir, "commit", "-m", "empty", "--allow-empty", "--no-gpg-sign")
		repo := NewGitRepository(dir)
		if err := os.Remove(filepath.Join(dir, ".git", "index")); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		snapshot, err := repo.Inspect(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if snapshot.IndexSHA256 != "" {
			t.Skipf("git recreated the index during inspection (%q); cannot assert the unavailable-index form", snapshot.IndexSHA256)
		}
		_, err = repo.CommitStaged(context.Background(), CommitStagedRequest{
			RepositoryID:        snapshot.RepositoryID,
			ExpectedHeadCommit:  snapshot.HeadCommit,
			ExpectedBranch:      snapshot.Branch,
			ExpectedIndexSHA256: strings.Repeat("a", 64),
			CommitMessage:       "no index\n",
		})
		requireErrorIs(t, err, ErrGitUnsupportedState)
	})
	for _, marker := range []string{
		"MERGE_HEAD",
		"CHERRY_PICK_HEAD",
		"REVERT_HEAD",
		"BISECT_START",
		"BISECT_LOG",
		"rebase-merge",
		"rebase-apply",
		"sequencer",
	} {
		marker := marker
		t.Run("in progress "+marker, func(t *testing.T) {
			f := newPhase4Fixture(t)
			f.write(t, "alpha.txt", "alpha staged in progress\n")
			if _, err := f.repo.StageFile(context.Background(), f.stageRequest(t, "alpha.txt")); err != nil {
				t.Fatal(err)
			}
			request := f.commitRequest(t, "in progress\n")
			target := filepath.Join(f.dir, ".git", marker)
			info, err := os.Stat(target)
			var err2 error
			if err == nil && info.IsDir() {
				err2 = os.MkdirAll(target, 0o755)
			} else {
				err2 = os.WriteFile(target, []byte(strings.Repeat("a", 40)+"\n"), 0o644)
			}
			if err2 != nil {
				t.Fatal(err2)
			}
			_, err = f.repo.CommitStaged(context.Background(), request)
			requireErrorIs(t, err, ErrGitUnsupportedState)
		})
	}
}

func TestCommitStagedRejectsEmptyAndNonModificationStagedState(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		f := newPhase4Fixture(t)
		_, err := f.repo.CommitStaged(context.Background(), f.commitRequest(t, "nothing\n"))
		requireErrorIs(t, err, ErrGitNothingToCommit)
	})
	t.Run("staged add", func(t *testing.T) {
		f := newPhase4Fixture(t)
		f.write(t, "added.txt", "added\n")
		gitCmd(t, f.dir, "add", "added.txt")
		_, err := f.repo.CommitStaged(context.Background(), f.commitRequest(t, "added\n"))
		requireErrorIs(t, err, ErrGitUnsupportedTarget)
	})
	t.Run("staged delete", func(t *testing.T) {
		f := newPhase4Fixture(t)
		if err := os.Remove(filepath.Join(f.dir, "notes.md")); err != nil {
			t.Fatal(err)
		}
		gitCmd(t, f.dir, "add", "--", "notes.md")
		_, err := f.repo.CommitStaged(context.Background(), f.commitRequest(t, "deleted\n"))
		requireErrorIs(t, err, ErrGitUnsupportedTarget)
	})
	t.Run("staged rename", func(t *testing.T) {
		f := newPhase4Fixture(t)
		gitCmd(t, f.dir, "mv", "alpha.txt", "renamed.txt")
		_, err := f.repo.CommitStaged(context.Background(), f.commitRequest(t, "renamed\n"))
		requireErrorIs(t, err, ErrGitUnsupportedTarget)
	})
	t.Run("unsafe attribute on staged path", func(t *testing.T) {
		f := newPhase4Fixture(t)
		f.write(t, "cleaned.dat", "data\n")
		f.write(t, ".gitattributes", "*.dat filter=tripscript\n")
		gitCmd(t, f.dir, "add", ".gitattributes", "cleaned.dat")
		gitCmd(t, f.dir, "commit", "-m", "attributes", "--no-gpg-sign")
		f.write(t, "cleaned.dat", "data changed\n")
		gitCmd(t, f.dir, "add", "cleaned.dat")
		_, err := f.repo.CommitStaged(context.Background(), f.commitRequest(t, "unsafe attribute\n"))
		requireErrorIs(t, err, ErrGitUnsafeAttributes)
	})
}

func TestCommitStagedRejectsStaleIdentity(t *testing.T) {
	f := newPhase4Fixture(t)
	f.write(t, "alpha.txt", "alpha staged stale\n")
	if _, err := f.repo.StageFile(context.Background(), f.stageRequest(t, "alpha.txt")); err != nil {
		t.Fatal(err)
	}
	bound := f.commitRequest(t, "stale identity\n")

	t.Run("wrong index", func(t *testing.T) {
		request := bound
		request.ExpectedIndexSHA256 = strings.Repeat("a", 64)
		_, err := f.repo.CommitStaged(context.Background(), request)
		requireErrorIs(t, err, ErrGitIndexConflict)
	})
	t.Run("wrong head", func(t *testing.T) {
		request := bound
		request.ExpectedHeadCommit = strings.Repeat("b", 40)
		_, err := f.repo.CommitStaged(context.Background(), request)
		requireErrorIs(t, err, ErrGitHeadConflict)
	})
	t.Run("wrong branch", func(t *testing.T) {
		request := bound
		request.ExpectedBranch = "side-branch"
		_, err := f.repo.CommitStaged(context.Background(), request)
		requireErrorIs(t, err, ErrGitBranchConflict)
	})
	t.Run("wrong repository", func(t *testing.T) {
		request := bound
		request.RepositoryID = strings.Repeat("c", 64)
		_, err := f.repo.CommitStaged(context.Background(), request)
		requireErrorIs(t, err, ErrGitRepositoryConflict)
	})
	t.Run("adversarial index drift", func(t *testing.T) {
		f.write(t, "notes.md", "# Notes drifted\n")
		gitCmd(t, f.dir, "add", "notes.md")
		_, err := f.repo.CommitStaged(context.Background(), bound)
		requireErrorIs(t, err, ErrGitIndexConflict)
		gitCmd(t, f.dir, "restore", "--staged", "notes.md")
	})
	t.Run("adversarial head drift", func(t *testing.T) {
		gitCmd(t, f.dir, "commit", "-m", "external", "--no-gpg-sign")
		_, err := f.repo.CommitStaged(context.Background(), bound)
		requireErrorIs(t, err, ErrGitHeadConflict)
	})
}

func TestCommitStagedRejectsInvalidMessages(t *testing.T) {
	f := newPhase4Fixture(t)
	f.write(t, "alpha.txt", "alpha staged message\n")
	if _, err := f.repo.StageFile(context.Background(), f.stageRequest(t, "alpha.txt")); err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{
		"empty":           "",
		"whitespace only": "  \n\t\n",
		"nul byte":        "message\x00with nul\n",
		"invalid utf8":    string([]byte{0xff, 0xfe, 0x41}),
		"oversized":       strings.Repeat("a", maxCommitMessageBytes+1),
	}
	for name, message := range cases {
		t.Run(name, func(t *testing.T) {
			request := f.commitRequest(t, "unused\n")
			request.CommitMessage = message
			_, err := f.repo.CommitStaged(context.Background(), request)
			requireErrorIs(t, err, ErrGitCommitMessageInvalid)
		})
	}
	if got := f.head(t); got != f.snapshot(t).HeadCommit {
		t.Fatal("invalid message attempts advanced HEAD")
	}
}

func TestCommitStagedPreservesUnrelatedUnstagedChanges(t *testing.T) {
	f := newPhase4Fixture(t)
	f.write(t, "alpha.txt", "alpha committed\n")
	f.write(t, "notes.md", "# Notes left alone\n")
	if _, err := f.repo.StageFile(context.Background(), f.stageRequest(t, "alpha.txt")); err != nil {
		t.Fatal(err)
	}
	notesBefore := f.worktreeBytes(t, "notes.md")
	result, err := f.repo.CommitStaged(context.Background(), f.commitRequest(t, "only alpha\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got := f.worktreeBytes(t, "notes.md"); !bytes.Equal(got, notesBefore) {
		t.Fatalf("unrelated worktree change was rewritten: %q -> %q", notesBefore, got)
	}
	after := f.snapshot(t)
	if len(after.Staged) != 0 {
		t.Fatalf("staged entries survived commit: %+v", after.Staged)
	}
	if len(after.Unstaged) != 1 || after.Unstaged[0].Path != "notes.md" {
		t.Fatalf("unrelated unstaged change lost: %+v", after.Unstaged)
	}
	if got := gitCmd(t, f.dir, "show", "--stat", "--format=%H", result.NewHeadCommit); !strings.Contains(got, "alpha.txt") || strings.Contains(got, "notes.md") {
		t.Fatalf("commit touched unexpected paths:\n%s", got)
	}
}

func TestGitControlCycleKeepsRemoteConfigurationAndRemoteRefsUntouched(t *testing.T) {
	f := newPhase4Fixture(t)
	remotesBefore := gitCmd(t, f.dir, "remote", "-v")
	configBefore := gitCmd(t, f.dir, "config", "--local", "--list")
	remoteRefsBefore := gitCmd(t, f.dir, "for-each-ref", "refs/remotes")
	if strings.TrimSpace(remoteRefsBefore) != "" {
		t.Fatalf("fixture unexpectedly has remote refs: %q", remoteRefsBefore)
	}

	f.write(t, "alpha.txt", "alpha cycle\n")
	first := f.snapshot(t).IndexSHA256
	if _, err := f.repo.StageFile(context.Background(), f.stageRequest(t, "alpha.txt")); err != nil {
		t.Fatal(err)
	}
	staged := f.snapshot(t).IndexSHA256
	if staged == first {
		t.Fatal("index identity did not change on stage")
	}
	if _, err := f.repo.UnstageFile(context.Background(), f.unstageRequest(t, "alpha.txt")); err != nil {
		t.Fatal(err)
	}
	unstaged := f.snapshot(t).IndexSHA256
	if unstaged == staged {
		t.Fatal("index identity did not change on unstage")
	}
	if _, err := f.repo.StageFile(context.Background(), f.stageRequest(t, "alpha.txt")); err != nil {
		t.Fatal(err)
	}
	restaged := f.snapshot(t).IndexSHA256
	if restaged == unstaged {
		t.Fatal("index identity did not change on re-stage")
	}
	if _, err := f.repo.CommitStaged(context.Background(), f.commitRequest(t, "cycle commit\n")); err != nil {
		t.Fatal(err)
	}

	if got := gitCmd(t, f.dir, "remote", "-v"); got != remotesBefore {
		t.Fatalf("remote configuration changed:\n%s\n%s", remotesBefore, got)
	}
	if got := gitCmd(t, f.dir, "config", "--local", "--list"); got != configBefore {
		t.Fatalf("local configuration changed:\n%s\n%s", configBefore, got)
	}
	if got := gitCmd(t, f.dir, "for-each-ref", "refs/remotes"); got != remoteRefsBefore {
		t.Fatalf("remote refs changed: %q -> %q", remoteRefsBefore, got)
	}
	if _, err := os.Stat(f.remote); err != nil {
		t.Fatalf("remote directory missing: %v", err)
	}
	if out, err := exec.Command("git", "-C", f.remote, "rev-parse", "--verify", "refs/heads/main").CombinedOutput(); err == nil {
		t.Fatalf("remote branch was created: %s", out)
	}
}

// The recorded argv is the only honest evidence that Phase-4 mutation runs
// through Git plumbing. The guard this test replaced relied on a
// repository-local `alias.add` booby trap, which is not a guard at all: Git
// resolves a built-in command before alias expansion, so the alias can never
// fire and a "armed" trap proved nothing. A PATH shim observes the real
// command line instead.
//
// Invocations are separated by argvRecordSeparator and the arguments of one
// invocation by argvTokenSeparator, so the log preserves exact token
// boundaries. That separation is also what makes assertion 5 true: an
// invocation recorded as distinct tokens cannot be a shell string, because
// execve receives an argument array and the shim only ever sees argv slots.
const (
	argvTokenSeparator  = "\x1f"
	argvRecordSeparator = "\x1e"
	gitArgvLogEnv       = "LLM_HUB_PHASE4_GIT_ARGV_LOG"
)

// porcelainMutationSubcommands are the porcelain entry points that would let
// repository configuration, hooks, an editor, or a clean filter act as a hidden
// execution or normalization path. Phase-4 mutation must never invoke one.
var porcelainMutationSubcommands = []string{
	"add", "am", "apply", "branch", "checkout", "cherry-pick", "clean", "commit",
	"fetch", "merge", "mv", "pull", "push", "rebase", "reset", "restore", "revert",
	"rm", "stash", "switch", "tag", "notes",
}

// requiredPlumbingSubcommands are the explicit plumbing calls the guarded cycle
// must issue. Their presence, together with the absence of every porcelain
// mutation, is what "plumbing only" means in this implementation.
var requiredPlumbingSubcommands = []string{
	"check-attr", "hash-object", "update-index", "ls-tree", "write-tree", "commit-tree", "update-ref",
}

// mutatingPlumbingSubcommands are the calls Phase 4 issues only through
// gitMutate, which is the single path that carries the safety -c configuration.
var mutatingPlumbingSubcommands = []string{
	"hash-object", "update-index", "write-tree", "commit-tree", "update-ref",
}

// requiredSafetyConfigFlags must accompany every mutating plumbing call so a
// repository configuration cannot turn plumbing into hooks, fsmonitor, or
// signing.
var requiredSafetyConfigFlags = []string{
	"core.fsmonitor=false",
	"core.hooksPath=/dev/null",
	"commit.gpgsign=false",
}

// gitArgvRecorder is a PATH shim that appends the argv of every git invocation
// the process makes, then delegates to the real binary.
type gitArgvRecorder struct {
	logPath string
}

// installGitArgvRecorder prepends a shim directory to PATH for the duration of
// the test. LookPath runs first, before the shim is on PATH, so the script can
// bake in the absolute real binary and can never recurse into itself.
func installGitArgvRecorder(t *testing.T) *gitArgvRecorder {
	t.Helper()

	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("resolve the real git binary before shimming PATH: %v", err)
	}
	if !filepath.IsAbs(realGit) {
		t.Fatalf("LookPath(%q) returned the non-absolute path %q", "git", realGit)
	}

	shimDir := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "git-argv.log")
	script := strings.Join([]string{
		"#!/bin/sh",
		"{",
		"  for arg in \"$@\"; do printf '%s" + argvTokenSeparator + "' \"$arg\"; done",
		"  printf '" + argvRecordSeparator + "'",
		"} >> \"$" + gitArgvLogEnv + "\"",
		"exec " + shellSingleQuote(realGit) + " \"$@\"",
		"",
	}, "\n")
	if err := os.WriteFile(filepath.Join(shimDir, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	// The product code builds cmd.Env from os.Environ(), so a t.Setenv value
	// reaches the shim as well as the git process it delegates to.
	t.Setenv(gitArgvLogEnv, logPath)
	t.Setenv("PATH", shimDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return &gitArgvRecorder{logPath: logPath}
}

// shellSingleQuote wraps a value in POSIX single quotes so a resolved path can
// never change the meaning of the generated shim script.
func shellSingleQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

// reset empties the recorded argv so the test observes only the calls it is
// about to make, not the fixture setup that built the repository.
func (r *gitArgvRecorder) reset(t *testing.T) {
	t.Helper()
	if err := os.WriteFile(r.logPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
}

// invocations returns every recorded invocation as its own token slice.
func (r *gitArgvRecorder) invocations(t *testing.T) [][]string {
	t.Helper()
	raw, err := os.ReadFile(r.logPath)
	if err != nil {
		t.Fatal(err)
	}
	trimmed := strings.TrimSuffix(string(raw), argvRecordSeparator)
	if trimmed == "" {
		return nil
	}
	records := strings.Split(trimmed, argvRecordSeparator)
	invocations := make([][]string, 0, len(records))
	for _, record := range records {
		invocations = append(invocations, strings.Split(record, argvTokenSeparator))
	}
	return invocations
}

// recordedGitSubcommand returns the actual subcommand token of a recorded
// invocation, skipping global options and the values they consume. Comparison is
// against this single token only, so "commit-tree", "update-ref", "write-tree",
// and "hash-object" are never mistaken for the "commit" or "reset" they contain.
func recordedGitSubcommand(args []string) (string, bool) {
	for i := 0; i < len(args); i++ {
		token := args[i]
		if token == "-C" || token == "-c" {
			i++ // the option consumes the following token as its value
			continue
		}
		if strings.HasPrefix(token, "-") {
			continue
		}
		return token, true
	}
	return "", false
}

func formatGitArgv(args []string) string {
	return fmt.Sprintf("%q", args)
}

func hasGitConfigSetting(args []string, setting string) bool {
	for i := 0; i < len(args); i++ {
		if args[i] == "-c" && i+1 < len(args) && args[i+1] == setting {
			return true
		}
		if args[i] == "-c"+setting {
			return true
		}
	}
	return false
}

// runPhase4Cycle runs the guarded cycle the browser drives:
// STAGE FILE -> UNSTAGE FILE -> STAGE FILE -> COMMIT STAGED.
func runPhase4Cycle(t *testing.T, f *phase4Fixture, message string) {
	t.Helper()
	ctx := context.Background()
	if _, err := f.repo.StageFile(ctx, f.stageRequest(t, "alpha.txt")); err != nil {
		t.Fatalf("STAGE FILE: %v", err)
	}
	if _, err := f.repo.UnstageFile(ctx, f.unstageRequest(t, "alpha.txt")); err != nil {
		t.Fatalf("UNSTAGE FILE: %v", err)
	}
	if _, err := f.repo.StageFile(ctx, f.stageRequest(t, "alpha.txt")); err != nil {
		t.Fatalf("second STAGE FILE: %v", err)
	}
	if _, err := f.repo.CommitStaged(ctx, f.commitRequest(t, message)); err != nil {
		t.Fatalf("COMMIT STAGED: %v", err)
	}
}

func TestGitControlUsesPlumbingOnlyNeverPorcelainMutation(t *testing.T) {
	f := newPhase4Fixture(t)
	recorder := installGitArgvRecorder(t)
	f.write(t, "alpha.txt", "alpha plumbing\n")

	// Everything above is fixture setup. The recorder is reset here so the
	// assertions measure only the Phase-4 mutation window. The request helpers
	// below shell out through the same PATH shim because they inspect the
	// repository first, which is expected: these assertions are about which
	// mutating subcommands appear, not about the total call count.
	recorder.reset(t)
	runPhase4Cycle(t, f, "plumbing only\n")

	invocations := recorder.invocations(t)
	if len(invocations) == 0 {
		t.Fatal("argv recorder observed no git invocation at all: the PATH shim is not armed, so every assertion below would be vacuous")
	}

	// Assertion 1: no recorded invocation uses a porcelain mutation subcommand.
	observed := map[string]bool{}
	for _, args := range invocations {
		subcommand, ok := recordedGitSubcommand(args)
		if !ok {
			continue
		}
		observed[subcommand] = true
		if containsString(porcelainMutationSubcommands, subcommand) {
			t.Fatalf("assertion 1 (no porcelain mutation) failed: porcelain subcommand %q was invoked: %s", subcommand, formatGitArgv(args))
		}
	}

	// Assertion 2: the expected plumbing subcommands did appear.
	for _, want := range requiredPlumbingSubcommands {
		if !observed[want] {
			t.Fatalf("assertion 2 (required plumbing present) failed: %q never invoked; observed subcommands %v", want, sortedStrings(observed))
		}
	}

	// Assertion 3: no invocation carries a signing flag.
	for _, args := range invocations {
		for _, token := range args {
			switch {
			case token == "-S", strings.HasPrefix(token, "--gpg-sign"):
				t.Fatalf("assertion 3 (no signing flag) failed: signing flag %q present: %s", token, formatGitArgv(args))
			case len(token) > 2 && strings.HasPrefix(token, "-u"):
				t.Fatalf("assertion 3 (no signing flag) failed: signing key flag %q present: %s", token, formatGitArgv(args))
			}
		}
	}

	// Assertion 4: the safety -c configuration accompanies every mutating call.
	for _, args := range invocations {
		subcommand, ok := recordedGitSubcommand(args)
		if !ok || !containsString(mutatingPlumbingSubcommands, subcommand) {
			continue
		}
		for _, setting := range requiredSafetyConfigFlags {
			if !hasGitConfigSetting(args, setting) {
				t.Fatalf("assertion 4 (safety -c configuration) failed: mutating %q was invoked without -c %s: %s", subcommand, setting, formatGitArgv(args))
			}
		}
		if hasGitConfigSetting(args, "commit.gpgsign=true") {
			t.Fatalf("assertion 3 (no signing path) failed: %q re-enabled signing: %s", subcommand, formatGitArgv(args))
		}
	}

	// Assertion 5: no invocation is a shell string. Proven structurally, not by
	// a second assertion: the shim was invoked by execve, it received argv
	// slots rather than a command line, and it recorded those slots as
	// separate tokens. A shell-concatenated implementation would have shown up
	// as one argument token here.
}

func sortedStrings(set map[string]bool) []string {
	values := make([]string, 0, len(set))
	for value := range set {
		values = append(values, value)
	}
	sort.Strings(values)
	return values
}

func TestCommitStagedForwardsMessageBytesWithoutCommandInjection(t *testing.T) {
	f := newPhase4Fixture(t)
	f.write(t, "alpha.txt", "alpha dash message\n")
	if _, err := f.repo.StageFile(context.Background(), f.stageRequest(t, "alpha.txt")); err != nil {
		t.Fatal(err)
	}
	// A message that looks like Git command-line flags is data, never argv.
	message := "-m --amend --no-verify -F\nsecond line\n"
	result, err := f.repo.CommitStaged(context.Background(), f.commitRequest(t, message))
	if err != nil {
		t.Fatal(err)
	}
	if got := f.commitMessage(t, result.NewHeadCommit); got != message {
		t.Fatalf("commit message %q != supplied %q", got, message)
	}
	if result.ParentCommit != strings.TrimSpace(gitCmd(t, f.dir, "rev-parse", "HEAD~1")) {
		t.Fatal("message was not forwarded as message bytes")
	}
}

func TestCommitStagedMessageIsExactWithoutInventedSuffix(t *testing.T) {
	f := newPhase4Fixture(t)
	f.write(t, "alpha.txt", "alpha exact message\n")
	if _, err := f.repo.StageFile(context.Background(), f.stageRequest(t, "alpha.txt")); err != nil {
		t.Fatal(err)
	}
	message := "subject line\n\nbody line one\nbody line two\n"
	result, err := f.repo.CommitStaged(context.Background(), f.commitRequest(t, message))
	if err != nil {
		t.Fatal(err)
	}
	if got := f.commitMessage(t, result.NewHeadCommit); got != message {
		t.Fatalf("commit message %q != supplied %q", got, message)
	}
	if got := strings.TrimSpace(gitCmd(t, f.dir, "log", "-1", "--format=%s", result.NewHeadCommit)); got != "subject line" {
		t.Fatalf("subject %q", got)
	}
	if result.CommitMessageSHA256 != contentSHA256([]byte(message)) {
		t.Fatal("commit message digest does not match the supplied bytes")
	}
}
