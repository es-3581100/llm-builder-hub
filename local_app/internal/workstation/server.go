package workstation

import (
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
	Store *Store
	Log   *log.Logger
}

type stateEnvelope struct {
	State State  `json:"state"`
	Hash  string `json:"state_sha256"`
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	assets, err := fs.Sub(webFS, "web")
	if err != nil {
		panic(err)
	}
	mux.Handle("GET /assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(assets))))
	mux.HandleFunc("GET /api/state", s.getState)
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

func (s *Server) getState(w http.ResponseWriter, _ *http.Request) {
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
	writeJSON(w, http.StatusOK, stateEnvelope{State: state, Hash: hash})
}

func (s *Server) saveState(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20))
	dec.DisallowUnknownFields()
	var next State
	if err := dec.Decode(&next); err != nil {
		http.Error(w, "invalid state: "+err.Error(), http.StatusBadRequest)
		return
	}
	saved, err := s.Store.Save(next)
	if errors.Is(err, ErrRevisionConflict) {
		http.Error(w, "CONFLICTED: authoritative local state changed; refresh before saving", http.StatusConflict)
		return
	}
	if err != nil {
		http.Error(w, "save failed: "+err.Error(), http.StatusBadRequest)
		return
	}
	hash, err := s.Store.Hash()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, stateEnvelope{State: saved, Hash: hash})
}

func (s *Server) exportHTML(w http.ResponseWriter, _ *http.Request) {
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
	b, err := ExportHTML(state, hash)
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
