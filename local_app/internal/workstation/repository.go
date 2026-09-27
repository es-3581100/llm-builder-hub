package workstation

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	defaultRepositoryDocumentLimit = 64
	defaultRepositoryDocumentBytes = int64(256 * 1024)
)

type RepositoryProvider interface {
	Inspect(context.Context) (RepositorySnapshot, error)
}

type GitRepository struct {
	root             string
	maxDocuments     int
	maxDocumentBytes int64
	writeAtomic      func(string, []byte, os.FileMode) error
}

type GitChange struct {
	Path         string `json:"path"`
	PreviousPath string `json:"previous_path,omitempty"`
	Status       string `json:"status"`
	Kind         string `json:"kind"`
}

type GitCommit struct {
	Commit  string `json:"commit"`
	Short   string `json:"short"`
	Date    string `json:"date"`
	Author  string `json:"author"`
	Subject string `json:"subject"`
}

type SourceFileRelationship struct {
	ID               string `json:"id"`
	Path             string `json:"path"`
	PreviousPath     string `json:"previous_path,omitempty"`
	Tracked          bool   `json:"tracked"`
	Untracked        bool   `json:"untracked"`
	Exists           bool   `json:"exists"`
	Regular          bool   `json:"regular"`
	Symlink          bool   `json:"symlink"`
	SymlinkTarget    string `json:"symlink_target,omitempty"`
	Size             int64  `json:"size,omitempty"`
	Binary           bool   `json:"binary"`
	TooLarge         bool   `json:"too_large"`
	DocumentID       string `json:"document_id,omitempty"`
	StagedStatus     string `json:"staged_status,omitempty"`
	UnstagedStatus   string `json:"unstaged_status,omitempty"`
	ContentAuthority string `json:"content_authority,omitempty"`
	ContentSHA256     string `json:"content_sha256,omitempty"`
}

type RepositoryDocument struct {
	ID           string `json:"id"`
	Path         string `json:"path"`
	Kind         string `json:"kind"`
	Title        string `json:"title"`
	Content      string `json:"content"`
	Size         int64  `json:"size"`
	RepositoryID  string `json:"repository_id"`
	Source        string `json:"source"`
	ContentSHA256 string `json:"content_sha256"`
}

type RepositorySnapshot struct {
	Authority            string                   `json:"authority"`
	RepositoryID         string                   `json:"repository_id"`
	SnapshotSHA256       string                   `json:"snapshot_sha256"`
	Root                 string                   `json:"root"`
	GitDir               string                   `json:"git_dir"`
	ObjectFormat         string                   `json:"object_format"`
	HeadCommit           string                   `json:"head_commit,omitempty"`
	HeadShort            string                   `json:"head_short,omitempty"`
	Branch               string                   `json:"branch,omitempty"`
	Detached             bool                     `json:"detached"`
	Unborn               bool                     `json:"unborn"`
	Clean                bool                     `json:"clean"`
	Staged               []GitChange              `json:"staged"`
	Unstaged             []GitChange              `json:"unstaged"`
	Untracked            []string                 `json:"untracked"`
	StagedDiff           string                   `json:"staged_diff"`
	UnstagedDiff         string                   `json:"unstaged_diff"`
	History              []GitCommit              `json:"history"`
	Files                []SourceFileRelationship `json:"files"`
	Documents            []RepositoryDocument     `json:"documents"`
	DocumentLimitReached bool                     `json:"document_limit_reached"`
	DocumentLimit        int                      `json:"document_limit"`
	MaxDocumentBytes     int64                    `json:"max_document_bytes"`
}

func NewGitRepository(root string) *GitRepository {
	return &GitRepository{
		root:             root,
		maxDocuments:     defaultRepositoryDocumentLimit,
		maxDocumentBytes: defaultRepositoryDocumentBytes,
		writeAtomic:      atomicReplaceFile,
	}
}

func (r *GitRepository) Inspect(ctx context.Context) (RepositorySnapshot, error) {
	configuredRoot, err := filepath.Abs(r.root)
	if err != nil {
		return RepositorySnapshot{}, fmt.Errorf("resolve repository root: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(configuredRoot); err == nil {
		configuredRoot = resolved
	}

	bareText, err := r.gitText(ctx, configuredRoot, "rev-parse", "--is-bare-repository")
	if err != nil {
		return RepositorySnapshot{}, fmt.Errorf("not a readable Git repository at %s: %w", configuredRoot, err)
	}
	bare := strings.TrimSpace(bareText) == "true"

	root := configuredRoot
	if !bare {
		if top, err := r.gitText(ctx, configuredRoot, "rev-parse", "--show-toplevel"); err == nil {
			root = strings.TrimSpace(top)
			if resolved, err := filepath.EvalSymlinks(root); err == nil {
				root = resolved
			}
		}
	}

	gitDirText, err := r.gitText(ctx, configuredRoot, "rev-parse", "--git-common-dir")
	if err != nil {
		return RepositorySnapshot{}, fmt.Errorf("resolve git directory: %w", err)
	}
	gitDir := strings.TrimSpace(gitDirText)
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(configuredRoot, gitDir)
	}
	gitDir = filepath.Clean(gitDir)
	if resolved, err := filepath.EvalSymlinks(gitDir); err == nil {
		gitDir = resolved
	}

	objectFormat := "sha1"
	if out, err := r.gitText(ctx, configuredRoot, "rev-parse", "--show-object-format"); err == nil && strings.TrimSpace(out) != "" {
		objectFormat = strings.TrimSpace(out)
	}
	rootsText, _ := r.gitText(ctx, configuredRoot, "rev-list", "--max-parents=0", "--all")
	roots := nonEmptyLines(rootsText)
	sort.Strings(roots)
	repositoryID := digestStrings("llm-hub-repository-v1", root, gitDir, objectFormat, strings.Join(roots, ","))

	snapshot := RepositorySnapshot{
		Authority:        "GIT",
		RepositoryID:     repositoryID,
		Root:             root,
		GitDir:           gitDir,
		ObjectFormat:     objectFormat,
		DocumentLimit:    r.maxDocuments,
		MaxDocumentBytes: r.maxDocumentBytes,
	}

	headText, headErr := r.gitText(ctx, configuredRoot, "rev-parse", "--verify", "HEAD")
	if headErr == nil {
		snapshot.HeadCommit = strings.TrimSpace(headText)
		if len(snapshot.HeadCommit) > 12 {
			snapshot.HeadShort = snapshot.HeadCommit[:12]
		} else {
			snapshot.HeadShort = snapshot.HeadCommit
		}
	} else {
		snapshot.Unborn = true
	}

	branchText, branchErr := r.gitText(ctx, configuredRoot, "symbolic-ref", "--quiet", "--short", "HEAD")
	if branchErr == nil {
		snapshot.Branch = strings.TrimSpace(branchText)
	} else if snapshot.HeadCommit != "" {
		snapshot.Detached = true
	}

	if !bare {
		stagedRaw, err := r.gitBytes(ctx, configuredRoot, "diff", "--cached", "--name-status", "-z", "--find-renames", "--no-ext-diff")
		if err != nil {
			return RepositorySnapshot{}, fmt.Errorf("inspect staged changes: %w", err)
		}
		unstagedRaw, err := r.gitBytes(ctx, configuredRoot, "diff", "--name-status", "-z", "--find-renames", "--no-ext-diff")
		if err != nil {
			return RepositorySnapshot{}, fmt.Errorf("inspect unstaged changes: %w", err)
		}
		snapshot.Staged, err = parseNameStatusZ(stagedRaw)
		if err != nil {
			return RepositorySnapshot{}, fmt.Errorf("parse staged changes: %w", err)
		}
		snapshot.Unstaged, err = parseNameStatusZ(unstagedRaw)
		if err != nil {
			return RepositorySnapshot{}, fmt.Errorf("parse unstaged changes: %w", err)
		}
		untrackedRaw, err := r.gitBytes(ctx, configuredRoot, "ls-files", "-z", "--others", "--exclude-standard")
		if err != nil {
			return RepositorySnapshot{}, fmt.Errorf("inspect untracked files: %w", err)
		}
		snapshot.Untracked = parseNULStrings(untrackedRaw)
		sort.Strings(snapshot.Untracked)

		stagedDiff, err := r.gitText(ctx, configuredRoot, "diff", "--cached", "--no-color", "--no-ext-diff", "--no-textconv", "--find-renames", "--")
		if err != nil {
			return RepositorySnapshot{}, fmt.Errorf("read staged diff: %w", err)
		}
		unstagedDiff, err := r.gitText(ctx, configuredRoot, "diff", "--no-color", "--no-ext-diff", "--no-textconv", "--find-renames", "--")
		if err != nil {
			return RepositorySnapshot{}, fmt.Errorf("read unstaged diff: %w", err)
		}
		snapshot.StagedDiff = stagedDiff
		snapshot.UnstagedDiff = unstagedDiff
	}

	snapshot.Clean = len(snapshot.Staged) == 0 && len(snapshot.Unstaged) == 0 && len(snapshot.Untracked) == 0

	historyRaw, err := r.gitBytes(ctx, configuredRoot, "log", "-z", "-n", "12", "--format=%H%x00%h%x00%aI%x00%an%x00%s")
	if err == nil {
		snapshot.History = parseHistoryNUL(historyRaw)
	} else if !snapshot.Unborn {
		return RepositorySnapshot{}, fmt.Errorf("read local history: %w", err)
	}

	if !bare {
		if err := r.loadFiles(ctx, configuredRoot, &snapshot); err != nil {
			return RepositorySnapshot{}, err
		}
	}

	snapshot.SnapshotSHA256 = repositorySnapshotHash(snapshot)
	return snapshot, nil
}

func (r *GitRepository) loadFiles(ctx context.Context, commandRoot string, snapshot *RepositorySnapshot) error {
	trackedRaw, err := r.gitBytes(ctx, commandRoot, "ls-files", "-z", "--cached")
	if err != nil {
		return fmt.Errorf("list tracked files: %w", err)
	}
	untrackedRaw, err := r.gitBytes(ctx, commandRoot, "ls-files", "-z", "--others", "--exclude-standard")
	if err != nil {
		return fmt.Errorf("list untracked files: %w", err)
	}
	tracked := make(map[string]bool)
	untracked := make(map[string]bool)
	paths := make(map[string]bool)
	for _, path := range parseNULStrings(trackedRaw) {
		tracked[path] = true
		paths[path] = true
	}
	for _, path := range parseNULStrings(untrackedRaw) {
		untracked[path] = true
		paths[path] = true
	}
	stagedStatus := changeStatusMap(snapshot.Staged)
	unstagedStatus := changeStatusMap(snapshot.Unstaged)
	previous := previousPathMap(snapshot.Staged, snapshot.Unstaged)

	ordered := make([]string, 0, len(paths))
	for path := range paths {
		ordered = append(ordered, path)
	}
	sort.Strings(ordered)

	for _, relPath := range ordered {
		if !safeRepositoryPath(relPath) {
			continue
		}
		fullPath := filepath.Join(snapshot.Root, filepath.FromSlash(relPath))
		rel := SourceFileRelationship{
			ID:               stableSourceID(snapshot.RepositoryID, relPath),
			Path:             relPath,
			PreviousPath:     previous[relPath],
			Tracked:          tracked[relPath],
			Untracked:        untracked[relPath],
			StagedStatus:     stagedStatus[relPath],
			UnstagedStatus:   unstagedStatus[relPath],
			ContentAuthority: "GIT_WORKTREE",
		}
		info, err := os.Lstat(fullPath)
		if errors.Is(err, os.ErrNotExist) {
			snapshot.Files = append(snapshot.Files, rel)
			continue
		}
		if err != nil {
			return fmt.Errorf("stat repository file %q: %w", relPath, err)
		}
		rel.Exists = true
		rel.Size = info.Size()
		rel.Symlink = info.Mode()&os.ModeSymlink != 0
		if rel.Symlink {
			rel.SymlinkTarget, _ = os.Readlink(fullPath)
			snapshot.Files = append(snapshot.Files, rel)
			continue
		}
		rel.Regular = info.Mode().IsRegular()
		if !rel.Regular {
			snapshot.Files = append(snapshot.Files, rel)
			continue
		}
		if rel.Size > r.maxDocumentBytes {
			rel.TooLarge = true
			snapshot.Files = append(snapshot.Files, rel)
			continue
		}
		content, err := os.ReadFile(fullPath)
		if err != nil {
			return fmt.Errorf("read repository file %q: %w", relPath, err)
		}
		rel.ContentSHA256 = contentSHA256(content)
		if !isTextContent(content) {
			rel.Binary = true
			snapshot.Files = append(snapshot.Files, rel)
			continue
		}
		if len(snapshot.Documents) >= r.maxDocuments {
			snapshot.DocumentLimitReached = true
			snapshot.Files = append(snapshot.Files, rel)
			continue
		}
		documentID := stableRepositoryDocumentID(snapshot.RepositoryID, relPath)
		rel.DocumentID = documentID
		snapshot.Documents = append(snapshot.Documents, RepositoryDocument{
			ID:           documentID,
			Path:         relPath,
			Kind:         documentKind(relPath),
			Title:        relPath,
			Content:      string(content),
			Size:         rel.Size,
			RepositoryID:  snapshot.RepositoryID,
			Source:        "git_worktree",
			ContentSHA256: rel.ContentSHA256,
		})
		snapshot.Files = append(snapshot.Files, rel)
	}
	return nil
}

func (r *GitRepository) gitBytes(ctx context.Context, root string, args ...string) ([]byte, error) {
	commandArgs := append([]string{"-C", root}, args...)
	cmd := exec.CommandContext(ctx, "git", commandArgs...)
	cmd.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0", "GIT_TERMINAL_PROMPT=0", "GIT_PAGER=cat", "PAGER=cat", "LC_ALL=C")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

func (r *GitRepository) gitText(ctx context.Context, root string, args ...string) (string, error) {
	out, err := r.gitBytes(ctx, root, args...)
	return string(out), err
}

func parseNameStatusZ(raw []byte) ([]GitChange, error) {
	parts := parseNULStrings(raw)
	changes := make([]GitChange, 0)
	for i := 0; i < len(parts); {
		status := parts[i]
		i++
		if status == "" {
			continue
		}
		if i >= len(parts) {
			return nil, fmt.Errorf("status %q missing path", status)
		}
		change := GitChange{Status: status, Kind: gitChangeKind(status)}
		if status[0] == 'R' || status[0] == 'C' {
			if i+1 >= len(parts) {
				return nil, fmt.Errorf("status %q missing rename/copy path", status)
			}
			change.PreviousPath = parts[i]
			change.Path = parts[i+1]
			i += 2
		} else {
			change.Path = parts[i]
			i++
		}
		changes = append(changes, change)
	}
	return changes, nil
}

func parseNULStrings(raw []byte) []string {
	parts := bytes.Split(raw, []byte{0})
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if len(part) > 0 {
			result = append(result, string(part))
		}
	}
	return result
}

func parseHistoryNUL(raw []byte) []GitCommit {
	fields := bytes.Split(raw, []byte{0})
	if len(fields) > 0 && len(fields[len(fields)-1]) == 0 {
		fields = fields[:len(fields)-1]
	}
	result := make([]GitCommit, 0, len(fields)/5)
	for len(fields) >= 5 {
		result = append(result, GitCommit{
			Commit: string(fields[0]), Short: string(fields[1]), Date: string(fields[2]), Author: string(fields[3]), Subject: string(fields[4]),
		})
		fields = fields[5:]
	}
	return result
}

func gitChangeKind(status string) string {
	if status == "" {
		return "unknown"
	}
	switch status[0] {
	case 'A':
		return "added"
	case 'M':
		return "modified"
	case 'D':
		return "deleted"
	case 'R':
		return "renamed"
	case 'C':
		return "copied"
	case 'T':
		return "type_changed"
	case 'U':
		return "unmerged"
	default:
		return "other"
	}
}

func changeStatusMap(changes []GitChange) map[string]string {
	result := make(map[string]string, len(changes))
	for _, change := range changes {
		result[change.Path] = change.Status
	}
	return result
}

func previousPathMap(groups ...[]GitChange) map[string]string {
	result := map[string]string{}
	for _, changes := range groups {
		for _, change := range changes {
			if change.PreviousPath != "" {
				result[change.Path] = change.PreviousPath
			}
		}
	}
	return result
}

func nonEmptyLines(value string) []string {
	var result []string
	for _, line := range strings.Split(value, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			result = append(result, line)
		}
	}
	return result
}

func digestStrings(parts ...string) string {
	h := sha256.New()
	for _, part := range parts {
		_, _ = h.Write([]byte(part))
		_, _ = h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func stableSourceID(repositoryID, path string) string {
	return "src-" + digestStrings("source-v1", repositoryID, path)[:20]
}

func stableRepositoryDocumentID(repositoryID, path string) string {
	return "git-" + digestStrings("document-v1", repositoryID, path)[:20]
}

func safeRepositoryPath(path string) bool {
	if path == "" || filepath.IsAbs(path) {
		return false
	}
	clean := filepath.Clean(filepath.FromSlash(path))
	return clean != "." && clean != ".." && !strings.HasPrefix(clean, ".."+string(filepath.Separator))
}

func isTextContent(content []byte) bool {
	if bytes.IndexByte(content, 0) >= 0 {
		return false
	}
	return utf8.Valid(content)
}

func documentKind(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".md", ".markdown":
		return "markdown"
	case ".yaml", ".yml":
		return "yaml"
	case ".json":
		return "json"
	case ".go":
		return "go"
	case ".js", ".cjs", ".mjs":
		return "javascript"
	case ".html", ".htm":
		return "html"
	case ".css":
		return "css"
	case ".sh", ".bash":
		return "shell"
	case ".txt", "":
		return "text"
	default:
		return "source"
	}
}

func repositorySnapshotHash(snapshot RepositorySnapshot) string {
	copy := snapshot
	copy.SnapshotSHA256 = ""
	raw, _ := json.Marshal(copy)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
