package workstation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

type fakeExecutionProvider struct {
	mu      sync.Mutex
	request ExecutionRequest
	result  ExecutionResult
	err     error
	started chan struct{}
	release chan struct{}
}

func (f *fakeExecutionProvider) Execute(_ context.Context, request ExecutionRequest) (ExecutionResult, error) {
	f.mu.Lock()
	f.request = request
	started := f.started
	release := f.release
	result := f.result
	err := f.err
	f.mu.Unlock()
	if started != nil {
		close(started)
	}
	if release != nil {
		<-release
	}
	return result, err
}

func postExecution(t *testing.T, server *Server, request ExecutionRequest) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	httpRequest := httptest.NewRequest(http.MethodPost, "/api/run", bytes.NewReader(raw))
	httpRequest.Header.Set("Content-Type", "application/json")
	server.Handler().ServeHTTP(rec, httpRequest)
	return rec
}

func TestRunEndpointBindsRequestReturnsResultAndDoesNotLogPrompt(t *testing.T) {
	provider := &fakeExecutionProvider{result: ExecutionResult{
		RunID: "run-1", Status: "PASS", ExitCode: 0,
	}}
	var logs bytes.Buffer
	server := &Server{
		Store:    newTestStore(t),
		Executor: provider,
		Log:      log.New(&logs, "", 0),
	}
	request := ExecutionRequest{
		RepositoryID:       strings.Repeat("a", 64),
		ExpectedHeadCommit: strings.Repeat("b", 40),
		Prompt:             "do not leak this exact prompt body",
		Model:              "opencode/space-bunny-free",
		Variant:            "max",
		AutoApprove:        true,
		TimeoutSeconds:     90,
	}
	rec := postExecution(t, server, request)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var result ExecutionResult
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.RunID != "run-1" || result.Status != "PASS" {
		t.Fatalf("result=%+v", result)
	}
	provider.mu.Lock()
	gotRequest := provider.request
	provider.mu.Unlock()
	if gotRequest.Prompt != request.Prompt || gotRequest.RepositoryID != request.RepositoryID {
		t.Fatalf("request binding=%+v", gotRequest)
	}
	gotLog := logs.String()
	if strings.Contains(gotLog, request.Prompt) {
		t.Fatalf("service log leaked prompt: %q", gotLog)
	}
	if !strings.Contains(gotLog, "prompt_sha256="+contentSHA256([]byte(request.Prompt))) ||
		!strings.Contains(gotLog, "prompt_bytes=34") ||
		!strings.Contains(gotLog, "run_id=run-1") {
		t.Fatalf("service log missing execution identity: %q", gotLog)
	}
}

func TestRunEndpointTypedErrors(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"invalid", ErrExecutionInvalidRequest, 400, "INVALID_REQUEST"},
		{"repo conflict", ErrExecutionRepositoryConflict, 409, "REPOSITORY_CONFLICT"},
		{"dirty", ErrExecutionRepositoryDirty, 409, "REPOSITORY_DIRTY"},
		{"unavailable", ErrExecutionUnavailable, 503, "OPENCODE_UNAVAILABLE"},
		{"contract", ErrExecutionContract, 412, "OPENCODE_CONTRACT_MISMATCH"},
		{"transport", ErrExecutionTransport, 502, "EXECUTION_TRANSPORT_FAILURE"},
		{"evidence", ErrExecutionEvidence, 500, "EVIDENCE_FAILURE"},
		{"unknown", errors.New("boom"), 500, "INTERNAL_ERROR"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := &Server{
				Store:    newTestStore(t),
				Executor: &fakeExecutionProvider{err: tc.err},
			}
			rec := postExecution(t, server, ExecutionRequest{
				RepositoryID: "repo", ExpectedHeadCommit: "head",
				Prompt: "prompt", Model: "model",
			})
			if rec.Code != tc.status {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
			var body executionErrorEnvelope
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body.Code != tc.code {
				t.Fatalf("code=%q want=%q", body.Code, tc.code)
			}
		})
	}
}

func TestRunEndpointRejectsMalformedJSONAndMissingExecutor(t *testing.T) {
	server := &Server{Store: newTestStore(t)}
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/run", strings.NewReader("{}")))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("missing executor status=%d", rec.Code)
	}
	var missing executionErrorEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &missing); err != nil {
		t.Fatal(err)
	}
	if missing.Code != "EXECUTION_NOT_CONFIGURED" {
		t.Fatalf("missing executor code=%q", missing.Code)
	}

	server.Executor = &fakeExecutionProvider{}
	rec = httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/run", strings.NewReader("{")))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("malformed status=%d body=%s", rec.Code, rec.Body.String())
	}
	var malformed executionErrorEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &malformed); err != nil {
		t.Fatal(err)
	}
	if malformed.Code != "INVALID_REQUEST" {
		t.Fatalf("malformed code=%q", malformed.Code)
	}
}

func TestRunEndpointAllowsOnlyOneActiveExecution(t *testing.T) {
	provider := &fakeExecutionProvider{
		result:  ExecutionResult{RunID: "run-1", Status: "PASS"},
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	server := &Server{Store: newTestStore(t), Executor: provider}
	request := ExecutionRequest{
		RepositoryID: "repo", ExpectedHeadCommit: "head",
		Prompt: "prompt", Model: "model",
	}

	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		done <- postExecution(t, server, request)
	}()
	<-provider.started

	status := httptest.NewRecorder()
	server.Handler().ServeHTTP(status, httptest.NewRequest(http.MethodGet, "/api/run-status", nil))
	if status.Code != http.StatusOK || !strings.Contains(status.Body.String(), "\"running\":true") {
		t.Fatalf("run status=%d body=%s", status.Code, status.Body.String())
	}

	second := postExecution(t, server, request)
	if second.Code != http.StatusConflict {
		t.Fatalf("second status=%d body=%s", second.Code, second.Body.String())
	}
	var body executionErrorEnvelope
	if err := json.Unmarshal(second.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Code != "EXECUTION_BUSY" {
		t.Fatalf("second code=%q", body.Code)
	}

	close(provider.release)
	first := <-done
	if first.Code != http.StatusOK {
		t.Fatalf("first status=%d body=%s", first.Code, first.Body.String())
	}

	status = httptest.NewRecorder()
	server.Handler().ServeHTTP(status, httptest.NewRequest(http.MethodGet, "/api/run-status", nil))
	if status.Code != http.StatusOK || !strings.Contains(status.Body.String(), "\"running\":false") {
		t.Fatalf("final run status=%d body=%s", status.Code, status.Body.String())
	}
}
