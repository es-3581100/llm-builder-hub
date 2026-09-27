package workstation

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "state.json")
	store := NewStore(path)
	if err := store.Ensure(); err != nil {
		t.Fatal(err)
	}
	return store
}

func TestDefaultStateContract(t *testing.T) {
	s := DefaultState()
	if err := s.Validate(); err != nil {
		t.Fatal(err)
	}
	if s.Authority != "LOCAL" {
		t.Fatalf("authority=%q", s.Authority)
	}
	if len(s.Sliders) != 3 {
		t.Fatalf("sliders=%d", len(s.Sliders))
	}
	for _, slider := range s.Sliders {
		if len(slider.Tools) != 5 {
			t.Fatalf("slider %s tools=%d", slider.ID, len(slider.Tools))
		}
	}
	for i, doc := range s.Documents {
		if doc.Order != i+1 {
			t.Fatalf("doc %s order=%d want %d", doc.ID, doc.Order, i+1)
		}
	}
}

func TestStoreExplicitSaveAndRevisionConflict(t *testing.T) {
	store := newTestStore(t)
	first, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	stale := first
	first.Documents[0].Content = "saved mutation"
	saved, err := store.Save(first)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Revision != first.Revision+1 {
		t.Fatalf("revision=%d", saved.Revision)
	}
	stale.Documents[0].Content = "stale mutation"
	if _, err := store.Save(stale); err != ErrRevisionConflict {
		t.Fatalf("want conflict, got %v", err)
	}
	got, _ := store.Load()
	if got.Documents[0].Content != "saved mutation" {
		t.Fatalf("authoritative content=%q", got.Documents[0].Content)
	}
}

func TestStoreUsesPrivatePermissions(t *testing.T) {
	store := newTestStore(t)
	info, err := os.Stat(store.Path())
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode=%#o", info.Mode().Perm())
	}
}

func TestStaticExportIsReadOnlySemanticProjection(t *testing.T) {
	state := DefaultState()
	html, err := ExportHTML(state, strings.Repeat("a", 64))
	if err != nil {
		t.Fatal(err)
	}
	body := string(html)
	checks := []string{
		`data-hub-projection="static"`, `STATIC PROJECTION`, `READ ONLY`,
		`data-hub-document="true"`, `data-state="saved"`, `data-authority="projection"`,
		`type="application/json"`,
	}
	for _, check := range checks {
		if !strings.Contains(body, check) {
			t.Fatalf("missing %q", check)
		}
	}
	forbidden := []string{`<textarea`, `data-action="save"`, `SAVE CHANGES`}
	for _, check := range forbidden {
		if strings.Contains(body, check) {
			t.Fatalf("static export contains mutation surface %q", check)
		}
	}
}

func TestExportEndpointUsesPersistedStateOnly(t *testing.T) {
	store := newTestStore(t)
	persisted, _ := store.Load()
	persisted.Documents[0].Content = "SAVED_ONLY_SENTINEL"
	if _, err := store.Save(persisted); err != nil {
		t.Fatal(err)
	}
	srv := &Server{Store: store}
	req := httptest.NewRequest(http.MethodGet, "/api/export", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "SAVED_ONLY_SENTINEL") {
		t.Fatal("persisted value missing")
	}
	if strings.Contains(rr.Body.String(), "UNSAVED_DRAFT_SENTINEL") {
		t.Fatal("draft leaked into export")
	}
}

func TestSaveEndpointRejectsMalformedToolSpine(t *testing.T) {
	store := newTestStore(t)
	state, _ := store.Load()
	state.Sliders[0].Tools = state.Sliders[0].Tools[:4]
	payload, _ := json.Marshal(state)
	srv := &Server{Store: store}
	req := httptest.NewRequest(http.MethodPost, "/api/save", strings.NewReader(string(payload)))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestSecurityHeaders(t *testing.T) {
	store := newTestStore(t)
	srv := &Server{Store: store}
	req := httptest.NewRequest(http.MethodGet, "/api/state", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	if rr.Header().Get("Content-Security-Policy") == "" {
		t.Fatal("missing CSP")
	}
	if rr.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("cache=%q", rr.Header().Get("Cache-Control"))
	}
	b, _ := io.ReadAll(rr.Result().Body)
	if !strings.Contains(string(b), `"authority":"LOCAL"`) {
		t.Fatal("authority missing")
	}
}
