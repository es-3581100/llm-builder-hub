package workstation

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func gitStatusPorcelain(t *testing.T, dir string) string {
	t.Helper()
	cmd := exec.Command("git", "-C", dir, "status", "--porcelain=v1", "--untracked-files=all")
	cmd.Env = append(os.Environ(), "LC_ALL=C", "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	if err != nil { t.Fatalf("git status: %v\n%s", err, out) }
	return string(out)
}

func decodeStateEnvelope(t *testing.T, rr *httptest.ResponseRecorder) stateEnvelope {
	t.Helper()
	var env stateEnvelope
	if err := json.Unmarshal(rr.Body.Bytes(), &env); err != nil { t.Fatalf("decode response: %v\n%s", err, rr.Body.String()) }
	return env
}

func TestStateRefreshObservesGitWithoutAdvancingWorkstationRevision(t *testing.T) {
	dir := initRepo(t, "repo")
	store := newTestStore(t)
	srv := &Server{Store: store, Repository: NewGitRepository(dir)}
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/state", nil))
	if rr.Code != http.StatusOK { t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String()) }
	first := decodeStateEnvelope(t, rr)
	if first.Repository == nil || !first.Repository.Clean { t.Fatalf("first repository=%+v", first.Repository) }
	rev := first.State.Revision
	if err := os.WriteFile(filepath.Join(dir, "alpha.txt"), []byte("external change\n"), 0o644); err != nil { t.Fatal(err) }
	rr = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/state", nil))
	second := decodeStateEnvelope(t, rr)
	if second.State.Revision != rev { t.Fatalf("refresh advanced workstation revision %d -> %d", rev, second.State.Revision) }
	if second.Repository == nil || second.Repository.Clean || len(second.Repository.Unstaged) != 1 { t.Fatalf("second repository=%+v", second.Repository) }
	if first.Repository.SnapshotSHA256 == second.Repository.SnapshotSHA256 { t.Fatal("repository snapshot hash did not change") }
}

func TestSaveMutatesWorkstationStateOnly(t *testing.T) {
	dir := initRepo(t, "repo")
	source := filepath.Join(dir, "alpha.txt")
	beforeBytes, err := os.ReadFile(source); if err != nil { t.Fatal(err) }
	beforeStatus := gitStatusPorcelain(t, dir)
	store := newTestStore(t)
	state, _ := store.Load()
	state.Project.Name = "renamed workstation label"
	state.Project.Root = "/not/the/repository"
	state.Project.Branch = "not-a-checkout"
	payload, _ := json.Marshal(state)
	srv := &Server{Store: store, Repository: NewGitRepository(dir)}
	req := httptest.NewRequest(http.MethodPost, "/api/save", strings.NewReader(string(payload)))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK { t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String()) }
	afterBytes, _ := os.ReadFile(source)
	if string(afterBytes) != string(beforeBytes) { t.Fatal("source bytes changed during workstation SAVE") }
	afterStatus := gitStatusPorcelain(t, dir)
	if afterStatus != beforeStatus { t.Fatalf("git status changed during SAVE: %q -> %q", beforeStatus, afterStatus) }
	env := decodeStateEnvelope(t, rr)
	if env.Repository == nil || env.Repository.Root != dir { t.Fatalf("saved project root retargeted inspector: %+v", env.Repository) }
	if env.State.Project.Root != "/not/the/repository" { t.Fatalf("workstation metadata did not save: %q", env.State.Project.Root) }
}

func TestRepositoryEndpointAndStaticExportAreReadOnlyProjections(t *testing.T) {
	dir := initRepo(t, "repo")
	store := newTestStore(t)
	srv := &Server{Store: store, Repository: NewGitRepository(dir)}
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/repository", nil))
	if rr.Code != http.StatusOK { t.Fatalf("repository status=%d body=%s", rr.Code, rr.Body.String()) }
	var repo RepositorySnapshot
	if err := json.Unmarshal(rr.Body.Bytes(), &repo); err != nil { t.Fatal(err) }
	if repo.Authority != "GIT" || repo.RepositoryID == "" || repo.HeadCommit == "" { t.Fatalf("repository=%+v", repo) }
	rr = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/export", nil))
	if rr.Code != http.StatusOK { t.Fatalf("export status=%d body=%s", rr.Code, rr.Body.String()) }
	body := rr.Body.String()
	for _, want := range []string{"data-repository-authority=\"git\"", "LOCAL GIT REPOSITORY", "data-authority=\"repository\"", "git_worktree", "alpha.txt", "READ ONLY"} {
		if !strings.Contains(body, want) { t.Fatalf("export missing %q", want) }
	}
	for _, bad := range []string{"<textarea", "data-action=\"save\"", "SAVE CHANGES"} {
		if strings.Contains(body, bad) { t.Fatalf("export contains mutation surface %q", bad) }
	}
}
