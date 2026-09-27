package workstation

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPhase4StaticExportContainsNoExecutionSurface(t *testing.T) {
	dir := initRepo(t, "repo")
	server := &Server{Store: newTestStore(t), Repository: NewGitRepository(dir)}
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/export", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, forbidden := range []string{
		"RUN OPENCODE",
		"/api/run",
		"execution-prompt",
		"data-action=\"run-opencode\"",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("static export contains execution surface %q", forbidden)
		}
	}
}
