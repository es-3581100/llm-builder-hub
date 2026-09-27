package workstation

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func documentForPath(t *testing.T, snapshot RepositorySnapshot, path string) RepositoryDocument {
	t.Helper()
	for _, document := range snapshot.Documents {
		if document.Path == path {
			return document
		}
	}
	t.Fatalf("missing repository document %q", path)
	return RepositoryDocument{}
}

func writeRequestFor(t *testing.T, repo *GitRepository, path, content string) WriteFileRequest {
	t.Helper()
	snapshot, err := repo.Inspect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	document := documentForPath(t, snapshot, path)
	return WriteFileRequest{
		RepositoryID:          snapshot.RepositoryID,
		DocumentID:            document.ID,
		Path:                  path,
		ExpectedContentSHA256: document.ContentSHA256,
		Content:               content,
	}
}

func TestWriteFileMutatesOnlyWorkingTreeAndPreservesBytesModeAndIndex(t *testing.T) {
	dir := initRepo(t, "repo")
	target := filepath.Join(dir, "alpha.txt")
	if err := os.Chmod(target, 0o755); err != nil {
		t.Fatal(err)
	}
	indexBefore := shaFile(t, filepath.Join(dir, ".git", "index"))

	repo := NewGitRepository(dir)
	request := writeRequestFor(t, repo, "alpha.txt", "line one\r\nline two\nno-final-newline")
	result, err := repo.WriteFile(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != request.Content {
		t.Fatalf("byte preservation failed: %q", got)
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Fatalf("mode changed: got %o", info.Mode().Perm())
	}
	indexAfter := shaFile(t, filepath.Join(dir, ".git", "index"))
	if indexAfter != indexBefore {
		t.Fatalf("index changed %s -> %s", indexBefore, indexAfter)
	}
	if result.BeforeContentSHA256 != request.ExpectedContentSHA256 {
		t.Fatalf("before hash=%s expected=%s", result.BeforeContentSHA256, request.ExpectedContentSHA256)
	}
	if result.AfterContentSHA256 != contentSHA256([]byte(request.Content)) {
		t.Fatalf("after hash=%s", result.AfterContentSHA256)
	}
	if result.Repository.Clean || len(result.Repository.Unstaged) != 1 || result.Repository.Unstaged[0].Path != "alpha.txt" {
		t.Fatalf("fresh repository snapshot does not prove ordinary working-tree diff: %+v", result.Repository.Unstaged)
	}
}

func TestWriteFileRejectsStaleExternalEditWithoutOverwritingIt(t *testing.T) {
	dir := initRepo(t, "repo")
	repo := NewGitRepository(dir)
	request := writeRequestFor(t, repo, "alpha.txt", "browser draft\n")
	external := []byte("external edit\n")
	if err := os.WriteFile(filepath.Join(dir, "alpha.txt"), external, 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := repo.WriteFile(context.Background(), request)
	if !errors.Is(err, ErrWriteContentConflict) {
		t.Fatalf("expected content conflict, got %v", err)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "alpha.txt"))
	if string(got) != string(external) {
		t.Fatalf("external edit was overwritten: %q", got)
	}
}

func TestWriteFileRejectsDeletedAndRenamedSource(t *testing.T) {
	t.Run("deleted", func(t *testing.T) {
		dir := initRepo(t, "repo")
		repo := NewGitRepository(dir)
		request := writeRequestFor(t, repo, "alpha.txt", "draft\n")
		if err := os.Remove(filepath.Join(dir, "alpha.txt")); err != nil {
			t.Fatal(err)
		}
		_, err := repo.WriteFile(context.Background(), request)
		if !errors.Is(err, ErrWriteContentConflict) {
			t.Fatalf("expected deleted-source conflict, got %v", err)
		}
	})

	t.Run("renamed", func(t *testing.T) {
		dir := initRepo(t, "repo")
		repo := NewGitRepository(dir)
		request := writeRequestFor(t, repo, "alpha.txt", "draft\n")
		if err := os.Rename(filepath.Join(dir, "alpha.txt"), filepath.Join(dir, "beta.txt")); err != nil {
			t.Fatal(err)
		}
		_, err := repo.WriteFile(context.Background(), request)
		if !errors.Is(err, ErrWriteContentConflict) {
			t.Fatalf("expected renamed-source conflict, got %v", err)
		}
		got, _ := os.ReadFile(filepath.Join(dir, "beta.txt"))
		if string(got) != "alpha\n" {
			t.Fatalf("renamed source changed: %q", got)
		}
	})
}

func TestWriteFileRejectsTraversalIdentityMismatchSymlinkAndBinary(t *testing.T) {
	dir := initRepo(t, "repo")
	repo := NewGitRepository(dir)
	good := writeRequestFor(t, repo, "alpha.txt", "draft\n")

	cases := []struct {
		name string
		edit func(WriteFileRequest) WriteFileRequest
		want error
	}{
		{
			name: "traversal",
			edit: func(r WriteFileRequest) WriteFileRequest { r.Path = "../outside.txt"; return r },
			want: ErrWriteInvalidRequest,
		},
		{
			name: "repository identity",
			edit: func(r WriteFileRequest) WriteFileRequest { r.RepositoryID = strings.Repeat("a", 64); return r },
			want: ErrWriteRepositoryConflict,
		},
		{
			name: "document identity",
			edit: func(r WriteFileRequest) WriteFileRequest { r.DocumentID = "git-not-the-document"; return r },
			want: ErrWriteDocumentConflict,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := repo.WriteFile(context.Background(), tc.edit(good))
			if !errors.Is(err, tc.want) {
				t.Fatalf("want %v got %v", tc.want, err)
			}
		})
	}

	if err := os.Symlink("alpha.txt", filepath.Join(dir, "link.txt")); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, dir, "add", "link.txt")
	snapshot, err := repo.Inspect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	linkRequest := WriteFileRequest{
		RepositoryID:          snapshot.RepositoryID,
		DocumentID:            stableRepositoryDocumentID(snapshot.RepositoryID, "link.txt"),
		Path:                  "link.txt",
		ExpectedContentSHA256: contentSHA256([]byte("alpha\n")),
		Content:               "draft\n",
	}
	if _, err := repo.WriteFile(context.Background(), linkRequest); !errors.Is(err, ErrWriteUnsupportedTarget) {
		t.Fatalf("expected symlink refusal, got %v", err)
	}

	binaryPath := filepath.Join(dir, "binary.bin")
	if err := os.WriteFile(binaryPath, []byte{0, 1, 2, 3}, 0o644); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, dir, "add", "binary.bin")
	snapshot, err = repo.Inspect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	binaryRequest := WriteFileRequest{
		RepositoryID:          snapshot.RepositoryID,
		DocumentID:            stableRepositoryDocumentID(snapshot.RepositoryID, "binary.bin"),
		Path:                  "binary.bin",
		ExpectedContentSHA256: contentSHA256([]byte{0, 1, 2, 3}),
		Content:               "text replacement\n",
	}
	if _, err := repo.WriteFile(context.Background(), binaryRequest); !errors.Is(err, ErrWriteUnsupportedTarget) {
		t.Fatalf("expected binary refusal, got %v", err)
	}
}

func TestWriteFileWriteFailureLeavesOriginalBytes(t *testing.T) {
	dir := initRepo(t, "repo")
	repo := NewGitRepository(dir)
	request := writeRequestFor(t, repo, "alpha.txt", "draft\n")
	repo.writeAtomic = func(string, []byte, os.FileMode) error {
		return errors.New("injected write failure")
	}

	_, err := repo.WriteFile(context.Background(), request)
	if err == nil || !strings.Contains(err.Error(), "injected write failure") {
		t.Fatalf("expected injected write failure, got %v", err)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "alpha.txt"))
	if string(got) != "alpha\n" {
		t.Fatalf("failed write changed original bytes: %q", got)
	}
}

func TestWriteFileDetectsReadbackMismatch(t *testing.T) {
	dir := initRepo(t, "repo")
	repo := NewGitRepository(dir)
	request := writeRequestFor(t, repo, "alpha.txt", "intended\n")
	repo.writeAtomic = func(path string, _ []byte, mode os.FileMode) error {
		return atomicReplaceFile(path, []byte("wrong bytes\n"), mode)
	}

	_, err := repo.WriteFile(context.Background(), request)
	if !errors.Is(err, ErrWriteReadbackMismatch) {
		t.Fatalf("expected readback mismatch, got %v", err)
	}
}
