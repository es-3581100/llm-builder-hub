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

// onDiskIndexSHA256 hashes the bytes a caller would actually find in
// .git/index, independently of any identity the product reports. Every
// Phase-4 index identity must describe exactly these bytes: a read-only
// inspection can refresh the Git index stat cache and rewrite the file, so an
// identity captured before that work describes bytes that no longer exist.
func onDiskIndexSHA256(t *testing.T, dir string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, ".git", "index"))
	if err != nil {
		t.Fatalf("read on-disk index: %v", err)
	}
	return contentSHA256(raw)
}

// staleStatIndexEntry rewrites one tracked index entry from its own HEAD tree
// entry, which is how the product's own staging path leaves .git/index. The
// entry keeps the committed content but loses its stat data, so the next
// read-only inspection has to refresh the stat cache and rewrite the file.
// This is deterministic: it does not depend on filesystem timestamp
// granularity.
func staleStatIndexEntry(t *testing.T, f *phase4Fixture, path string) {
	t.Helper()
	entry, ok := f.treeEntries(t, "HEAD")[path]
	if !ok {
		t.Fatalf("path %q missing from HEAD tree", path)
	}
	// treeEntries reports "<mode> <sha>"; --cacheinfo requires "<mode>,<sha>,<path>".
	gitCmd(t, f.dir, "update-index", "--cacheinfo", strings.ReplaceAll(entry, " ", ",")+","+path)
}

func TestIndexIdentityReportedAfterStageMatchesOnDiskIndex(t *testing.T) {
	f := newPhase4Fixture(t)
	f.write(t, "alpha.txt", "alpha staged\n")
	result, err := f.repo.StageFile(context.Background(), f.stageRequest(t, "alpha.txt"))
	if err != nil {
		t.Fatal(err)
	}
	disk := onDiskIndexSHA256(t, f.dir)
	if result.AfterIndexSHA256 != disk {
		t.Fatalf("STAGE FILE reported index identity %q but .git/index on disk is %q", result.AfterIndexSHA256, disk)
	}
	if result.Repository.IndexSHA256 != disk {
		t.Fatalf("STAGE FILE snapshot index identity %q but .git/index on disk is %q", result.Repository.IndexSHA256, disk)
	}
}

func TestIndexIdentityReportedAfterUnstageMatchesOnDiskIndex(t *testing.T) {
	f := newPhase4Fixture(t)
	f.write(t, "alpha.txt", "alpha staged\n")
	if _, err := f.repo.StageFile(context.Background(), f.stageRequest(t, "alpha.txt")); err != nil {
		t.Fatal(err)
	}
	// Reverting the worktree to the committed bytes leaves UNSTAGE FILE
	// restoring an index entry whose content matches the worktree, so the
	// read-only inspection that follows the restore refreshes the stat cache
	// and rewrites .git/index. Reading the identity before that work is the
	// defect this test guards.
	f.write(t, "alpha.txt", "alpha\n")
	result, err := f.repo.UnstageFile(context.Background(), f.unstageRequest(t, "alpha.txt"))
	if err != nil {
		t.Fatal(err)
	}
	disk := onDiskIndexSHA256(t, f.dir)
	if result.AfterIndexSHA256 != disk {
		t.Fatalf("UNSTAGE FILE reported index identity %q but .git/index on disk is %q", result.AfterIndexSHA256, disk)
	}
	if result.Repository.IndexSHA256 != disk {
		t.Fatalf("UNSTAGE FILE snapshot index identity %q but .git/index on disk is %q", result.Repository.IndexSHA256, disk)
	}
}

// After COMMIT STAGED the index identity is allowed to change: `git write-tree`
// legitimately rewrites .git/index to record the cache-tree. Only the reported
// identity has to describe the bytes that are on disk when the call returns.
func TestIndexIdentityReportedAfterCommitMatchesOnDiskIndex(t *testing.T) {
	f := newPhase4Fixture(t)
	f.write(t, "alpha.txt", "alpha staged for commit\n")
	if _, err := f.repo.StageFile(context.Background(), f.stageRequest(t, "alpha.txt")); err != nil {
		t.Fatal(err)
	}
	result, err := f.repo.CommitStaged(context.Background(), f.commitRequest(t, "phase four index identity\n"))
	if err != nil {
		t.Fatal(err)
	}
	disk := onDiskIndexSHA256(t, f.dir)
	if result.AfterIndexSHA256 != disk {
		t.Fatalf("COMMIT STAGED reported index identity %q but .git/index on disk is %q", result.AfterIndexSHA256, disk)
	}
	if result.Repository.IndexSHA256 != disk {
		t.Fatalf("COMMIT STAGED snapshot index identity %q but .git/index on disk is %q", result.Repository.IndexSHA256, disk)
	}
}

// This complements TestIndexIdentityIsDeterministicAndStableAcrossRepeatedInspection,
// which inspects a repository whose index is already settled. Here the index
// needs its stat cache refreshed, so a plain inspection rewrites .git/index and
// the reported identity has to describe the rewritten bytes.
func TestIndexIdentityReportedByPlainInspectMatchesOnDiskIndex(t *testing.T) {
	f := newPhase4Fixture(t)
	staleStatIndexEntry(t, f, "notes.md")
	first, err := f.repo.Inspect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	disk := onDiskIndexSHA256(t, f.dir)
	if !validSHA256(first.IndexSHA256) {
		t.Fatalf("index identity missing or malformed: %q", first.IndexSHA256)
	}
	if first.IndexSHA256 != disk {
		t.Fatalf("INSPECT reported index identity %q but .git/index on disk is %q", first.IndexSHA256, disk)
	}
}

func TestIndexIdentityIsStableAcrossRepeatedInspectionOfRefreshedIndex(t *testing.T) {
	f := newPhase4Fixture(t)
	staleStatIndexEntry(t, f, "notes.md")
	first, err := f.repo.Inspect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	second, err := f.repo.Inspect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	disk := onDiskIndexSHA256(t, f.dir)
	if second.IndexSHA256 != first.IndexSHA256 {
		t.Fatalf("index identity drifted across repeated inspection %q -> %q", first.IndexSHA256, second.IndexSHA256)
	}
	if second.IndexSHA256 != disk {
		t.Fatalf("repeated INSPECT reported index identity %q but .git/index on disk is %q", second.IndexSHA256, disk)
	}
	if second.SnapshotSHA256 != first.SnapshotSHA256 {
		t.Fatal("stable repository state produced a different snapshot hash")
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
