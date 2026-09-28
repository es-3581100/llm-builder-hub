package workstation

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIndexIdentityIsDeterministicAndStableAcrossRepeatedInspection(t *testing.T) {
	dir := initRepo(t, "repo")
	first := inspectRepo(t, dir)
	if !validSHA256(first.IndexSHA256) {
		t.Fatalf("index identity missing or malformed: %q", first.IndexSHA256)
	}
	raw := shaFile(t, filepath.Join(dir, ".git", "index"))
	if first.IndexSHA256 != raw {
		t.Fatalf("index identity %q does not match index file sha256 %q", first.IndexSHA256, raw)
	}
	second := inspectRepo(t, dir)
	if second.IndexSHA256 != first.IndexSHA256 {
		t.Fatalf("index identity drifted across inspection %q -> %q", first.IndexSHA256, second.IndexSHA256)
	}
	if second.SnapshotSHA256 != first.SnapshotSHA256 {
		t.Fatal("stable repository state produced a different snapshot hash")
	}
}

func TestIndexIdentityIsEmptyWhenNoIndexExists(t *testing.T) {
	unborn := filepath.Join(t.TempDir(), "no index repo")
	if err := os.MkdirAll(unborn, 0o755); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, unborn, "init", "-b", "main")
	if _, err := os.Stat(filepath.Join(unborn, ".git", "index")); !os.IsNotExist(err) {
		t.Fatalf("expected no index file, stat err=%v", err)
	}
	got := inspectRepo(t, unborn)
	if got.IndexSHA256 != "" {
		t.Fatalf("expected empty index identity for a repository without an index, got %q", got.IndexSHA256)
	}
	if got.SnapshotSHA256 == "" || strings.TrimSpace(got.SnapshotSHA256) == "" {
		t.Fatal("snapshot hash must remain populated")
	}
}

func TestIndexIdentityUnchangedByPhase3WriteFile(t *testing.T) {
	dir := initRepo(t, "repo")
	repo := NewGitRepository(dir)
	before, err := repo.Inspect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	request := writeRequestFor(t, repo, "alpha.txt", "phase 3 write\n")
	if _, err := repo.WriteFile(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	after, err := repo.Inspect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if after.IndexSHA256 != before.IndexSHA256 {
		t.Fatalf("WRITE FILE changed index identity %q -> %q", before.IndexSHA256, after.IndexSHA256)
	}
	if after.SnapshotSHA256 == before.SnapshotSHA256 {
		t.Fatal("WRITE FILE must still change the repository snapshot hash")
	}
}
