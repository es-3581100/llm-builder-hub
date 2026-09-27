package workstation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

var (
	ErrWriteInvalidRequest      = errors.New("invalid write request")
	ErrWriteRepositoryConflict  = errors.New("repository identity conflict")
	ErrWriteDocumentConflict    = errors.New("document identity conflict")
	ErrWriteContentConflict     = errors.New("source content conflict")
	ErrWriteUnsupportedTarget   = errors.New("unsupported write target")
	ErrWriteReadbackMismatch    = errors.New("write readback mismatch")
)

type WriteFileRequest struct {
	RepositoryID          string `json:"repository_id"`
	DocumentID            string `json:"document_id"`
	Path                  string `json:"path"`
	ExpectedContentSHA256 string `json:"expected_content_sha256"`
	Content               string `json:"content"`
}

type WriteFileResult struct {
	RepositoryID        string             `json:"repository_id"`
	DocumentID          string             `json:"document_id"`
	Path                string             `json:"path"`
	BeforeContentSHA256 string             `json:"before_content_sha256"`
	AfterContentSHA256  string             `json:"after_content_sha256"`
	BytesWritten        int64              `json:"bytes_written"`
	Repository          RepositorySnapshot `json:"repository"`
}

func contentSHA256(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

func validSHA256(value string) bool {
	if len(value) != sha256.Size*2 || strings.ToLower(value) != value {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size
}

func canonicalWritePath(path string) (string, error) {
	if strings.ContainsRune(path, '\\') || !safeRepositoryPath(path) {
		return "", fmt.Errorf("%w: unsafe repository path %q", ErrWriteInvalidRequest, path)
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(path)))
	if clean != path {
		return "", fmt.Errorf("%w: repository path must be canonical: %q", ErrWriteInvalidRequest, path)
	}
	return clean, nil
}

func findSourceRelationship(snapshot RepositorySnapshot, path string) (SourceFileRelationship, bool) {
	for _, file := range snapshot.Files {
		if file.Path == path {
			return file, true
		}
	}
	return SourceFileRelationship{}, false
}

func findRepositoryDocument(snapshot RepositorySnapshot, id, path string) (RepositoryDocument, bool) {
	for _, document := range snapshot.Documents {
		if document.ID == id && document.Path == path {
			return document, true
		}
	}
	return RepositoryDocument{}, false
}

func validateWriteTarget(root, relPath string) (string, os.FileInfo, error) {
	fullPath := filepath.Join(root, filepath.FromSlash(relPath))
	relative, err := filepath.Rel(root, fullPath)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", nil, fmt.Errorf("%w: path escapes repository root", ErrWriteInvalidRequest)
	}

	current := root
	parts := strings.Split(filepath.FromSlash(relPath), string(filepath.Separator))
	for i, part := range parts {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if errors.Is(err, os.ErrNotExist) {
			return "", nil, fmt.Errorf("%w: source no longer exists: %s", ErrWriteContentConflict, relPath)
		}
		if err != nil {
			return "", nil, fmt.Errorf("stat write target %q: %w", relPath, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", nil, fmt.Errorf("%w: symlink component in %s", ErrWriteUnsupportedTarget, relPath)
		}
		if i < len(parts)-1 && !info.IsDir() {
			return "", nil, fmt.Errorf("%w: non-directory path component in %s", ErrWriteUnsupportedTarget, relPath)
		}
		if i == len(parts)-1 {
			if !info.Mode().IsRegular() {
				return "", nil, fmt.Errorf("%w: source is not a regular file: %s", ErrWriteUnsupportedTarget, relPath)
			}
			return fullPath, info, nil
		}
	}
	return "", nil, fmt.Errorf("%w: empty write target", ErrWriteInvalidRequest)
}

func atomicReplaceFile(path string, content []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	file, err := os.CreateTemp(dir, ".llm-hub-write-*")
	if err != nil {
		return err
	}
	tempPath := file.Name()
	cleanup := true
	defer func() {
		_ = file.Close()
		if cleanup {
			_ = os.Remove(tempPath)
		}
	}()

	if err := file.Chmod(mode); err != nil {
		return err
	}
	written, err := file.Write(content)
	if err != nil {
		return err
	}
	if written != len(content) {
		return io.ErrShortWrite
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempPath, path); err != nil {
		return err
	}
	cleanup = false

	if directory, err := os.Open(dir); err == nil {
		_ = directory.Sync()
		_ = directory.Close()
	}
	return nil
}

func (r *GitRepository) WriteFile(ctx context.Context, request WriteFileRequest) (WriteFileResult, error) {
	var result WriteFileResult

	path, err := canonicalWritePath(request.Path)
	if err != nil {
		return result, err
	}
	if request.RepositoryID == "" || request.DocumentID == "" || !validSHA256(request.ExpectedContentSHA256) {
		return result, fmt.Errorf("%w: repository_id, document_id, and lowercase sha256 are required", ErrWriteInvalidRequest)
	}
	next := []byte(request.Content)
	if int64(len(next)) > r.maxDocumentBytes {
		return result, fmt.Errorf("%w: replacement exceeds %d bytes", ErrWriteUnsupportedTarget, r.maxDocumentBytes)
	}
	if !isTextContent(next) {
		return result, fmt.Errorf("%w: replacement content must be UTF-8 text without NUL bytes", ErrWriteUnsupportedTarget)
	}

	before, err := r.Inspect(ctx)
	if err != nil {
		return result, err
	}
	if before.RepositoryID != request.RepositoryID {
		return result, fmt.Errorf("%w: expected %s got %s", ErrWriteRepositoryConflict, request.RepositoryID, before.RepositoryID)
	}

	relationship, ok := findSourceRelationship(before, path)
	if !ok {
		return result, fmt.Errorf("%w: path is not present in the current repository view: %s", ErrWriteDocumentConflict, path)
	}
	if !relationship.Exists {
		return result, fmt.Errorf("%w: source was deleted: %s", ErrWriteContentConflict, path)
	}
	if relationship.Symlink || !relationship.Regular || relationship.Binary || relationship.TooLarge {
		return result, fmt.Errorf("%w: source is not an editable text file: %s", ErrWriteUnsupportedTarget, path)
	}
	document, ok := findRepositoryDocument(before, request.DocumentID, path)
	if !ok || document.RepositoryID != request.RepositoryID {
		return result, fmt.Errorf("%w: document identity no longer matches %s", ErrWriteDocumentConflict, path)
	}
	if document.ContentSHA256 != request.ExpectedContentSHA256 {
		return result, fmt.Errorf("%w: expected %s got %s", ErrWriteContentConflict, request.ExpectedContentSHA256, document.ContentSHA256)
	}

	fullPath, initialInfo, err := validateWriteTarget(before.Root, path)
	if err != nil {
		return result, err
	}
	current, err := os.ReadFile(fullPath)
	if err != nil {
		return result, fmt.Errorf("read write target %q: %w", path, err)
	}
	currentSHA := contentSHA256(current)
	if currentSHA != request.ExpectedContentSHA256 {
		return result, fmt.Errorf("%w: expected %s got %s", ErrWriteContentConflict, request.ExpectedContentSHA256, currentSHA)
	}

	// Re-check immediately before replacement. This catches ordinary external
	// edits, chmod/replacement, deletion, and rename between inspection and write.
	latestInfo, err := os.Lstat(fullPath)
	if errors.Is(err, os.ErrNotExist) {
		return result, fmt.Errorf("%w: source disappeared before write: %s", ErrWriteContentConflict, path)
	}
	if err != nil {
		return result, fmt.Errorf("restat write target %q: %w", path, err)
	}
	if latestInfo.Mode()&os.ModeSymlink != 0 || !latestInfo.Mode().IsRegular() {
		return result, fmt.Errorf("%w: source type changed before write: %s", ErrWriteUnsupportedTarget, path)
	}
	if !os.SameFile(initialInfo, latestInfo) || initialInfo.Mode() != latestInfo.Mode() {
		return result, fmt.Errorf("%w: source identity or mode changed before write: %s", ErrWriteContentConflict, path)
	}
	latest, err := os.ReadFile(fullPath)
	if err != nil {
		return result, fmt.Errorf("reread write target %q: %w", path, err)
	}
	if contentSHA256(latest) != request.ExpectedContentSHA256 {
		return result, fmt.Errorf("%w: source changed before write: %s", ErrWriteContentConflict, path)
	}

	mode := initialInfo.Mode() & (os.ModePerm | os.ModeSetuid | os.ModeSetgid | os.ModeSticky)
	writer := r.writeAtomic
	if writer == nil {
		writer = atomicReplaceFile
	}
	if err := writer(fullPath, next, mode); err != nil {
		return result, fmt.Errorf("atomic write %q: %w", path, err)
	}

	afterBytes, err := os.ReadFile(fullPath)
	if err != nil {
		return result, fmt.Errorf("%w: readback failed for %s: %v", ErrWriteReadbackMismatch, path, err)
	}
	afterSHA := contentSHA256(afterBytes)
	expectedAfterSHA := contentSHA256(next)
	result = WriteFileResult{
		RepositoryID:        request.RepositoryID,
		DocumentID:          request.DocumentID,
		Path:                path,
		BeforeContentSHA256: currentSHA,
		AfterContentSHA256:  afterSHA,
		BytesWritten:        int64(len(next)),
	}
	if afterSHA != expectedAfterSHA || string(afterBytes) != request.Content {
		return result, fmt.Errorf("%w: expected %s got %s", ErrWriteReadbackMismatch, expectedAfterSHA, afterSHA)
	}

	after, err := r.Inspect(ctx)
	if err != nil {
		return result, fmt.Errorf("fresh repository inspection after write: %w", err)
	}
	if after.RepositoryID != request.RepositoryID {
		return result, fmt.Errorf("%w: repository identity changed after write", ErrWriteRepositoryConflict)
	}
	afterDocument, ok := findRepositoryDocument(after, request.DocumentID, path)
	if !ok || afterDocument.ContentSHA256 != expectedAfterSHA {
		return result, fmt.Errorf("%w: fresh repository view does not match readback", ErrWriteReadbackMismatch)
	}
	result.Repository = after
	return result, nil
}
