package workstation

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"net/http"
)

//go:embed web/*
var webFS embed.FS

type Server struct {
	Store      *Store
	Repository RepositoryProvider
	Log        *log.Logger
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
	mux.HandleFunc("POST /api/stage-file", s.stageRepositoryFile)
	mux.HandleFunc("POST /api/unstage-file", s.unstageRepositoryFile)
	mux.HandleFunc("POST /api/commit", s.commitStagedRepository)
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

// gitControlError maps a Phase-4 local Git control sentinel onto the stable
// HTTP status/code pair. A sentinel that is not recognised is an internal
// failure rather than a silently downgraded success.
func gitControlError(err error) (int, string) {
	switch {
	case errors.Is(err, ErrGitInvalidRequest):
		return http.StatusBadRequest, "INVALID_REQUEST"
	case errors.Is(err, ErrGitCommitMessageInvalid):
		return http.StatusBadRequest, "COMMIT_MESSAGE_INVALID"
	case errors.Is(err, ErrGitRepositoryConflict):
		return http.StatusConflict, "REPOSITORY_CONFLICT"
	case errors.Is(err, ErrGitHeadConflict):
		return http.StatusConflict, "HEAD_CONFLICT"
	case errors.Is(err, ErrGitBranchConflict):
		return http.StatusConflict, "BRANCH_CONFLICT"
	case errors.Is(err, ErrGitIndexConflict):
		return http.StatusConflict, "INDEX_CONFLICT"
	case errors.Is(err, ErrGitWorktreeConflict):
		return http.StatusConflict, "WORKTREE_CONFLICT"
	case errors.Is(err, ErrGitRefUpdateConflict):
		return http.StatusConflict, "REF_UPDATE_CONFLICT"
	case errors.Is(err, ErrGitUnsupportedTarget):
		return http.StatusUnprocessableEntity, "UNSUPPORTED_TARGET"
	case errors.Is(err, ErrGitUnsupportedState):
		return http.StatusUnprocessableEntity, "UNSUPPORTED_GIT_STATE"
	case errors.Is(err, ErrGitUnsafeAttributes):
		return http.StatusUnprocessableEntity, "UNSAFE_GIT_ATTRIBUTES"
	case errors.Is(err, ErrGitNothingToCommit):
		return http.StatusUnprocessableEntity, "NOTHING_TO_COMMIT"
	default:
		return http.StatusInternalServerError, "INTERNAL_ERROR"
	}
}

// gitControlLog is the structured record of one local Git control request. It
// carries identities and routing only. File contents, the commit message, and
// every other byte of user content are deliberately absent: there is no field
// here that can hold them.
type gitControlLog struct {
	action       string
	status       int
	code         string
	path         string
	beforeIndex  string
	afterIndex   string
	expectedHead string
	newHead      string
}

func (s *Server) logGitControl(entry gitControlLog) {
	if s.Log == nil {
		return
	}
	s.Log.Printf(
		"%s status=%d code=%s path=%q before_index_sha256=%s after_index_sha256=%s expected_head_commit=%s new_head_commit=%s",
		entry.action, entry.status, entry.code, entry.path,
		entry.beforeIndex, entry.afterIndex, entry.expectedHead, entry.newHead,
	)
}

func (s *Server) stageRepositoryFile(w http.ResponseWriter, r *http.Request) {
	controller, ok := s.Repository.(GitCommitController)
	if !ok || controller == nil {
		writeJSON(w, http.StatusNotFound, writeFileErrorEnvelope{
			Error: "local git control is not configured",
			Code:  "WRITE_NOT_CONFIGURED",
		})
		return
	}

	defer r.Body.Close()
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20))
	dec.DisallowUnknownFields()
	var request StageFileRequest
	if err := dec.Decode(&request); err != nil {
		status, code := http.StatusBadRequest, "INVALID_REQUEST"
		s.logGitControl(gitControlLog{action: "POST /api/stage-file", status: status, code: code})
		writeJSON(w, status, writeFileErrorEnvelope{Error: "invalid stage request: " + err.Error(), Code: code})
		return
	}

	result, err := controller.StageFile(r.Context(), request)
	if err != nil {
		status, code := gitControlError(err)
		s.logGitControl(gitControlLog{
			action:       "POST /api/stage-file",
			status:       status,
			code:         code,
			path:         request.Path,
			beforeIndex:  result.BeforeIndexSHA256,
			expectedHead: result.ExpectedHeadCommit,
		})
		writeJSON(w, status, writeFileErrorEnvelope{Error: err.Error(), Code: code})
		return
	}

	s.logGitControl(gitControlLog{
		action:       "POST /api/stage-file",
		status:       http.StatusOK,
		code:         "OK",
		path:         request.Path,
		beforeIndex:  result.BeforeIndexSHA256,
		afterIndex:   result.AfterIndexSHA256,
		expectedHead: result.ExpectedHeadCommit,
		newHead:      result.HeadCommit,
	})
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) unstageRepositoryFile(w http.ResponseWriter, r *http.Request) {
	controller, ok := s.Repository.(GitCommitController)
	if !ok || controller == nil {
		writeJSON(w, http.StatusNotFound, writeFileErrorEnvelope{
			Error: "local git control is not configured",
			Code:  "WRITE_NOT_CONFIGURED",
		})
		return
	}

	defer r.Body.Close()
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20))
	dec.DisallowUnknownFields()
	var request UnstageFileRequest
	if err := dec.Decode(&request); err != nil {
		status, code := http.StatusBadRequest, "INVALID_REQUEST"
		s.logGitControl(gitControlLog{action: "POST /api/unstage-file", status: status, code: code})
		writeJSON(w, status, writeFileErrorEnvelope{Error: "invalid unstage request: " + err.Error(), Code: code})
		return
	}

	result, err := controller.UnstageFile(r.Context(), request)
	if err != nil {
		status, code := gitControlError(err)
		s.logGitControl(gitControlLog{
			action:       "POST /api/unstage-file",
			status:       status,
			code:         code,
			path:         request.Path,
			beforeIndex:  result.BeforeIndexSHA256,
			expectedHead: result.ExpectedHeadCommit,
		})
		writeJSON(w, status, writeFileErrorEnvelope{Error: err.Error(), Code: code})
		return
	}

	s.logGitControl(gitControlLog{
		action:       "POST /api/unstage-file",
		status:       http.StatusOK,
		code:         "OK",
		path:         request.Path,
		beforeIndex:  result.BeforeIndexSHA256,
		afterIndex:   result.AfterIndexSHA256,
		expectedHead: result.ExpectedHeadCommit,
		newHead:      result.HeadCommit,
	})
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) commitStagedRepository(w http.ResponseWriter, r *http.Request) {
	controller, ok := s.Repository.(GitCommitController)
	if !ok || controller == nil {
		writeJSON(w, http.StatusNotFound, writeFileErrorEnvelope{
			Error: "local git control is not configured",
			Code:  "WRITE_NOT_CONFIGURED",
		})
		return
	}

	defer r.Body.Close()
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20))
	dec.DisallowUnknownFields()
	var request CommitStagedRequest
	if err := dec.Decode(&request); err != nil {
		status, code := http.StatusBadRequest, "INVALID_REQUEST"
		s.logGitControl(gitControlLog{action: "POST /api/commit", status: status, code: code})
		writeJSON(w, status, writeFileErrorEnvelope{Error: "invalid commit request: " + err.Error(), Code: code})
		return
	}

	result, err := controller.CommitStaged(r.Context(), request)
	if err != nil {
		status, code := gitControlError(err)
		s.logGitControl(gitControlLog{
			action:       "POST /api/commit",
			status:       status,
			code:         code,
			beforeIndex:  result.BeforeIndexSHA256,
			expectedHead: result.ExpectedHeadCommit,
		})
		writeJSON(w, status, writeFileErrorEnvelope{Error: err.Error(), Code: code})
		return
	}

	s.logGitControl(gitControlLog{
		action:       "POST /api/commit",
		status:       http.StatusOK,
		code:         "OK",
		beforeIndex:  result.BeforeIndexSHA256,
		afterIndex:   result.AfterIndexSHA256,
		expectedHead: result.ExpectedHeadCommit,
		newHead:      result.NewHeadCommit,
	})
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
