package workstation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func gitCmd(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "LC_ALL=C", "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

func initRepo(t *testing.T, name string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, dir, "init", "-b", "main")
	gitCmd(t, dir, "config", "user.name", "Phase Two")
	gitCmd(t, dir, "config", "user.email", "phase2@example.invalid")
	if err := os.WriteFile(filepath.Join(dir, "alpha.txt"), []byte("alpha\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.md"), []byte("# Notes\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, dir, "add", "alpha.txt", "notes.md")
	gitCmd(t, dir, "commit", "-m", "initial")
	return dir
}

func inspectRepo(t *testing.T, dir string) RepositorySnapshot {
	t.Helper()
	got, err := NewGitRepository(dir).Inspect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func findFile(s RepositorySnapshot, path string) (SourceFileRelationship, bool) {
	for _, f := range s.Files {
		if f.Path == path {
			return f, true
		}
	}
	return SourceFileRelationship{}, false
}

func shaFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

func TestRepositoryCleanAndStableDocumentIdentity(t *testing.T) {
	dir := initRepo(t, "repo")
	first := inspectRepo(t, dir)
	if first.Authority != "GIT" || !first.Clean || first.Branch != "main" || first.HeadCommit == "" || first.Detached || first.Unborn {
		t.Fatalf("unexpected clean snapshot: %+v", first)
	}
	if len(first.Documents) != 2 {
		t.Fatalf("documents=%d", len(first.Documents))
	}
	var id string
	for _, d := range first.Documents {
		if d.Path == "alpha.txt" {
			id = d.ID
		}
	}
	if id == "" {
		t.Fatal("missing alpha document")
	}
	if err := os.WriteFile(filepath.Join(dir, "alpha.txt"), []byte("alpha changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	second := inspectRepo(t, dir)
	var id2 string
	for _, d := range second.Documents {
		if d.Path == "alpha.txt" {
			id2 = d.ID
		}
	}
	if id2 != id {
		t.Fatalf("document identity drifted %q -> %q", id, id2)
	}
	if first.RepositoryID != second.RepositoryID {
		t.Fatal("repository identity changed on worktree edit")
	}
	if first.SnapshotSHA256 == second.SnapshotSHA256 {
		t.Fatal("snapshot hash did not change")
	}
}

func TestRepositoryAdversarialWorkingTreeStates(t *testing.T) {
	dir := initRepo(t, "repo with spaces")
	if got := inspectRepo(t, dir); !got.Clean {
		t.Fatal("initial repository not clean")
	}
	if err := os.WriteFile(filepath.Join(dir, "alpha.txt"), []byte("unstaged\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := inspectRepo(t, dir)
	if len(got.Unstaged) != 1 || len(got.Staged) != 0 || got.Clean {
		t.Fatalf("unstaged=%+v staged=%+v clean=%v", got.Unstaged, got.Staged, got.Clean)
	}
	gitCmd(t, dir, "add", "alpha.txt")
	got = inspectRepo(t, dir)
	if len(got.Staged) != 1 || len(got.Unstaged) != 0 {
		t.Fatalf("staged=%+v unstaged=%+v", got.Staged, got.Unstaged)
	}
	if err := os.WriteFile(filepath.Join(dir, "alpha.txt"), []byte("staged then unstaged\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got = inspectRepo(t, dir)
	if len(got.Staged) != 1 || len(got.Unstaged) != 1 {
		t.Fatalf("combined staged=%+v unstaged=%+v", got.Staged, got.Unstaged)
	}
	if err := os.WriteFile(filepath.Join(dir, "new file.txt"), []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got = inspectRepo(t, dir)
	if len(got.Untracked) != 1 || got.Untracked[0] != "new file.txt" {
		t.Fatalf("untracked=%v", got.Untracked)
	}
	if err := os.WriteFile(filepath.Join(dir, "binary.bin"), []byte{0, 1, 2, 3}, 0o644); err != nil {
		t.Fatal(err)
	}
	got = inspectRepo(t, dir)
	f, ok := findFile(got, "binary.bin")
	if !ok || !f.Binary || f.DocumentID != "" {
		t.Fatalf("binary relation=%+v ok=%v", f, ok)
	}
}

func TestRepositoryDetachedUnbornDeleteRenameAndHistory(t *testing.T) {
	dir := initRepo(t, "repo")
	gitCmd(t, dir, "checkout", "--detach", "HEAD")
	got := inspectRepo(t, dir)
	if !got.Detached || got.Branch != "" || got.HeadCommit == "" {
		t.Fatalf("detached snapshot=%+v", got)
	}
	gitCmd(t, dir, "checkout", "main")
	if err := os.Remove(filepath.Join(dir, "notes.md")); err != nil {
		t.Fatal(err)
	}
	got = inspectRepo(t, dir)
	f, ok := findFile(got, "notes.md")
	if !ok || f.Exists || f.UnstagedStatus == "" {
		t.Fatalf("deleted relationship=%+v ok=%v", f, ok)
	}
	gitCmd(t, dir, "restore", "notes.md")
	gitCmd(t, dir, "mv", "alpha.txt", "renamed.txt")
	gitCmd(t, dir, "add", "-A")
	got = inspectRepo(t, dir)
	if len(got.Staged) != 1 || got.Staged[0].Kind != "renamed" || got.Staged[0].PreviousPath != "alpha.txt" || got.Staged[0].Path != "renamed.txt" {
		t.Fatalf("rename=%+v", got.Staged)
	}
	f, ok = findFile(got, "renamed.txt")
	if !ok || f.PreviousPath != "alpha.txt" {
		t.Fatalf("rename relation=%+v", f)
	}
	gitCmd(t, dir, "commit", "-m", "rename")
	got = inspectRepo(t, dir)
	if len(got.History) < 2 {
		t.Fatalf("history=%+v", got.History)
	}
	unborn := filepath.Join(t.TempDir(), "new repo")
	if err := os.MkdirAll(unborn, 0o755); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, unborn, "init", "-b", "main")
	fresh := inspectRepo(t, unborn)
	if !fresh.Unborn || fresh.HeadCommit != "" || fresh.Branch != "main" || !fresh.Clean {
		t.Fatalf("unborn=%+v", fresh)
	}
}

func TestRepositoryInspectionDoesNotMutateIndex(t *testing.T) {
	dir := initRepo(t, "repo")
	index := filepath.Join(dir, ".git", "index")
	before := shaFile(t, index)
	_ = inspectRepo(t, dir)
	after := shaFile(t, index)
	if before != after {
		t.Fatalf("index changed %s -> %s", before, after)
	}
}

func TestRepositoryIdentityIsRepositoryScopedNotBasenameOnly(t *testing.T) {
	a := initRepo(t, "same")
	bbase := t.TempDir()
	b := filepath.Join(bbase, "same")
	if err := os.MkdirAll(b, 0o755); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, b, "init", "-b", "main")
	gitCmd(t, b, "config", "user.name", "Phase Two")
	gitCmd(t, b, "config", "user.email", "phase2@example.invalid")
	if err := os.WriteFile(filepath.Join(b, "alpha.txt"), []byte("alpha\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, b, "add", "alpha.txt")
	gitCmd(t, b, "commit", "-m", "initial")
	sa := inspectRepo(t, a)
	sb := inspectRepo(t, b)
	if filepath.Base(sa.Root) != filepath.Base(sb.Root) {
		t.Fatalf("test setup basenames differ: %q %q", sa.Root, sb.Root)
	}
	if sa.RepositoryID == sb.RepositoryID {
		t.Fatal("different repositories with same basename shared identity")
	}
}

func TestRepositoryRejectsNonRepository(t *testing.T) {
	_, err := NewGitRepository(t.TempDir()).Inspect(context.Background())
	if err == nil {
		t.Fatal("expected non-repository error")
	}
}
