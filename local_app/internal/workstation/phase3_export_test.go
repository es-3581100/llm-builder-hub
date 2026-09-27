package workstation

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPhase3StaticExportContainsNoSourceMutationSurface(t *testing.T) {
	dir := initRepo(t, "repo")
	store := newTestStore(t)
	srv := &Server{Store: store, Repository: NewGitRepository(dir)}
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/export", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	for _, forbidden := range []string{
		"WRITE FILE",
		"EDIT SOURCE",
		"CANCEL SOURCE EDIT",
		"/api/write-file",
		"data-action=\"write-file\"",
		"<textarea",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("static export contains Phase-3 mutation surface %q", forbidden)
		}
	}
}
