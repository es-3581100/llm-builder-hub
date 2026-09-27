package workstation

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"net/http"
	"sync"
)

//go:embed web/*
var webFS embed.FS

type Server struct {
	Store      *Store
	Repository RepositoryProvider
	Executor   ExecutionProvider
	Log        *log.Logger

	executionMu      sync.Mutex
	executionRunning bool
}

type stateEnvelope struct {
	State      State               `json:"state"`
	Hash       string              `json:"state_sha256"`
	Repository *RepositorySnapshot `json:"repository,omitempty"`
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	assets, err := fs.Sub(webFS, "web")
	if err != nil {
		panic(err)
	}
	mux.Handle("GET /assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(assets))))
	mux.HandleFunc("GET /api/state", s.getState)
	mux.HandleFunc("GET /api/repository", s.getRepository)
	mux.HandleFunc("POST /api/save", s.saveState)
	mux.HandleFunc("POST /api/write-file", s.writeRepositoryFile)
	mux.HandleFunc("POST /api/run", s.runExecutor)
	mux.HandleFunc("GET /api/run-status", s.getRunStatus)
	mux.HandleFunc("GET /api/export", s.exportHTML)
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		b, err := webFS.ReadFile("web/index.html")
		if err != nil {
			http.Error(w, "UI unavailable", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(b)
	})
	return securityHeaders(mux)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) inspectRepository(ctx context.Context) (*RepositorySnapshot, error) {
	if s.Repository == nil {
		return nil, nil
	}
	snapshot, err := s.Repository.Inspect(ctx)
	if err != nil {
		return nil, err
	}
	return &snapshot, nil
}

func (s *Server) envelope(ctx context.Context, state State, hash string) (stateEnvelope, error) {
	repository, err := s.inspectRepository(ctx)
	if err != nil {
		return stateEnvelope{}, err
	}
	return stateEnvelope{State: state, Hash: hash, Repository: repository}, nil
}

func (s *Server) getState(w http.ResponseWriter, r *http.Request) {
	state, err := s.Store.Load()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	hash, err := s.Store.Hash()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	envelope, err := s.envelope(r.Context(), state, hash)
	if err != nil {
		http.Error(w, "repository inspection failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, envelope)
}

func (s *Server) getRepository(w http.ResponseWriter, r *http.Request) {
	if s.Repository == nil {
		http.Error(w, "repository inspection is not configured", http.StatusNotFound)
		return
	}
	snapshot, err := s.Repository.Inspect(r.Context())
	if err != nil {
		http.Error(w, "repository inspection failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, snapshot)
}

func (s *Server) logSave(status int, before, after int64) {
	if s.Log == nil {
		return
	}
	s.Log.Printf("POST /api/save status=%d revision_before=%d revision_after=%d", status, before, after)
}

func (s *Server) saveState(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	status := http.StatusInternalServerError
	before, after := int64(-1), int64(-1)
	defer func() { s.logSave(status, before, after) }()

	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20))
	dec.DisallowUnknownFields()
	var next State
	if err := dec.Decode(&next); err != nil {
		status = http.StatusBadRequest
		http.Error(w, "invalid state: "+err.Error(), status)
		return
	}
	before = next.Revision

	saved, err := s.Store.Save(next)
	if errors.Is(err, ErrRevisionConflict) {
		status = http.StatusConflict
		http.Error(w, "CONFLICTED: authoritative local state changed; refresh before saving", status)
		return
	}
	if err != nil {
		status = http.StatusBadRequest
		http.Error(w, "save failed: "+err.Error(), status)
		return
	}
	after = saved.Revision

	hash, err := s.Store.Hash()
	if err != nil {
		status = http.StatusInternalServerError
		http.Error(w, err.Error(), status)
		return
	}
	envelope, err := s.envelope(r.Context(), saved, hash)
	if err != nil {
		status = http.StatusInternalServerError
		http.Error(w, "repository inspection failed after workstation save: "+err.Error(), status)
		return
	}
	status = http.StatusOK
	writeJSON(w, status, envelope)
}

type writeFileErrorEnvelope struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

func writeFileError(err error) (int, string) {
	switch {
	case errors.Is(err, ErrWriteInvalidRequest):
		return http.StatusBadRequest, "INVALID_REQUEST"
	case errors.Is(err, ErrWriteRepositoryConflict):
		return http.StatusConflict, "REPOSITORY_CONFLICT"
	case errors.Is(err, ErrWriteDocumentConflict):
		return http.StatusConflict, "DOCUMENT_CONFLICT"
	case errors.Is(err, ErrWriteContentConflict):
		return http.StatusConflict, "CONTENT_CONFLICT"
	case errors.Is(err, ErrWriteUnsupportedTarget):
		return http.StatusUnprocessableEntity, "UNSUPPORTED_TARGET"
	case errors.Is(err, ErrWriteReadbackMismatch):
		return http.StatusInternalServerError, "READBACK_MISMATCH"
	default:
		return http.StatusInternalServerError, "INTERNAL_ERROR"
	}
}

func (s *Server) logWriteFile(status int, code, path string, result WriteFileResult) {
	if s.Log == nil {
		return
	}
	s.Log.Printf(
		"POST /api/write-file status=%d code=%s path=%q before_sha256=%s after_sha256=%s",
		status, code, path, result.BeforeContentSHA256, result.AfterContentSHA256,
	)
}

func (s *Server) writeRepositoryFile(w http.ResponseWriter, r *http.Request) {
	writer, ok := s.Repository.(RepositoryWriter)
	if !ok || writer == nil {
		writeJSON(w, http.StatusNotFound, writeFileErrorEnvelope{
			Error: "repository writing is not configured",
			Code:  "WRITE_NOT_CONFIGURED",
		})
		return
	}

	defer r.Body.Close()
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20))
	dec.DisallowUnknownFields()
	var request WriteFileRequest
	if err := dec.Decode(&request); err != nil {
		status, code := http.StatusBadRequest, "INVALID_REQUEST"
		s.logWriteFile(status, code, "", WriteFileResult{})
		writeJSON(w, status, writeFileErrorEnvelope{Error: "invalid write request: " + err.Error(), Code: code})
		return
	}

	result, err := writer.WriteFile(r.Context(), request)
	if err != nil {
		status, code := writeFileError(err)
		s.logWriteFile(status, code, request.Path, result)
		writeJSON(w, status, writeFileErrorEnvelope{Error: err.Error(), Code: code})
		return
	}

	s.logWriteFile(http.StatusOK, "OK", request.Path, result)
	writeJSON(w, http.StatusOK, result)
}

type executionErrorEnvelope struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

type executionStatusEnvelope struct {
	Running bool `json:"running"`
}

func executionHTTPError(err error) (int, string) {
	switch {
	case errors.Is(err, ErrExecutionInvalidRequest):
		return http.StatusBadRequest, "INVALID_REQUEST"
	case errors.Is(err, ErrExecutionRepositoryConflict):
		return http.StatusConflict, "REPOSITORY_CONFLICT"
	case errors.Is(err, ErrExecutionRepositoryDirty):
		return http.StatusConflict, "REPOSITORY_DIRTY"
	case errors.Is(err, ErrExecutionUnavailable):
		return http.StatusServiceUnavailable, "OPENCODE_UNAVAILABLE"
	case errors.Is(err, ErrExecutionContract):
		return http.StatusPreconditionFailed, "OPENCODE_CONTRACT_MISMATCH"
	case errors.Is(err, ErrExecutionTransport):
		return http.StatusBadGateway, "EXECUTION_TRANSPORT_FAILURE"
	case errors.Is(err, ErrExecutionEvidence):
		return http.StatusInternalServerError, "EVIDENCE_FAILURE"
	default:
		return http.StatusInternalServerError, "INTERNAL_ERROR"
	}
}

func (s *Server) beginExecution() bool {
	s.executionMu.Lock()
	defer s.executionMu.Unlock()
	if s.executionRunning {
		return false
	}
	s.executionRunning = true
	return true
}

func (s *Server) finishExecution() {
	s.executionMu.Lock()
	s.executionRunning = false
	s.executionMu.Unlock()
}

func (s *Server) getRunStatus(w http.ResponseWriter, _ *http.Request) {
	s.executionMu.Lock()
	running := s.executionRunning
	s.executionMu.Unlock()
	writeJSON(w, http.StatusOK, executionStatusEnvelope{Running: running})
}

func (s *Server) logExecution(status int, code string, request ExecutionRequest, result ExecutionResult) {
	if s.Log == nil {
		return
	}
	promptSHA := contentSHA256([]byte(request.Prompt))
	s.Log.Printf(
		"POST /api/run status=%d code=%s repository_id=%s expected_head=%s prompt_sha256=%s prompt_bytes=%d model=%q variant=%q auto=%t run_id=%s execution_status=%s exit_code=%d",
		status,
		code,
		request.RepositoryID,
		request.ExpectedHeadCommit,
		promptSHA,
		len([]byte(request.Prompt)),
		request.Model,
		request.Variant,
		request.AutoApprove,
		result.RunID,
		result.Status,
		result.ExitCode,
	)
}

func (s *Server) runExecutor(w http.ResponseWriter, r *http.Request) {
	if s.Executor == nil {
		writeJSON(w, http.StatusNotFound, executionErrorEnvelope{
			Error: "local execution is not configured",
			Code:  "EXECUTION_NOT_CONFIGURED",
		})
		return
	}
	defer r.Body.Close()
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20))
	dec.DisallowUnknownFields()
	var request ExecutionRequest
	if err := dec.Decode(&request); err != nil {
		s.logExecution(http.StatusBadRequest, "INVALID_REQUEST", ExecutionRequest{}, ExecutionResult{})
		writeJSON(w, http.StatusBadRequest, executionErrorEnvelope{
			Error: "invalid execution request: " + err.Error(),
			Code:  "INVALID_REQUEST",
		})
		return
	}
	if !s.beginExecution() {
		s.logExecution(http.StatusConflict, "EXECUTION_BUSY", request, ExecutionResult{})
		writeJSON(w, http.StatusConflict, executionErrorEnvelope{
			Error: "another local execution is already running",
			Code:  "EXECUTION_BUSY",
		})
		return
	}
	defer s.finishExecution()

	// Execution is intentionally bounded by the executor's own timeout and is
	// not cancelled merely because the browser disconnects or reloads.
	result, err := s.Executor.Execute(context.Background(), request)
	if err != nil {
		status, code := executionHTTPError(err)
		s.logExecution(status, code, request, result)
		writeJSON(w, status, executionErrorEnvelope{Error: err.Error(), Code: code})
		return
	}
	s.logExecution(http.StatusOK, "OK", request, result)
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) exportHTML(w http.ResponseWriter, r *http.Request) {
	state, err := s.Store.Load()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	hash, err := s.Store.Hash()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	repository, err := s.inspectRepository(r.Context())
	if err != nil {
		http.Error(w, "repository inspection failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	b, err := ExportHTMLWithRepository(state, hash, repository)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="llm-hub-project.html"`)
	_, _ = w.Write(b)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
