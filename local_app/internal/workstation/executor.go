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
	"strconv"
	"strings"
	"time"
)

const (
	defaultExecutionTimeoutSeconds = 1800
	maxExecutionTimeoutSeconds     = 7200
	defaultTranscriptLimitBytes    = 8 * 1024 * 1024
)

var (
	ErrExecutionInvalidRequest    = errors.New("invalid execution request")
	ErrExecutionRepositoryConflict = errors.New("execution repository conflict")
	ErrExecutionRepositoryDirty   = errors.New("execution repository is dirty")
	ErrExecutionUnavailable       = errors.New("opencode unavailable")
	ErrExecutionContract          = errors.New("opencode cli contract mismatch")
	ErrExecutionEvidence          = errors.New("execution evidence failure")
	ErrExecutionTransport         = errors.New("execution transport failure")
)

type ExecutionRequest struct {
	RepositoryID      string `json:"repository_id"`
	ExpectedHeadCommit string `json:"expected_head_commit"`
	Prompt            string `json:"prompt"`
	Model             string `json:"model"`
	Variant           string `json:"variant,omitempty"`
	AutoApprove       bool   `json:"auto_approve"`
	TimeoutSeconds    int    `json:"timeout_seconds,omitempty"`
}

type ExecutionResult struct {
	RunID                    string             `json:"run_id"`
	Status                   string             `json:"status"`
	RepositoryID             string             `json:"repository_id"`
	StartingHeadCommit       string             `json:"starting_head_commit"`
	PromptSHA256             string             `json:"prompt_sha256"`
	PromptBytes              int                `json:"prompt_bytes"`
	Model                    string             `json:"model"`
	Variant                  string             `json:"variant,omitempty"`
	AutoApprove              bool               `json:"auto_approve"`
	TimeoutSeconds           int                `json:"timeout_seconds"`
	StartedAt                string             `json:"started_at"`
	CompletedAt              string             `json:"completed_at"`
	ExitCode                 int                `json:"exit_code"`
	OpenCodeVersion          string             `json:"opencode_version"`
	OpenCodeRunHelpSHA256    string             `json:"opencode_run_help_sha256"`
	TranscriptTruncated      bool               `json:"transcript_truncated"`
	EvidenceDir              string             `json:"evidence_dir"`
	FinalRepository          RepositorySnapshot `json:"repository"`
}

type ExecutionCommandResult struct {
	Output    []byte
	ExitCode  int
	Truncated bool
}

type ExecutionCommandRunner interface {
	Run(context.Context, string, []string, string, []string, int) (ExecutionCommandResult, error)
}

type OSExecutionCommandRunner struct{}

type limitedCapture struct {
	limit     int
	data      bytes.Buffer
	truncated bool
}

func (w *limitedCapture) Write(p []byte) (int, error) {
	if w.limit <= 0 {
		w.truncated = w.truncated || len(p) > 0
		return len(p), nil
	}
	remaining := w.limit - w.data.Len()
	if remaining > 0 {
		if len(p) <= remaining {
			_, _ = w.data.Write(p)
		} else {
			_, _ = w.data.Write(p[:remaining])
			w.truncated = true
		}
	} else if len(p) > 0 {
		w.truncated = true
	}
	return len(p), nil
}

func (OSExecutionCommandRunner) Run(ctx context.Context, name string, args []string, dir string, env []string, outputLimit int) (ExecutionCommandResult, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = env
	capture := &limitedCapture{limit: outputLimit}
	cmd.Stdout = capture
	cmd.Stderr = capture
	err := cmd.Run()
	result := ExecutionCommandResult{
		Output:    append([]byte(nil), capture.data.Bytes()...),
		ExitCode:  0,
		Truncated: capture.truncated,
	}
	if err == nil {
		return result, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
		return result, nil
	}
	return ExecutionCommandResult{}, err
}

type OpenCodeExecutor struct {
	Repository      RepositoryProvider
	Binary          string
	EvidenceRoot    string
	Runner          ExecutionCommandRunner
	Now             func() time.Time
	TranscriptLimit int
}

func NewOpenCodeExecutor(repository RepositoryProvider, evidenceRoot string) *OpenCodeExecutor {
	return &OpenCodeExecutor{
		Repository:      repository,
		Binary:          "opencode",
		EvidenceRoot:    evidenceRoot,
		Runner:          OSExecutionCommandRunner{},
		Now:             time.Now,
		TranscriptLimit: defaultTranscriptLimitBytes,
	}
}

func (e *OpenCodeExecutor) Execute(ctx context.Context, request ExecutionRequest) (ExecutionResult, error) {
	if e.Repository == nil {
		return ExecutionResult{}, fmt.Errorf("%w: repository inspection is not configured", ErrExecutionInvalidRequest)
	}
	if e.Runner == nil {
		e.Runner = OSExecutionCommandRunner{}
	}
	if e.Now == nil {
		e.Now = time.Now
	}
	if e.Binary == "" {
		e.Binary = "opencode"
	}
	if e.TranscriptLimit <= 0 {
		e.TranscriptLimit = defaultTranscriptLimitBytes
	}

	request.RepositoryID = strings.TrimSpace(request.RepositoryID)
	request.ExpectedHeadCommit = strings.TrimSpace(request.ExpectedHeadCommit)
	request.Model = strings.TrimSpace(request.Model)
	request.Variant = strings.TrimSpace(request.Variant)
	if request.RepositoryID == "" || request.ExpectedHeadCommit == "" || request.Model == "" || request.Prompt == "" {
		return ExecutionResult{}, fmt.Errorf("%w: repository_id, expected_head_commit, model, and prompt are required", ErrExecutionInvalidRequest)
	}
	if len(request.Prompt) > 1024*1024 {
		return ExecutionResult{}, fmt.Errorf("%w: prompt exceeds 1 MiB", ErrExecutionInvalidRequest)
	}
	if strings.ContainsRune(request.Model, '\x00') || strings.ContainsRune(request.Variant, '\x00') || strings.ContainsRune(request.Prompt, '\x00') {
		return ExecutionResult{}, fmt.Errorf("%w: NUL bytes are not allowed", ErrExecutionInvalidRequest)
	}
	timeoutSeconds := request.TimeoutSeconds
	if timeoutSeconds == 0 {
		timeoutSeconds = defaultExecutionTimeoutSeconds
	}
	if timeoutSeconds < 1 || timeoutSeconds > maxExecutionTimeoutSeconds {
		return ExecutionResult{}, fmt.Errorf("%w: timeout_seconds must be between 1 and %d", ErrExecutionInvalidRequest, maxExecutionTimeoutSeconds)
	}

	before, err := e.Repository.Inspect(ctx)
	if err != nil {
		return ExecutionResult{}, fmt.Errorf("%w: inspect repository: %v", ErrExecutionRepositoryConflict, err)
	}
	if before.RepositoryID != request.RepositoryID {
		return ExecutionResult{}, fmt.Errorf("%w: repository identity changed", ErrExecutionRepositoryConflict)
	}
	if before.Unborn || before.HeadCommit == "" || before.HeadCommit != request.ExpectedHeadCommit {
		return ExecutionResult{}, fmt.Errorf("%w: HEAD changed from requested baseline", ErrExecutionRepositoryConflict)
	}
	if !before.Clean {
		return ExecutionResult{}, fmt.Errorf("%w: staged=%d unstaged=%d untracked=%d", ErrExecutionRepositoryDirty, len(before.Staged), len(before.Unstaged), len(before.Untracked))
	}

	evidenceRoot, err := e.resolveEvidenceRoot(before.Root)
	if err != nil {
		return ExecutionResult{}, err
	}
	promptSum := sha256.Sum256([]byte(request.Prompt))
	promptSHA := hex.EncodeToString(promptSum[:])
	started := e.Now().UTC()
	runID := started.Format("20060102T150405.000000000Z") + "-" + promptSHA[:12]
	evidenceDir := filepath.Join(evidenceRoot, runID)
	if err := os.MkdirAll(evidenceDir, 0o700); err != nil {
		return ExecutionResult{}, fmt.Errorf("%w: create evidence directory: %v", ErrExecutionEvidence, err)
	}
	if err := os.Chmod(evidenceDir, 0o700); err != nil {
		return ExecutionResult{}, fmt.Errorf("%w: chmod evidence directory: %v", ErrExecutionEvidence, err)
	}
	if err := writePrivateFile(filepath.Join(evidenceDir, "prompt.txt"), []byte(request.Prompt)); err != nil {
		return ExecutionResult{}, err
	}

	env := append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_PAGER=cat",
		"PAGER=cat",
		"LC_ALL=C",
	)
	versionResult, err := e.Runner.Run(ctx, e.Binary, []string{"--version"}, before.Root, env, 256*1024)
	if err != nil {
		return ExecutionResult{}, fmt.Errorf("%w: opencode --version: %v", ErrExecutionUnavailable, err)
	}
	if versionResult.ExitCode != 0 {
		return ExecutionResult{}, fmt.Errorf("%w: opencode --version exit=%d", ErrExecutionUnavailable, versionResult.ExitCode)
	}
	helpResult, err := e.Runner.Run(ctx, e.Binary, []string{"run", "--help"}, before.Root, env, 2*1024*1024)
	if err != nil {
		return ExecutionResult{}, fmt.Errorf("%w: opencode run --help: %v", ErrExecutionUnavailable, err)
	}
	if helpResult.ExitCode != 0 {
		return ExecutionResult{}, fmt.Errorf("%w: opencode run --help exit=%d", ErrExecutionUnavailable, helpResult.ExitCode)
	}
	helpText := string(helpResult.Output)
	requiredFlags := []string{"--model", "--dir"}
	if request.Variant != "" {
		requiredFlags = append(requiredFlags, "--variant")
	}
	if request.AutoApprove {
		requiredFlags = append(requiredFlags, "--auto")
	}
	for _, flag := range requiredFlags {
		if !strings.Contains(helpText, flag) {
			return ExecutionResult{}, fmt.Errorf("%w: installed opencode run --help does not advertise %s", ErrExecutionContract, flag)
		}
	}
	if err := writePrivateFile(filepath.Join(evidenceDir, "opencode-version.txt"), versionResult.Output); err != nil {
		return ExecutionResult{}, err
	}
	if err := writePrivateFile(filepath.Join(evidenceDir, "opencode-run-help.txt"), helpResult.Output); err != nil {
		return ExecutionResult{}, err
	}

	args := []string{"run", "--model", request.Model, "--dir", before.Root}
	if request.Variant != "" {
		args = append(args, "--variant", request.Variant)
	}
	if request.AutoApprove {
		args = append(args, "--auto")
	}
	args = append(args, request.Prompt)

	runCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSeconds)*time.Second)
	defer cancel()
	runResult, transportErr := e.Runner.Run(runCtx, e.Binary, args, before.Root, env, e.TranscriptLimit)
	if transportErr != nil {
		return ExecutionResult{}, fmt.Errorf("%w: %v", ErrExecutionTransport, transportErr)
	}
	if err := writePrivateFile(filepath.Join(evidenceDir, "transcript.log"), runResult.Output); err != nil {
		return ExecutionResult{}, err
	}

	after, inspectErr := e.Repository.Inspect(context.Background())
	if inspectErr != nil {
		return ExecutionResult{}, fmt.Errorf("%w: inspect repository after execution: %v", ErrExecutionEvidence, inspectErr)
	}
	completed := e.Now().UTC()
	helpSum := sha256.Sum256(helpResult.Output)
	status := "PASS"
	if runResult.ExitCode != 0 {
		status = "FAIL"
	}
	result := ExecutionResult{
		RunID:                 runID,
		Status:                status,
		RepositoryID:          before.RepositoryID,
		StartingHeadCommit:    before.HeadCommit,
		PromptSHA256:          promptSHA,
		PromptBytes:           len([]byte(request.Prompt)),
		Model:                 request.Model,
		Variant:               request.Variant,
		AutoApprove:           request.AutoApprove,
		TimeoutSeconds:        timeoutSeconds,
		StartedAt:             started.Format(time.RFC3339Nano),
		CompletedAt:           completed.Format(time.RFC3339Nano),
		ExitCode:              runResult.ExitCode,
		OpenCodeVersion:       strings.TrimSpace(string(versionResult.Output)),
		OpenCodeRunHelpSHA256: contentSHA256(helpResult.Output),
		TranscriptTruncated:   runResult.Truncated,
		EvidenceDir:           evidenceDir,
		FinalRepository:       after,
	}
	raw, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return ExecutionResult{}, fmt.Errorf("%w: marshal execution result: %v", ErrExecutionEvidence, err)
	}
	raw = append(raw, '\n')
	if err := writePrivateFile(filepath.Join(evidenceDir, "execution.json"), raw); err != nil {
		return ExecutionResult{}, err
	}
	return result, nil
}

func (e *OpenCodeExecutor) resolveEvidenceRoot(repositoryRoot string) (string, error) {
	root := strings.TrimSpace(e.EvidenceRoot)
	if root == "" {
		if stateHome := os.Getenv("XDG_STATE_HOME"); stateHome != "" {
			root = filepath.Join(stateHome, "llm-hub", "executions")
		} else if home, err := os.UserHomeDir(); err == nil && home != "" {
			root = filepath.Join(home, ".local", "state", "llm-hub", "executions")
		} else {
			root = filepath.Join(os.TempDir(), "llm-hub-executions")
		}
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("%w: resolve evidence root: %v", ErrExecutionEvidence, err)
	}
	repoAbs, err := filepath.Abs(repositoryRoot)
	if err != nil {
		return "", fmt.Errorf("%w: resolve repository root: %v", ErrExecutionEvidence, err)
	}
	rel, err := filepath.Rel(repoAbs, abs)
	if err != nil {
		return "", fmt.Errorf("%w: compare evidence and repository roots: %v", ErrExecutionEvidence, err)
	}
	if rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))) {
		return "", fmt.Errorf("%w: evidence root must be outside repository", ErrExecutionEvidence)
	}
	if err := os.MkdirAll(abs, 0o700); err != nil {
		return "", fmt.Errorf("%w: create evidence root: %v", ErrExecutionEvidence, err)
	}
	return abs, nil
}

func writePrivateFile(path string, content []byte) error {
	if err := os.WriteFile(path, content, 0o600); err != nil {
		return fmt.Errorf("%w: write %s: %v", ErrExecutionEvidence, filepath.Base(path), err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return fmt.Errorf("%w: chmod %s: %v", ErrExecutionEvidence, filepath.Base(path), err)
	}
	return nil
}

func executionArgsForEvidence(request ExecutionRequest, repositoryRoot string) []string {
	args := []string{"run", "--model", request.Model, "--dir", repositoryRoot}
	if request.Variant != "" {
		args = append(args, "--variant", request.Variant)
	}
	if request.AutoApprove {
		args = append(args, "--auto")
	}
	args = append(args, "<prompt sha256="+contentSHA256([]byte(request.Prompt))+" bytes="+strconv.Itoa(len([]byte(request.Prompt)))+">")
	return args
}
