package workstation

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type scriptedExecutionCall struct {
	Name string
	Args []string
	Dir  string
}

type scriptedExecutionRunner struct {
	Calls   []scriptedExecutionCall
	Results []ExecutionCommandResult
	Errors  []error
}

func (r *scriptedExecutionRunner) Run(_ context.Context, name string, args []string, dir string, _ []string, _ int) (ExecutionCommandResult, error) {
	r.Calls = append(r.Calls, scriptedExecutionCall{Name: name, Args: append([]string(nil), args...), Dir: dir})
	index := len(r.Calls) - 1
	var result ExecutionCommandResult
	if index < len(r.Results) {
		result = r.Results[index]
	}
	var err error
	if index < len(r.Errors) {
		err = r.Errors[index]
	}
	return result, err
}

func executionRequestFor(t *testing.T, repository *GitRepository, prompt string) ExecutionRequest {
	t.Helper()
	snapshot := inspectRepo(t, repository.root)
	return ExecutionRequest{
		RepositoryID:       snapshot.RepositoryID,
		ExpectedHeadCommit: snapshot.HeadCommit,
		Prompt:             prompt,
		Model:              "opencode/space-bunny-free",
		Variant:            "max",
		AutoApprove:        true,
		TimeoutSeconds:     90,
	}
}

func newScriptedExecutor(t *testing.T, repository *GitRepository, runner *scriptedExecutionRunner) *OpenCodeExecutor {
	t.Helper()
	return &OpenCodeExecutor{
		Repository:      repository,
		Binary:          "opencode",
		EvidenceRoot:    filepath.Join(t.TempDir(), "evidence"),
		Runner:          runner,
		Now: func() time.Time {
			return time.Date(2026, 9, 27, 23, 30, 0, 123456789, time.UTC)
		},
		TranscriptLimit: 1024,
	}
}

func successExecutionRunner() *scriptedExecutionRunner {
	return &scriptedExecutionRunner{Results: []ExecutionCommandResult{
		{Output: []byte("opencode 1.2.3\n"), ExitCode: 0},
		{Output: []byte("Usage: opencode run --model MODEL --dir DIR --variant NAME --auto\n"), ExitCode: 0},
		{Output: []byte("builder transcript\n"), ExitCode: 0},
	}}
}

func TestExecutorBindsExactRepositoryPromptAndCLIContract(t *testing.T) {
	dir := initRepo(t, "repo")
	repository := NewGitRepository(dir)
	runner := successExecutionRunner()
	executor := newScriptedExecutor(t, repository, runner)
	request := executionRequestFor(t, repository, "build exactly this\n")

	result, err := executor.Execute(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "PASS" || result.ExitCode != 0 {
		t.Fatalf("result=%+v", result)
	}
	if result.RepositoryID != request.RepositoryID || result.StartingHeadCommit != request.ExpectedHeadCommit {
		t.Fatalf("identity drift in result: %+v", result)
	}
	if result.PromptSHA256 != contentSHA256([]byte(request.Prompt)) || result.PromptBytes != len([]byte(request.Prompt)) {
		t.Fatalf("prompt identity mismatch: %+v", result)
	}
	if result.OpenCodeRunHelpSHA256 == "" || result.OpenCodeVersion != "opencode 1.2.3" {
		t.Fatalf("missing cli identity: %+v", result)
	}
	if len(runner.Calls) != 3 {
		t.Fatalf("calls=%d", len(runner.Calls))
	}
	wantRun := []string{
		"run", "--model", request.Model, "--dir", dir,
		"--variant", request.Variant, "--auto", request.Prompt,
	}
	if strings.Join(runner.Calls[2].Args, "\x00") != strings.Join(wantRun, "\x00") {
		t.Fatalf("run args=%q want=%q", runner.Calls[2].Args, wantRun)
	}
	for _, name := range []string{"prompt.txt", "opencode-version.txt", "opencode-run-help.txt", "transcript.log", "execution.json"} {
		path := filepath.Join(result.EvidenceDir, name)
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("%s missing: %v", name, err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("%s mode=%o", name, info.Mode().Perm())
		}
	}
	promptBytes, _ := os.ReadFile(filepath.Join(result.EvidenceDir, "prompt.txt"))
	if string(promptBytes) != request.Prompt {
		t.Fatalf("prompt evidence mismatch %q", promptBytes)
	}
}

func TestExecutorRejectsIdentityHeadAndDirtyConflictsBeforeOpenCode(t *testing.T) {
	dir := initRepo(t, "repo")
	repository := NewGitRepository(dir)

	tests := []struct {
		name   string
		mutate func(*ExecutionRequest)
		dirty  bool
		target error
	}{
		{"repository identity", func(r *ExecutionRequest) { r.RepositoryID = strings.Repeat("a", 64) }, false, ErrExecutionRepositoryConflict},
		{"head identity", func(r *ExecutionRequest) { r.ExpectedHeadCommit = strings.Repeat("b", 40) }, false, ErrExecutionRepositoryConflict},
		{"dirty worktree", func(r *ExecutionRequest) {}, true, ErrExecutionRepositoryDirty},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.dirty {
				if err := os.WriteFile(filepath.Join(dir, "dirty.txt"), []byte("dirty\n"), 0o644); err != nil {
					t.Fatal(err)
				}
				defer os.Remove(filepath.Join(dir, "dirty.txt"))
			}
			runner := successExecutionRunner()
			executor := newScriptedExecutor(t, repository, runner)
			request := executionRequestFor(t, repository, "prompt")
			tc.mutate(&request)
			_, err := executor.Execute(context.Background(), request)
			if !errors.Is(err, tc.target) {
				t.Fatalf("err=%v want %v", err, tc.target)
			}
			if len(runner.Calls) != 0 {
				t.Fatalf("opencode invoked before preflight rejection: %+v", runner.Calls)
			}
		})
	}
}

func TestExecutorRejectsMissingAdvertisedFlags(t *testing.T) {
	dir := initRepo(t, "repo")
	repository := NewGitRepository(dir)
	runner := &scriptedExecutionRunner{Results: []ExecutionCommandResult{
		{Output: []byte("opencode 1.2.3\n"), ExitCode: 0},
		{Output: []byte("Usage: opencode run --model MODEL --dir DIR\n"), ExitCode: 0},
	}}
	executor := newScriptedExecutor(t, repository, runner)
	request := executionRequestFor(t, repository, "prompt")
	_, err := executor.Execute(context.Background(), request)
	if !errors.Is(err, ErrExecutionContract) {
		t.Fatalf("err=%v", err)
	}
	if len(runner.Calls) != 2 {
		t.Fatalf("run executed despite contract mismatch: %+v", runner.Calls)
	}
}

func TestExecutorRecordsNonzeroModelExitAsFailedExecution(t *testing.T) {
	dir := initRepo(t, "repo")
	repository := NewGitRepository(dir)
	runner := successExecutionRunner()
	runner.Results[2] = ExecutionCommandResult{Output: []byte("model failed\n"), ExitCode: 17}
	executor := newScriptedExecutor(t, repository, runner)
	result, err := executor.Execute(context.Background(), executionRequestFor(t, repository, "prompt"))
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "FAIL" || result.ExitCode != 17 {
		t.Fatalf("result=%+v", result)
	}
	transcript, err := os.ReadFile(filepath.Join(result.EvidenceDir, "transcript.log"))
	if err != nil || string(transcript) != "model failed\n" {
		t.Fatalf("transcript=%q err=%v", transcript, err)
	}
}

func TestExecutorRefusesEvidenceRootInsideRepository(t *testing.T) {
	dir := initRepo(t, "repo")
	repository := NewGitRepository(dir)
	runner := successExecutionRunner()
	executor := newScriptedExecutor(t, repository, runner)
	executor.EvidenceRoot = filepath.Join(dir, ".llm-hub-evidence")
	_, err := executor.Execute(context.Background(), executionRequestFor(t, repository, "prompt"))
	if !errors.Is(err, ErrExecutionEvidence) {
		t.Fatalf("err=%v", err)
	}
	if len(runner.Calls) != 0 {
		t.Fatalf("opencode invoked despite unsafe evidence root")
	}
}

func TestExecutorRejectsInvalidRequests(t *testing.T) {
	dir := initRepo(t, "repo")
	repository := NewGitRepository(dir)
	cases := []ExecutionRequest{
		{},
		{RepositoryID: "r", ExpectedHeadCommit: "h", Model: "m", Prompt: ""},
		{RepositoryID: "r", ExpectedHeadCommit: "h", Model: "m", Prompt: "p", TimeoutSeconds: maxExecutionTimeoutSeconds + 1},
		{RepositoryID: "r", ExpectedHeadCommit: "h", Model: "m\x00bad", Prompt: "p"},
	}
	for i, request := range cases {
		executor := newScriptedExecutor(t, repository, successExecutionRunner())
		_, err := executor.Execute(context.Background(), request)
		if !errors.Is(err, ErrExecutionInvalidRequest) {
			t.Fatalf("case %d err=%v", i, err)
		}
	}
}

func TestLimitedCaptureTruncatesWithoutShortWrites(t *testing.T) {
	capture := &limitedCapture{limit: 4}
	n, err := capture.Write([]byte("abcdef"))
	if err != nil || n != 6 {
		t.Fatalf("write n=%d err=%v", n, err)
	}
	if capture.data.String() != "abcd" || !capture.truncated {
		t.Fatalf("capture=%q truncated=%v", capture.data.String(), capture.truncated)
	}
}
