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
	"strconv"
	"strings"
	"unicode/utf8"
)

// maxCommitMessageBytes bounds the supplied commit message. Git itself applies
// no hard cap, so Phase 4 enforces an explicit, documented byte limit instead
// of accepting unbounded input.
const maxCommitMessageBytes = 4096

// Phase-4 local Git mutation never uses porcelain. These sentinels are the
// only authority the HTTP layer maps, and every rejection is typed rather than
// silently normalized.
var (
	ErrGitInvalidRequest       = errors.New("invalid local git control request")
	ErrGitRepositoryConflict   = errors.New("repository identity conflict")
	ErrGitHeadConflict         = errors.New("head commit conflict")
	ErrGitBranchConflict       = errors.New("branch conflict")
	ErrGitIndexConflict        = errors.New("index identity conflict")
	ErrGitWorktreeConflict     = errors.New("working tree content conflict")
	ErrGitRefUpdateConflict    = errors.New("branch ref update conflict")
	ErrGitCommitMessageInvalid = errors.New("commit message invalid")
	ErrGitUnsupportedTarget    = errors.New("unsupported local git control target")
	ErrGitUnsupportedState     = errors.New("unsupported local git state")
	ErrGitUnsafeAttributes     = errors.New("unsafe git attributes")
	ErrGitNothingToCommit      = errors.New("nothing staged to commit")
)

type GitCommitController interface {
	StageFile(context.Context, StageFileRequest) (StageFileResult, error)
	UnstageFile(context.Context, UnstageFileRequest) (UnstageFileResult, error)
	CommitStaged(context.Context, CommitStagedRequest) (CommitStagedResult, error)
}

type StageFileRequest struct {
	RepositoryID           string `json:"repository_id"`
	Path                   string `json:"path"`
	ExpectedHeadCommit     string `json:"expected_head_commit"`
	ExpectedBranch         string `json:"expected_branch"`
	ExpectedIndexSHA256    string `json:"expected_index_sha256"`
	ExpectedWorktreeSHA256 string `json:"expected_worktree_sha256"`
}

type StageFileResult struct {
	RepositoryID       string             `json:"repository_id"`
	Path               string             `json:"path"`
	Branch             string             `json:"branch"`
	ExpectedHeadCommit string             `json:"expected_head_commit"`
	HeadCommit         string             `json:"head_commit"`
	BeforeIndexSHA256  string             `json:"before_index_sha256"`
	AfterIndexSHA256   string             `json:"after_index_sha256"`
	WorktreeSHA256     string             `json:"worktree_sha256"`
	StagedObjectID     string             `json:"staged_object_id"`
	StagedMode         string             `json:"staged_mode"`
	Repository         RepositorySnapshot `json:"repository"`
}

type UnstageFileRequest struct {
	RepositoryID        string `json:"repository_id"`
	Path                string `json:"path"`
	ExpectedHeadCommit  string `json:"expected_head_commit"`
	ExpectedBranch      string `json:"expected_branch"`
	ExpectedIndexSHA256 string `json:"expected_index_sha256"`
}

type UnstageFileResult struct {
	RepositoryID       string             `json:"repository_id"`
	Path               string             `json:"path"`
	Branch             string             `json:"branch"`
	ExpectedHeadCommit string             `json:"expected_head_commit"`
	HeadCommit         string             `json:"head_commit"`
	BeforeIndexSHA256  string             `json:"before_index_sha256"`
	AfterIndexSHA256   string             `json:"after_index_sha256"`
	RestoredObjectID   string             `json:"restored_object_id"`
	RestoredMode       string             `json:"restored_mode"`
	Repository         RepositorySnapshot `json:"repository"`
}

type CommitStagedRequest struct {
	RepositoryID        string `json:"repository_id"`
	ExpectedHeadCommit  string `json:"expected_head_commit"`
	ExpectedBranch      string `json:"expected_branch"`
	ExpectedIndexSHA256 string `json:"expected_index_sha256"`
	CommitMessage       string `json:"commit_message"`
}

type CommitStagedResult struct {
	RepositoryID        string             `json:"repository_id"`
	Branch              string             `json:"branch"`
	BranchRef           string             `json:"branch_ref"`
	ExpectedHeadCommit  string             `json:"expected_head_commit"`
	HeadCommit          string             `json:"head_commit"`
	NewHeadCommit       string             `json:"new_head_commit"`
	ParentCommit        string             `json:"parent_commit"`
	TreeID              string             `json:"tree_id"`
	BeforeIndexSHA256   string             `json:"before_index_sha256"`
	AfterIndexSHA256    string             `json:"after_index_sha256"`
	CommitMessageSHA256 string             `json:"commit_message_sha256"`
	StagedPaths         []string           `json:"staged_paths"`
	Repository          RepositorySnapshot `json:"repository"`
}

// gitControlState is the observed repository identity a mutation is bound to.
type gitControlState struct {
	root        string
	gitDirs     []string
	headCommit  string
	branch      string
	branchRef   string
	indexSHA256 string
}

// inProgressMarkers are the Git operation markers that make a repository
// ineligible for Phase-4 mutation. The gate is deliberately conservative: any
// present marker blocks mutation instead of guessing the repository state.
var inProgressMarkers = []string{
	"MERGE_HEAD",
	"CHERRY_PICK_HEAD",
	"REVERT_HEAD",
	"BISECT_START",
	"BISECT_LOG",
	"rebase-merge",
	"rebase-apply",
	"sequencer",
}

// pinnedGitEnv replaces every inherited value that could make Git interactive
// or non-deterministic. GIT_OPTIONAL_LOCKS is pinned to 1 because Phase 4
// forbids GIT_OPTIONAL_LOCKS=0 for mutating plumbing.
var pinnedGitEnv = []string{
	"GIT_OPTIONAL_LOCKS=1",
	"GIT_TERMINAL_PROMPT=0",
	"GIT_PAGER=cat",
	"PAGER=cat",
	"LC_ALL=C",
	"GIT_EDITOR=:",
}

var pinnedGitEnvNames = []string{
	"GIT_OPTIONAL_LOCKS",
	"GIT_TERMINAL_PROMPT",
	"GIT_PAGER",
	"PAGER",
	"LC_ALL",
	"GIT_EDITOR",
	"EDITOR",
}

// mutateGitEnv returns the environment for mutating Git plumbing. Inherited
// interactive variables are dropped, not merely overridden, so no editor,
// pager, or credential prompt can be reached through the process environment.
func mutateGitEnv(environ []string) []string {
	env := make([]string, 0, len(environ)+len(pinnedGitEnv))
	for _, entry := range environ {
		name, _, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		pinned := false
		for _, candidate := range pinnedGitEnvNames {
			if name == candidate {
				pinned = true
				break
			}
		}
		if pinned {
			continue
		}
		env = append(env, entry)
	}
	return append(env, pinnedGitEnv...)
}

// gitMutate runs mutating Git plumbing. It uses an argument array only, never a
// shell, never opens an editor or prompt, never signs, and never resolves a
// repository hook. fsmonitor and signing are disabled explicitly because a
// repository configuration must not turn plumbing into an execution path.
func (r *GitRepository) gitMutate(ctx context.Context, root string, stdin []byte, args ...string) ([]byte, error) {
	commandArgs := []string{
		"-C", root,
		"-c", "core.fsmonitor=false",
		"-c", "core.hooksPath=/dev/null",
		"-c", "commit.gpgsign=false",
	}
	commandArgs = append(commandArgs, args...)
	cmd := exec.CommandContext(ctx, "git", commandArgs...)
	cmd.Env = mutateGitEnv(os.Environ())
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return out, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

func (r *GitRepository) gitMutateText(ctx context.Context, root string, stdin []byte, args ...string) (string, error) {
	out, err := r.gitMutate(ctx, root, stdin, args...)
	return string(out), err
}

func validObjectID(value string) bool {
	return (len(value) == 40 || len(value) == 64) && validLowerHex(value)
}

func validLowerHex(value string) bool {
	if value == "" {
		return false
	}
	for i := 0; i < len(value); i++ {
		c := value[i]
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') {
			continue
		}
		return false
	}
	return true
}

func validBranchName(name string) bool {
	if name == "" || len(name) > 240 {
		return false
	}
	if strings.HasPrefix(name, "-") || strings.HasPrefix(name, "/") || strings.HasSuffix(name, "/") {
		return false
	}
	if strings.HasSuffix(name, ".") || strings.HasSuffix(name, ".lock") {
		return false
	}
	if strings.Contains(name, "..") || strings.Contains(name, "//") || strings.Contains(name, "@{") {
		return false
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		if c <= ' ' || c >= 0x7f {
			return false
		}
		switch c {
		case '~', '^', ':', '?', '*', '[', '\\':
			return false
		}
	}
	return true
}

func validateCommitMessage(message string) error {
	if len(message) > maxCommitMessageBytes {
		return fmt.Errorf("%w: message is %d bytes, limit is %d", ErrGitCommitMessageInvalid, len(message), maxCommitMessageBytes)
	}
	if strings.ContainsRune(message, 0) {
		return fmt.Errorf("%w: message contains a NUL byte", ErrGitCommitMessageInvalid)
	}
	if !utf8.ValidString(message) {
		return fmt.Errorf("%w: message is not valid UTF-8", ErrGitCommitMessageInvalid)
	}
	if strings.TrimSpace(message) == "" {
		return fmt.Errorf("%w: message is empty after trimming whitespace", ErrGitCommitMessageInvalid)
	}
	return nil
}

func absoluteGitPath(root, value string) string {
	trimmed := strings.TrimSpace(value)
	if filepath.IsAbs(trimmed) {
		return filepath.Clean(trimmed)
	}
	return filepath.Join(root, filepath.FromSlash(trimmed))
}

// requireSupportedState enforces the Phase-4 supported repository state before
// any identity comparison, so an unsupported repository form is reported as
// unsupported instead of as a stale-identity conflict.
func (r *GitRepository) requireSupportedState(ctx context.Context, root string) (gitControlState, error) {
	var state gitControlState
	state.root = root

	head, headErr := r.gitText(ctx, root, "rev-parse", "--verify", "HEAD")
	if headErr != nil || strings.TrimSpace(head) == "" {
		return state, fmt.Errorf("%w: repository has no HEAD commit", ErrGitUnsupportedState)
	}
	state.headCommit = strings.TrimSpace(head)

	ref, refErr := r.gitText(ctx, root, "symbolic-ref", "--quiet", "HEAD")
	if refErr != nil {
		return state, fmt.Errorf("%w: HEAD is detached", ErrGitUnsupportedState)
	}
	state.branchRef = strings.TrimSpace(ref)
	if !strings.HasPrefix(state.branchRef, "refs/heads/") {
		return state, fmt.Errorf("%w: HEAD is attached to %q instead of a local branch", ErrGitUnsupportedState, state.branchRef)
	}
	state.branch = strings.TrimPrefix(state.branchRef, "refs/heads/")
	if !validBranchName(state.branch) {
		return state, fmt.Errorf("%w: attached branch %q is not a supported branch name", ErrGitUnsupportedState, state.branch)
	}

	indexSHA256, err := r.indexIdentity(ctx, root)
	if err != nil {
		return state, fmt.Errorf("resolve index identity: %w", err)
	}
	if indexSHA256 == "" {
		return state, fmt.Errorf("%w: index identity is unavailable for this repository form", ErrGitUnsupportedState)
	}
	state.indexSHA256 = indexSHA256

	for _, args := range [][]string{{"rev-parse", "--git-dir"}, {"rev-parse", "--git-common-dir"}} {
		out, err := r.gitText(ctx, root, args...)
		if err != nil {
			return state, fmt.Errorf("resolve git directory: %w", err)
		}
		state.gitDirs = append(state.gitDirs, absoluteGitPath(root, out))
	}
	if err := rejectInProgressOperations(state.gitDirs); err != nil {
		return state, err
	}
	return state, nil
}

func rejectInProgressOperations(gitDirs []string) error {
	seen := map[string]bool{}
	for _, gitDir := range gitDirs {
		if gitDir == "" || seen[gitDir] {
			continue
		}
		seen[gitDir] = true
		for _, marker := range inProgressMarkers {
			if _, err := os.Lstat(filepath.Join(gitDir, marker)); err == nil {
				return fmt.Errorf("%w: %s is present", ErrGitUnsupportedState, marker)
			}
		}
	}
	return nil
}

// rejectUnsafeAttributes refuses a path a custom Git filter or a
// working-tree-encoding attribute applies to. Both are hidden execution or
// hidden conversion paths for `git hash-object --path`.
func (r *GitRepository) rejectUnsafeAttributes(ctx context.Context, root, path string) error {
	out, err := r.gitBytes(ctx, root, "check-attr", "filter", "working-tree-encoding", "--", path)
	if err != nil {
		return fmt.Errorf("%w: cannot resolve git attributes for %s: %v", ErrGitUnsupportedState, path, err)
	}
	for _, line := range nonEmptyLines(string(out)) {
		// "<path>: <attribute>: <value>"; the value never contains ": " and
		// the path may, so the two trailing fields are split from the right.
		valueAt := strings.LastIndex(line, ": ")
		if valueAt < 0 {
			continue
		}
		attributeAt := strings.LastIndex(line[:valueAt], ": ")
		if attributeAt < 0 {
			continue
		}
		attribute := line[attributeAt+2 : valueAt]
		value := strings.TrimSpace(line[valueAt+2:])
		if value == "unspecified" || value == "unset" {
			continue
		}
		switch attribute {
		case "filter":
			return fmt.Errorf("%w: filter %q applies to %s", ErrGitUnsafeAttributes, value, path)
		case "working-tree-encoding":
			return fmt.Errorf("%w: working-tree-encoding %q applies to %s", ErrGitUnsafeAttributes, value, path)
		}
	}
	return nil
}

type gitTreeEntry struct {
	mode     string
	objectID string
}

func (r *GitRepository) headTreeEntry(ctx context.Context, root, path string) (gitTreeEntry, error) {
	out, err := r.gitBytes(ctx, root, "ls-tree", "HEAD", "--", path)
	if err != nil {
		return gitTreeEntry{}, fmt.Errorf("read HEAD tree entry for %q: %w", path, err)
	}
	lines := nonEmptyLines(string(out))
	if len(lines) != 1 {
		return gitTreeEntry{}, fmt.Errorf("%w: %s is not a single tracked entry in HEAD", ErrGitUnsupportedTarget, path)
	}
	fields := strings.Fields(lines[0])
	tab := strings.Index(lines[0], "\t")
	if len(fields) < 4 || tab < 0 || lines[0][tab+1:] != path {
		return gitTreeEntry{}, fmt.Errorf("%w: %s does not resolve to one HEAD blob", ErrGitUnsupportedTarget, path)
	}
	if fields[1] != "blob" {
		return gitTreeEntry{}, fmt.Errorf("%w: %s is not a regular blob in HEAD", ErrGitUnsupportedTarget, path)
	}
	if !validObjectID(fields[2]) {
		return gitTreeEntry{}, fmt.Errorf("unexpected HEAD tree entry for %s: %q", path, lines[0])
	}
	return gitTreeEntry{mode: fields[0], objectID: fields[2]}, nil
}

type gitIndexEntry struct {
	mode     string
	objectID string
	stage    int
}

func (r *GitRepository) indexEntry(ctx context.Context, root, path string) (gitIndexEntry, error) {
	out, err := r.gitBytes(ctx, root, "ls-files", "-s", "--", path)
	if err != nil {
		return gitIndexEntry{}, fmt.Errorf("read index entry for %q: %w", path, err)
	}
	lines := nonEmptyLines(string(out))
	if len(lines) != 1 {
		return gitIndexEntry{}, fmt.Errorf("%w: %s is not a single stage-0 index entry", ErrGitUnsupportedTarget, path)
	}
	tab := strings.Index(lines[0], "\t")
	fields := strings.Fields(lines[0][:tab])
	if len(fields) != 3 || tab < 0 {
		return gitIndexEntry{}, fmt.Errorf("unexpected index entry for %s: %q", path, lines[0])
	}
	stage, err := strconv.Atoi(fields[2])
	if err != nil {
		return gitIndexEntry{}, fmt.Errorf("unexpected index stage for %s: %q", path, lines[0])
	}
	if stage != 0 {
		return gitIndexEntry{}, fmt.Errorf("%w: %s has unmerged index stage %d", ErrGitUnsupportedTarget, path, stage)
	}
	if !validObjectID(fields[1]) {
		return gitIndexEntry{}, fmt.Errorf("unexpected index object for %s: %q", path, lines[0])
	}
	return gitIndexEntry{mode: fields[0], objectID: fields[1], stage: stage}, nil
}

// resolvePhase4Target walks the target path without following a symlink and
// requires a regular file at the end, so a blob is never derived from a path
// that escapes the repository root.
func resolvePhase4Target(root, relPath string) (string, os.FileInfo, error) {
	fullPath := filepath.Join(root, filepath.FromSlash(relPath))
	relative, err := filepath.Rel(root, fullPath)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", nil, fmt.Errorf("%w: path escapes the repository root", ErrGitInvalidRequest)
	}
	parts := strings.Split(filepath.FromSlash(relPath), string(filepath.Separator))
	current := root
	for i, part := range parts {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if errors.Is(err, os.ErrNotExist) {
			return "", nil, fmt.Errorf("%w: target does not exist at %s", ErrGitUnsupportedTarget, relPath)
		}
		if err != nil {
			return "", nil, fmt.Errorf("stat target %q: %w", relPath, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", nil, fmt.Errorf("%w: target traverses a symlink at %s", ErrGitUnsupportedTarget, relPath)
		}
		if i == len(parts)-1 {
			if !info.Mode().IsRegular() {
				return "", nil, fmt.Errorf("%w: target is not a regular file: %s", ErrGitUnsupportedTarget, relPath)
			}
			return fullPath, info, nil
		}
		if !info.IsDir() {
			return "", nil, fmt.Errorf("%w: target path component is not a directory: %s", ErrGitUnsupportedTarget, relPath)
		}
	}
	return "", nil, fmt.Errorf("%w: empty target path", ErrGitInvalidRequest)
}

// worktreeIndexMode is the index mode a worktree file would check in as. Phase 4
// never stages an executable-bit or type change, so a mismatch between the
// worktree and the current index entry is rejected.
func worktreeIndexMode(info os.FileInfo) string {
	if info.Mode()&0o111 != 0 {
		return "100755"
	}
	return "100644"
}

func changeForPath(changes []GitChange, path string) (GitChange, bool) {
	for _, change := range changes {
		if change.Path == path {
			return change, true
		}
	}
	return GitChange{}, false
}

func (r *GitRepository) StageFile(ctx context.Context, request StageFileRequest) (StageFileResult, error) {
	var result StageFileResult

	path, err := canonicalWritePath(request.Path)
	if err != nil {
		return result, fmt.Errorf("%w: repository path must be canonical: %q", ErrGitInvalidRequest, request.Path)
	}
	if request.RepositoryID == "" || !validObjectID(request.ExpectedHeadCommit) ||
		!validBranchName(request.ExpectedBranch) || !validSHA256(request.ExpectedIndexSHA256) ||
		!validSHA256(request.ExpectedWorktreeSHA256) {
		return result, fmt.Errorf("%w: repository_id, path, expected_head_commit, expected_branch, expected_index_sha256, and expected_worktree_sha256 are required", ErrGitInvalidRequest)
	}

	root, err := r.commandRoot()
	if err != nil {
		return result, err
	}
	state, err := r.requireSupportedState(ctx, root)
	if err != nil {
		return result, err
	}

	before, err := r.Inspect(ctx)
	if err != nil {
		return result, err
	}
	result = StageFileResult{
		RepositoryID:       before.RepositoryID,
		Path:               path,
		Branch:             before.Branch,
		ExpectedHeadCommit: before.HeadCommit,
		HeadCommit:         before.HeadCommit,
		BeforeIndexSHA256:  before.IndexSHA256,
	}
	if err := compareBoundIdentity(before, request.RepositoryID, request.ExpectedHeadCommit, request.ExpectedBranch, request.ExpectedIndexSHA256); err != nil {
		return result, err
	}

	// Attributes are rejected before the change-shape gate: a custom filter is
	// a hidden execution path regardless of the shape of the pending change.
	if err := r.rejectUnsafeAttributes(ctx, root, path); err != nil {
		return result, err
	}

	entry, err := r.indexEntry(ctx, root, path)
	if err != nil {
		return result, err
	}
	if _, err := r.headTreeEntry(ctx, root, path); err != nil {
		return result, err
	}
	if _, staged := changeForPath(before.Staged, path); staged {
		return result, fmt.Errorf("%w: %s already has a staged entry", ErrGitUnsupportedTarget, path)
	}
	unstaged, ok := changeForPath(before.Unstaged, path)
	if !ok || unstaged.Kind != "modified" || unstaged.Status == "" || unstaged.Status[0] != 'M' {
		return result, fmt.Errorf("%w: %s is not an unstaged tracked modification", ErrGitUnsupportedTarget, path)
	}
	if containsString(before.Untracked, path) {
		return result, fmt.Errorf("%w: %s is untracked", ErrGitUnsupportedTarget, path)
	}

	fullPath, info, err := resolvePhase4Target(before.Root, path)
	if err != nil {
		return result, err
	}
	if entry.mode != worktreeIndexMode(info) {
		return result, fmt.Errorf("%w: %s has a mode or type change that Phase 4 does not stage", ErrGitUnsupportedTarget, path)
	}

	current, err := os.ReadFile(fullPath)
	if err != nil {
		return result, fmt.Errorf("read stage target %q: %w", path, err)
	}
	observedSHA := contentSHA256(current)
	result.WorktreeSHA256 = observedSHA
	if observedSHA != request.ExpectedWorktreeSHA256 {
		return result, fmt.Errorf("%w: expected %s got %s", ErrGitWorktreeConflict, request.ExpectedWorktreeSHA256, observedSHA)
	}

	// Re-check every bound identity from disk immediately before mutation.
	if err := r.rebindState(ctx, root, state, request.ExpectedHeadCommit, request.ExpectedBranch, request.ExpectedIndexSHA256); err != nil {
		return result, err
	}
	latestInfo, err := os.Lstat(fullPath)
	if errors.Is(err, os.ErrNotExist) {
		return result, fmt.Errorf("%w: %s disappeared before staging", ErrGitWorktreeConflict, path)
	}
	if err != nil {
		return result, fmt.Errorf("restat stage target %q: %w", path, err)
	}
	if !latestInfo.Mode().IsRegular() {
		return result, fmt.Errorf("%w: %s changed type before staging", ErrGitUnsupportedTarget, path)
	}
	latestEntry, err := r.indexEntry(ctx, root, path)
	if err != nil {
		return result, err
	}
	if latestEntry.mode != entry.mode || latestEntry.objectID != entry.objectID {
		return result, fmt.Errorf("%w: index entry for %s changed before staging", ErrGitIndexConflict, path)
	}
	latest, err := os.ReadFile(fullPath)
	if err != nil {
		return result, fmt.Errorf("reread stage target %q: %w", path, err)
	}
	if contentSHA256(latest) != observedSHA {
		return result, fmt.Errorf("%w: %s changed before staging", ErrGitWorktreeConflict, path)
	}

	// hash-object --path keeps built-in text/EOL conversion consistent with Git
	// check-in. Custom filters and encodings were already rejected.
	blob, err := r.gitMutateText(ctx, root, nil, "hash-object", "-w", "--path="+path, fullPath)
	if err != nil {
		return result, fmt.Errorf("create candidate blob for %q: %w", path, err)
	}
	blobID := strings.TrimSpace(blob)
	if !validObjectID(blobID) {
		return result, fmt.Errorf("unexpected blob id %q for %s", blobID, path)
	}
	if _, err := r.gitMutate(ctx, root, nil, "update-index", "--cacheinfo", entry.mode+","+blobID+","+path); err != nil {
		return result, fmt.Errorf("update index entry for %q: %w", path, err)
	}

	after, err := r.Inspect(ctx)
	if err != nil {
		return result, fmt.Errorf("fresh repository inspection after stage: %w", err)
	}
	if after.RepositoryID != request.RepositoryID {
		return result, fmt.Errorf("%w: repository identity changed after staging", ErrGitRepositoryConflict)
	}
	if after.IndexSHA256 == result.BeforeIndexSHA256 {
		return result, fmt.Errorf("index identity did not change after staging %s", path)
	}
	result.AfterIndexSHA256 = after.IndexSHA256
	result.StagedObjectID = blobID
	result.StagedMode = entry.mode
	result.Repository = after
	return result, nil
}

func (r *GitRepository) UnstageFile(ctx context.Context, request UnstageFileRequest) (UnstageFileResult, error) {
	var result UnstageFileResult

	path, err := canonicalWritePath(request.Path)
	if err != nil {
		return result, fmt.Errorf("%w: repository path must be canonical: %q", ErrGitInvalidRequest, request.Path)
	}
	if request.RepositoryID == "" || !validObjectID(request.ExpectedHeadCommit) ||
		!validBranchName(request.ExpectedBranch) || !validSHA256(request.ExpectedIndexSHA256) {
		return result, fmt.Errorf("%w: repository_id, path, expected_head_commit, expected_branch, and expected_index_sha256 are required", ErrGitInvalidRequest)
	}

	root, err := r.commandRoot()
	if err != nil {
		return result, err
	}
	state, err := r.requireSupportedState(ctx, root)
	if err != nil {
		return result, err
	}

	before, err := r.Inspect(ctx)
	if err != nil {
		return result, err
	}
	result = UnstageFileResult{
		RepositoryID:       before.RepositoryID,
		Path:               path,
		Branch:             before.Branch,
		ExpectedHeadCommit: before.HeadCommit,
		HeadCommit:         before.HeadCommit,
		BeforeIndexSHA256:  before.IndexSHA256,
	}
	if err := compareBoundIdentity(before, request.RepositoryID, request.ExpectedHeadCommit, request.ExpectedBranch, request.ExpectedIndexSHA256); err != nil {
		return result, err
	}

	if err := r.rejectUnsafeAttributes(ctx, root, path); err != nil {
		return result, err
	}

	staged, ok := changeForPath(before.Staged, path)
	if !ok || staged.Kind != "modified" || staged.Status == "" || staged.Status[0] != 'M' {
		return result, fmt.Errorf("%w: %s is not a staged tracked modification", ErrGitUnsupportedTarget, path)
	}
	if _, err := r.indexEntry(ctx, root, path); err != nil {
		return result, err
	}
	if _, err := r.headTreeEntry(ctx, root, path); err != nil {
		return result, err
	}

	if err := r.rebindState(ctx, root, state, request.ExpectedHeadCommit, request.ExpectedBranch, request.ExpectedIndexSHA256); err != nil {
		return result, err
	}
	head, err := r.headTreeEntry(ctx, root, path)
	if err != nil {
		return result, err
	}

	if _, err := r.gitMutate(ctx, root, nil, "update-index", "--cacheinfo", head.mode+","+head.objectID+","+path); err != nil {
		return result, fmt.Errorf("restore index entry for %q: %w", path, err)
	}

	after, err := r.Inspect(ctx)
	if err != nil {
		return result, fmt.Errorf("fresh repository inspection after unstage: %w", err)
	}
	if after.RepositoryID != request.RepositoryID {
		return result, fmt.Errorf("%w: repository identity changed after unstaging", ErrGitRepositoryConflict)
	}
	if after.IndexSHA256 == result.BeforeIndexSHA256 {
		return result, fmt.Errorf("index identity did not change after unstaging %s", path)
	}
	result.AfterIndexSHA256 = after.IndexSHA256
	result.RestoredObjectID = head.objectID
	result.RestoredMode = head.mode
	result.Repository = after
	return result, nil
}

func (r *GitRepository) CommitStaged(ctx context.Context, request CommitStagedRequest) (CommitStagedResult, error) {
	var result CommitStagedResult

	if request.RepositoryID == "" || !validObjectID(request.ExpectedHeadCommit) ||
		!validBranchName(request.ExpectedBranch) || !validSHA256(request.ExpectedIndexSHA256) {
		return result, fmt.Errorf("%w: repository_id, expected_head_commit, expected_branch, expected_index_sha256, and commit_message are required", ErrGitInvalidRequest)
	}
	if err := validateCommitMessage(request.CommitMessage); err != nil {
		return result, err
	}

	root, err := r.commandRoot()
	if err != nil {
		return result, err
	}
	state, err := r.requireSupportedState(ctx, root)
	if err != nil {
		return result, err
	}

	before, err := r.Inspect(ctx)
	if err != nil {
		return result, err
	}
	result = CommitStagedResult{
		RepositoryID:        before.RepositoryID,
		Branch:              before.Branch,
		BranchRef:           state.branchRef,
		ExpectedHeadCommit:  before.HeadCommit,
		HeadCommit:          before.HeadCommit,
		BeforeIndexSHA256:   before.IndexSHA256,
		CommitMessageSHA256: contentSHA256([]byte(request.CommitMessage)),
	}
	if err := compareBoundIdentity(before, request.RepositoryID, request.ExpectedHeadCommit, request.ExpectedBranch, request.ExpectedIndexSHA256); err != nil {
		return result, err
	}

	if len(before.Staged) == 0 {
		return result, fmt.Errorf("%w: the index has no staged entries", ErrGitNothingToCommit)
	}
	paths := make([]string, 0, len(before.Staged))
	for _, change := range before.Staged {
		if change.Kind != "modified" || change.Status == "" || change.Status[0] != 'M' || change.PreviousPath != "" {
			return result, fmt.Errorf("%w: staged entry %q is a %s change, Phase 4 commits staged modifications only", ErrGitUnsupportedTarget, change.Path, change.Kind)
		}
		paths = append(paths, change.Path)
	}
	sort.Strings(paths)
	result.StagedPaths = paths

	// Every staged path is re-verified: a custom filter must not become a
	// hidden execution path at commit time either.
	for _, path := range paths {
		if err := r.rejectUnsafeAttributes(ctx, root, path); err != nil {
			return result, err
		}
	}

	if err := r.rebindState(ctx, root, state, request.ExpectedHeadCommit, request.ExpectedBranch, request.ExpectedIndexSHA256); err != nil {
		return result, err
	}
	latest, err := r.Inspect(ctx)
	if err != nil {
		return result, err
	}
	if len(latest.Staged) != len(paths) {
		return result, fmt.Errorf("%w: the staged set changed before commit", ErrGitIndexConflict)
	}
	for _, path := range paths {
		change, ok := changeForPath(latest.Staged, path)
		if !ok || change.Kind != "modified" {
			return result, fmt.Errorf("%w: staged entry %q changed before commit", ErrGitIndexConflict, path)
		}
	}

	tree, err := r.gitMutateText(ctx, root, nil, "write-tree")
	if err != nil {
		return result, fmt.Errorf("write staged tree: %w", err)
	}
	treeID := strings.TrimSpace(tree)
	if !validObjectID(treeID) {
		return result, fmt.Errorf("unexpected tree id %q", treeID)
	}

	commit, err := r.gitMutateText(ctx, root, []byte(request.CommitMessage), "commit-tree", treeID, "-p", request.ExpectedHeadCommit)
	if err != nil {
		return result, fmt.Errorf("create commit object: %w", err)
	}
	commitID := strings.TrimSpace(commit)
	if !validObjectID(commitID) {
		return result, fmt.Errorf("unexpected commit id %q", commitID)
	}

	// Compare-and-swap on the observed branch ref only. The ref is never forced.
	if _, err := r.gitMutate(ctx, root, nil, "update-ref", state.branchRef, commitID, request.ExpectedHeadCommit); err != nil {
		return result, fmt.Errorf("%w: could not advance %s to %s from %s: %v", ErrGitRefUpdateConflict, state.branchRef, commitID, request.ExpectedHeadCommit, err)
	}

	after, err := r.Inspect(ctx)
	if err != nil {
		return result, fmt.Errorf("fresh repository inspection after commit: %w", err)
	}
	if after.RepositoryID != request.RepositoryID {
		return result, fmt.Errorf("%w: repository identity changed after commit", ErrGitRepositoryConflict)
	}
	if after.HeadCommit != commitID {
		return result, fmt.Errorf("%w: observed HEAD %s after advancing %s to %s", ErrGitRefUpdateConflict, after.HeadCommit, state.branchRef, commitID)
	}
	result.HeadCommit = after.HeadCommit
	result.NewHeadCommit = after.HeadCommit
	result.ParentCommit = request.ExpectedHeadCommit
	result.TreeID = treeID
	result.AfterIndexSHA256 = after.IndexSHA256
	result.Repository = after
	return result, nil
}

// rebindState re-derives every bound identity from disk and rejects any drift
// between the inspected snapshot and the moment of mutation.
func (r *GitRepository) rebindState(ctx context.Context, root string, state gitControlState, expectedHead, expectedBranch, expectedIndex string) error {
	latest, err := r.requireSupportedState(ctx, root)
	if err != nil {
		return err
	}
	if latest.headCommit != expectedHead || latest.headCommit != state.headCommit {
		return fmt.Errorf("%w: expected %s got %s", ErrGitHeadConflict, expectedHead, latest.headCommit)
	}
	if latest.branch != expectedBranch || latest.branchRef != state.branchRef {
		return fmt.Errorf("%w: expected %s got %s", ErrGitBranchConflict, expectedBranch, latest.branch)
	}
	if latest.indexSHA256 != expectedIndex || latest.indexSHA256 != state.indexSHA256 {
		return fmt.Errorf("%w: expected %s got %s", ErrGitIndexConflict, expectedIndex, latest.indexSHA256)
	}
	return nil
}

func compareBoundIdentity(before RepositorySnapshot, repositoryID, expectedHead, expectedBranch, expectedIndex string) error {
	if before.RepositoryID != repositoryID {
		return fmt.Errorf("%w: expected %s got %s", ErrGitRepositoryConflict, repositoryID, before.RepositoryID)
	}
	if before.HeadCommit != expectedHead {
		return fmt.Errorf("%w: expected %s got %s", ErrGitHeadConflict, expectedHead, before.HeadCommit)
	}
	if before.Branch != expectedBranch {
		return fmt.Errorf("%w: expected %s got %s", ErrGitBranchConflict, expectedBranch, before.Branch)
	}
	if before.IndexSHA256 != expectedIndex {
		return fmt.Errorf("%w: expected %s got %s", ErrGitIndexConflict, expectedIndex, before.IndexSHA256)
	}
	return nil
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
