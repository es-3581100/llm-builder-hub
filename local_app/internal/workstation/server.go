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
