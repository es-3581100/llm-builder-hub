package workstation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type inspectOnlyRepository struct {
	snapshot RepositorySnapshot
}

func (r inspectOnlyRepository) Inspect(context.Context) (RepositorySnapshot, error) {
	return r.snapshot, nil
}

func postWriteFile(t *testing.T, srv *Server, request WriteFileRequest) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/write-file", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	srv.Handler().ServeHTTP(rr, req)
	return rr
}

func decodeWriteResult(t *testing.T, rr *httptest.ResponseRecorder) WriteFileResult {
	t.Helper()
	var result WriteFileResult
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode write result: %v\n%s", err, rr.Body.String())
	}
	return result
}

func decodeWriteError(t *testing.T, rr *httptest.ResponseRecorder) writeFileErrorEnvelope {
	t.Helper()
	var result writeFileErrorEnvelope
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode write error: %v\n%s", err, rr.Body.String())
	}
	return result
}

func TestWriteFileEndpointMutatesWorkingTreeButNotWorkstationStateOrIndex(t *testing.T) {
	dir := initRepo(t, "repo")
	repository := NewGitRepository(dir)
	store := newTestStore(t)
	stateBefore, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	indexBefore := shaFile(t, filepath.Join(dir, ".git", "index"))
	request := writeRequestFor(t, repository, "alpha.txt", "written through API\n")

	var logs bytes.Buffer
	srv := &Server{
		Store:      store,
		Repository: repository,
		Log:        log.New(&logs, "", 0),
	}
	rr := postWriteFile(t, srv, request)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	result := decodeWriteResult(t, rr)
	if result.Repository.Clean || len(result.Repository.Unstaged) != 1 || result.Repository.Unstaged[0].Path != "alpha.txt" {
		t.Fatalf("success response missing fresh Git diff: %+v", result.Repository.Unstaged)
	}
	got, err := os.ReadFile(filepath.Join(dir, "alpha.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != request.Content {
		t.Fatalf("working tree content=%q", got)
	}
	indexAfter := shaFile(t, filepath.Join(dir, ".git", "index"))
	if indexAfter != indexBefore {
		t.Fatalf("index changed %s -> %s", indexBefore, indexAfter)
	}
	stateAfter, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if stateAfter.Revision != stateBefore.Revision {
		t.Fatalf("WRITE FILE changed workstation revision %d -> %d", stateBefore.Revision, stateAfter.Revision)
	}
	if !strings.Contains(logs.String(), "POST /api/write-file status=200 code=OK path=\"alpha.txt\"") {
		t.Fatalf("missing structured success log: %q", logs.String())
	}
	if !strings.Contains(logs.String(), "before_sha256="+request.ExpectedContentSHA256) ||
		!strings.Contains(logs.String(), "after_sha256="+contentSHA256([]byte(request.Content))) {
		t.Fatalf("missing before/after hashes in success log: %q", logs.String())
	}
}

func TestWriteFileEndpointReturnsTypedContentConflictAndPreservesExternalEdit(t *testing.T) {
	dir := initRepo(t, "repo")
	repository := NewGitRepository(dir)
	request := writeRequestFor(t, repository, "alpha.txt", "browser draft\n")
	external := []byte("external edit wins\n")
	if err := os.WriteFile(filepath.Join(dir, "alpha.txt"), external, 0o644); err != nil {
		t.Fatal(err)
	}

	var logs bytes.Buffer
	srv := &Server{
		Store:      newTestStore(t),
		Repository: repository,
		Log:        log.New(&logs, "", 0),
	}
	rr := postWriteFile(t, srv, request)
	if rr.Code != http.StatusConflict {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	body := decodeWriteError(t, rr)
	if body.Code != "CONTENT_CONFLICT" {
		t.Fatalf("code=%q error=%q", body.Code, body.Error)
	}
	got, err := os.ReadFile(filepath.Join(dir, "alpha.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(external) {
		t.Fatalf("external edit overwritten: %q", got)
	}
	if !strings.Contains(logs.String(), "status=409 code=CONTENT_CONFLICT path=\"alpha.txt\"") {
		t.Fatalf("missing structured conflict log: %q", logs.String())
	}
}

func TestWriteFileEndpointErrorMapping(t *testing.T) {
	dir := initRepo(t, "repo")
	repository := NewGitRepository(dir)
	good := writeRequestFor(t, repository, "alpha.txt", "draft\n")
	srv := &Server{Store: newTestStore(t), Repository: repository}

	t.Run("invalid request", func(t *testing.T) {
		request := good
		request.Path = "../outside.txt"
		rr := postWriteFile(t, srv, request)
		if rr.Code != http.StatusBadRequest || decodeWriteError(t, rr).Code != "INVALID_REQUEST" {
			t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("repository conflict", func(t *testing.T) {
		request := good
		request.RepositoryID = strings.Repeat("a", 64)
		rr := postWriteFile(t, srv, request)
		if rr.Code != http.StatusConflict || decodeWriteError(t, rr).Code != "REPOSITORY_CONFLICT" {
			t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("document conflict", func(t *testing.T) {
		request := good
		request.DocumentID = "git-stale-document"
		rr := postWriteFile(t, srv, request)
		if rr.Code != http.StatusConflict || decodeWriteError(t, rr).Code != "DOCUMENT_CONFLICT" {
			t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("unsupported target", func(t *testing.T) {
		if err := os.WriteFile(filepath.Join(dir, "binary.bin"), []byte{0, 1, 2}, 0o644); err != nil {
			t.Fatal(err)
		}
		gitCmd(t, dir, "add", "binary.bin")
		snapshot, err := repository.Inspect(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		request := WriteFileRequest{
			RepositoryID:          snapshot.RepositoryID,
			DocumentID:            stableRepositoryDocumentID(snapshot.RepositoryID, "binary.bin"),
			Path:                  "binary.bin",
			ExpectedContentSHA256: contentSHA256([]byte{0, 1, 2}),
			Content:               "replacement\n",
		}
		rr := postWriteFile(t, srv, request)
		if rr.Code != http.StatusUnprocessableEntity || decodeWriteError(t, rr).Code != "UNSUPPORTED_TARGET" {
			t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
		}
	})
}

func TestWriteFileEndpointRejectsMalformedJSONAndMissingWriter(t *testing.T) {
	dir := initRepo(t, "repo")
	repository := NewGitRepository(dir)
	var logs bytes.Buffer
	srv := &Server{
		Store:      newTestStore(t),
		Repository: repository,
		Log:        log.New(&logs, "", 0),
	}
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/write-file", strings.NewReader("{")))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("malformed status=%d body=%s", rr.Code, rr.Body.String())
	}
	if decodeWriteError(t, rr).Code != "INVALID_REQUEST" {
		t.Fatalf("malformed body=%s", rr.Body.String())
	}
	if !strings.Contains(logs.String(), "status=400 code=INVALID_REQUEST") {
		t.Fatalf("malformed request not logged: %q", logs.String())
	}

	snapshot, err := repository.Inspect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	srv = &Server{
		Store:      newTestStore(t),
		Repository: inspectOnlyRepository{snapshot: snapshot},
	}
	rr = postWriteFile(t, srv, writeRequestFor(t, repository, "alpha.txt", "draft\n"))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("missing writer status=%d body=%s", rr.Code, rr.Body.String())
	}
	if decodeWriteError(t, rr).Code != "WRITE_NOT_CONFIGURED" {
		t.Fatalf("missing writer body=%s", rr.Body.String())
	}
}

type failingRepositoryWriter struct {
	RepositoryProvider
	err error
}

func (w failingRepositoryWriter) WriteFile(context.Context, WriteFileRequest) (WriteFileResult, error) {
	return WriteFileResult{}, w.err
}

func TestWriteFileEndpointMapsReadbackAndUnknownFailures(t *testing.T) {
	dir := initRepo(t, "repo")
	snapshot := inspectRepo(t, dir)

	cases := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{"readback", ErrWriteReadbackMismatch, http.StatusInternalServerError, "READBACK_MISMATCH"},
		{"unknown", errors.New("boom"), http.StatusInternalServerError, "INTERNAL_ERROR"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := &Server{
				Store: newTestStore(t),
				Repository: failingRepositoryWriter{
					RepositoryProvider: inspectOnlyRepository{snapshot: snapshot},
					err:                tc.err,
				},
			}
			request := WriteFileRequest{
				RepositoryID:          snapshot.RepositoryID,
				DocumentID:            "git-test",
				Path:                  "alpha.txt",
				ExpectedContentSHA256: strings.Repeat("a", 64),
				Content:               "draft\n",
			}
			rr := postWriteFile(t, srv, request)
			if rr.Code != tc.status || decodeWriteError(t, rr).Code != tc.code {
				t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
			}
		})
	}
}
