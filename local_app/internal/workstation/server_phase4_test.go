package workstation

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	stageEndpoint   = "/api/stage-file"
	unstageEndpoint = "/api/unstage-file"
	commitEndpoint  = "/api/commit"
)

func phase4Server(t *testing.T, repository RepositoryProvider) *Server {
	t.Helper()
	return &Server{Store: newTestStore(t), Repository: repository}
}

func postGitControl(t *testing.T, srv *Server, endpoint string, request any) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	return postGitControlBody(t, srv, endpoint, payload)
}

func postGitControlBody(t *testing.T, srv *Server, endpoint string, payload []byte) *httptest.ResponseRecorder {
	t.Helper()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, endpoint, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	srv.Handler().ServeHTTP(rr, req)
	return rr
}

func decodeGitControlResult[T any](t *testing.T, rr *httptest.ResponseRecorder) T {
	t.Helper()
	var result T
	if err := json.Unmarshal(rr.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode %T: %v\n%s", result, err, rr.Body.String())
	}
	return result
}

func requireGitControlOK(t *testing.T, rr *httptest.ResponseRecorder) {
	t.Helper()
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d want 200 body=%s", rr.Code, rr.Body.String())
	}
}

// requireJSONField proves a response carries the key on the wire, so a
// "present" assertion cannot be satisfied by a decoded zero value alone.
func requireJSONField(t *testing.T, rr *httptest.ResponseRecorder, field string) {
	t.Helper()
	var shape map[string]json.RawMessage
	if err := json.Unmarshal(rr.Body.Bytes(), &shape); err != nil {
		t.Fatalf("decode response shape: %v\n%s", err, rr.Body.String())
	}
	if _, ok := shape[field]; !ok {
		t.Fatalf("response is missing the %q key: %s", field, rr.Body.String())
	}
}

func requireGitControlError(t *testing.T, rr *httptest.ResponseRecorder, wantStatus int, wantCode string) writeFileErrorEnvelope {
	t.Helper()
	if rr.Code != wantStatus {
		t.Fatalf("status=%d want %d body=%s", rr.Code, wantStatus, rr.Body.String())
	}
	envelope := decodeWriteError(t, rr)
	if envelope.Code != wantCode {
		t.Fatalf("code=%q want %q error=%q", envelope.Code, wantCode, envelope.Error)
	}
	if envelope.Error == "" {
		t.Fatalf("error envelope has no message: %s", rr.Body.String())
	}
	return envelope
}

func requireChangePath(t *testing.T, label string, changes []GitChange, path, kind string) {
	t.Helper()
	if len(changes) != 1 || changes[0].Path != path || changes[0].Kind != kind {
		t.Fatalf("%s = %+v, want one %s entry for %q", label, changes, kind, path)
	}
}

func requireNoChange(t *testing.T, label string, changes []GitChange) {
	t.Helper()
	if len(changes) != 0 {
		t.Fatalf("%s = %+v, want none", label, changes)
	}
}

// stageAlphaFixture leaves the fixture with one unstaged tracked modification
// to alpha.txt and returns the identity-bound stage request for it.
func stageAlphaFixture(t *testing.T, f *phase4Fixture, content string) StageFileRequest {
	t.Helper()
	f.write(t, "alpha.txt", content)
	return f.stageRequest(t, "alpha.txt")
}

func TestPhase4EndpointsExposeTypedSuccessAndFreshRepositoryState(t *testing.T) {
	f := newPhase4Fixture(t)
	srv := phase4Server(t, f.repo)

	request := stageAlphaFixture(t, f, "alpha edited for phase four\n")
	indexBefore := request.ExpectedIndexSHA256
	headBefore := request.ExpectedHeadCommit
	modeBefore := f.indexMode(t, "alpha.txt")
	historyBefore := len(f.snapshot(t).History)

	t.Run("stage", func(t *testing.T) {
		rr := postGitControl(t, srv, stageEndpoint, request)
		requireGitControlOK(t, rr)
		requireJSONField(t, rr, "repository")
		result := decodeGitControlResult[StageFileResult](t, rr)

		if result.Repository.RepositoryID != request.RepositoryID {
			t.Fatalf("fresh snapshot repository id %q != %q", result.Repository.RepositoryID, request.RepositoryID)
		}
		if result.Repository.HeadCommit != headBefore || result.HeadCommit != headBefore {
			t.Fatalf("stage moved HEAD: snapshot=%q result=%q want %q", result.Repository.HeadCommit, result.HeadCommit, headBefore)
		}
		if result.BeforeIndexSHA256 != indexBefore {
			t.Fatalf("before index identity %q != bound %q", result.BeforeIndexSHA256, indexBefore)
		}
		if result.AfterIndexSHA256 == result.BeforeIndexSHA256 {
			t.Fatal("stage did not change index identity")
		}
		if result.AfterIndexSHA256 != result.Repository.IndexSHA256 {
			t.Fatalf("returned identity %q != fresh snapshot %q", result.AfterIndexSHA256, result.Repository.IndexSHA256)
		}
		if result.StagedMode != modeBefore {
			t.Fatalf("staged mode %q != index mode %q", result.StagedMode, modeBefore)
		}
		if !validObjectID(result.StagedObjectID) {
			t.Fatalf("staged object id %q is not an object id", result.StagedObjectID)
		}
		requireChangePath(t, "fresh staged", result.Repository.Staged, "alpha.txt", "modified")
		requireNoChange(t, "fresh unstaged", result.Repository.Unstaged)
	})

	t.Run("unstage", func(t *testing.T) {
		worktreeBefore := f.worktreeBytes(t, "alpha.txt")
		rr := postGitControl(t, srv, unstageEndpoint, f.unstageRequest(t, "alpha.txt"))
		requireGitControlOK(t, rr)
		requireJSONField(t, rr, "repository")
		result := decodeGitControlResult[UnstageFileResult](t, rr)

		if result.Repository.HeadCommit != headBefore || result.HeadCommit != headBefore {
			t.Fatalf("unstage moved HEAD: snapshot=%q result=%q want %q", result.Repository.HeadCommit, result.HeadCommit, headBefore)
		}
		if result.BeforeIndexSHA256 == result.AfterIndexSHA256 {
			t.Fatal("unstage did not change index identity")
		}
		if result.AfterIndexSHA256 != result.Repository.IndexSHA256 {
			t.Fatalf("returned identity %q != fresh snapshot %q", result.AfterIndexSHA256, result.Repository.IndexSHA256)
		}
		if result.RestoredMode != modeBefore || !validObjectID(result.RestoredObjectID) {
			t.Fatalf("restored index entry %q/%q does not match the HEAD entry %q", result.RestoredMode, result.RestoredObjectID, modeBefore)
		}
		requireNoChange(t, "fresh staged", result.Repository.Staged)
		requireChangePath(t, "fresh unstaged", result.Repository.Unstaged, "alpha.txt", "modified")
		if got := f.worktreeBytes(t, "alpha.txt"); !bytes.Equal(got, worktreeBefore) {
			t.Fatalf("unstage changed worktree bytes: %q -> %q", worktreeBefore, got)
		}
	})

	if _, err := f.repo.StageFile(context.Background(), f.stageRequest(t, "alpha.txt")); err != nil {
		t.Fatal(err)
	}
	message := "phase four typed commit\n"
	t.Run("commit", func(t *testing.T) {
		rr := postGitControl(t, srv, commitEndpoint, f.commitRequest(t, message))
		requireGitControlOK(t, rr)
		requireJSONField(t, rr, "repository")
		result := decodeGitControlResult[CommitStagedResult](t, rr)

		if result.Branch != "main" || result.BranchRef != "refs/heads/main" {
			t.Fatalf("unexpected branch identity %q/%q", result.Branch, result.BranchRef)
		}
		if result.ExpectedHeadCommit != headBefore || result.ParentCommit != headBefore {
			t.Fatalf("commit parent %q/%q is not the bound HEAD %q", result.ExpectedHeadCommit, result.ParentCommit, headBefore)
		}
		if result.NewHeadCommit == headBefore || result.NewHeadCommit != result.Repository.HeadCommit {
			t.Fatalf("commit did not advance HEAD: new=%q snapshot=%q old=%q", result.NewHeadCommit, result.Repository.HeadCommit, headBefore)
		}
		if !validObjectID(result.NewHeadCommit) || !validObjectID(result.TreeID) {
			t.Fatalf("commit tree %q / new head %q is not an object id", result.TreeID, result.NewHeadCommit)
		}
		if result.BeforeIndexSHA256 == result.AfterIndexSHA256 {
			t.Fatal("commit did not change index identity")
		}
		if result.AfterIndexSHA256 != result.Repository.IndexSHA256 {
			t.Fatalf("returned identity %q != fresh snapshot %q", result.AfterIndexSHA256, result.Repository.IndexSHA256)
		}
		if result.CommitMessageSHA256 != contentSHA256([]byte(message)) {
			t.Fatalf("commit message digest %q is not the digest of the supplied message", result.CommitMessageSHA256)
		}
		if len(result.StagedPaths) != 1 || result.StagedPaths[0] != "alpha.txt" {
			t.Fatalf("staged paths = %v, want [alpha.txt]", result.StagedPaths)
		}
		requireNoChange(t, "fresh staged", result.Repository.Staged)
		if !result.Repository.Clean {
			t.Fatalf("repository is not clean after committing the staged modification: %+v", result.Repository)
		}
		if got := f.head(t); got != result.NewHeadCommit {
			t.Fatalf("branch ref is at %q, response claims %q", got, result.NewHeadCommit)
		}
		if got := f.commitMessage(t, result.NewHeadCommit); got != message {
			t.Fatalf("stored message = %q, want %q", got, message)
		}
		if got := len(f.snapshot(t).History); got != historyBefore+1 {
			t.Fatalf("history grew from %d to %d entries, want exactly one new commit", historyBefore, got)
		}
	})
}

func TestPhase4EndpointsMapEveryTypedErrorCodeThroughRealRepositoryState(t *testing.T) {
	cases := []struct {
		name     string
		endpoint string
		status   int
		code     string
		prepare  func(t *testing.T) (*phase4Fixture, any)
		after    func(t *testing.T, f *phase4Fixture)
	}{
		{
			name:     "INVALID_REQUEST",
			endpoint: stageEndpoint,
			status:   http.StatusBadRequest,
			code:     "INVALID_REQUEST",
			prepare: func(t *testing.T) (*phase4Fixture, any) {
				f := newPhase4Fixture(t)
				request := stageAlphaFixture(t, f, "alpha traversal attempt\n")
				request.Path = "../outside.txt"
				return f, request
			},
		},
		{
			name:     "COMMIT_MESSAGE_INVALID",
			endpoint: commitEndpoint,
			status:   http.StatusBadRequest,
			code:     "COMMIT_MESSAGE_INVALID",
			prepare: func(t *testing.T) (*phase4Fixture, any) {
				f := newPhase4Fixture(t)
				if _, err := f.repo.StageFile(context.Background(), stageAlphaFixture(t, f, "alpha staged empty message\n")); err != nil {
					t.Fatal(err)
				}
				request := f.commitRequest(t, "unused\n")
				request.CommitMessage = "   \n\t\n"
				return f, request
			},
			after: func(t *testing.T, f *phase4Fixture) {
				if got := f.head(t); got != f.snapshot(t).HeadCommit {
					t.Fatalf("invalid message advanced HEAD to %q", got)
				}
			},
		},
		{
			name:     "REPOSITORY_CONFLICT",
			endpoint: stageEndpoint,
			status:   http.StatusConflict,
			code:     "REPOSITORY_CONFLICT",
			prepare: func(t *testing.T) (*phase4Fixture, any) {
				f := newPhase4Fixture(t)
				request := stageAlphaFixture(t, f, "alpha wrong repository\n")
				request.RepositoryID = strings.Repeat("c", 64)
				return f, request
			},
		},
		{
			name:     "HEAD_CONFLICT",
			endpoint: stageEndpoint,
			status:   http.StatusConflict,
			code:     "HEAD_CONFLICT",
			prepare: func(t *testing.T) (*phase4Fixture, any) {
				f := newPhase4Fixture(t)
				request := stageAlphaFixture(t, f, "alpha wrong head\n")
				request.ExpectedHeadCommit = strings.Repeat("b", 40)
				return f, request
			},
		},
		{
			name:     "BRANCH_CONFLICT",
			endpoint: stageEndpoint,
			status:   http.StatusConflict,
			code:     "BRANCH_CONFLICT",
			prepare: func(t *testing.T) (*phase4Fixture, any) {
				f := newPhase4Fixture(t)
				request := stageAlphaFixture(t, f, "alpha wrong branch\n")
				request.ExpectedBranch = "side-branch"
				return f, request
			},
		},
		{
			name:     "INDEX_CONFLICT",
			endpoint: stageEndpoint,
			status:   http.StatusConflict,
			code:     "INDEX_CONFLICT",
			prepare: func(t *testing.T) (*phase4Fixture, any) {
				f := newPhase4Fixture(t)
				request := stageAlphaFixture(t, f, "alpha wrong index\n")
				request.ExpectedIndexSHA256 = strings.Repeat("a", 64)
				return f, request
			},
		},
		{
			name:     "WORKTREE_CONFLICT",
			endpoint: stageEndpoint,
			status:   http.StatusConflict,
			code:     "WORKTREE_CONFLICT",
			prepare: func(t *testing.T) (*phase4Fixture, any) {
				f := newPhase4Fixture(t)
				request := stageAlphaFixture(t, f, "alpha wrong worktree\n")
				request.ExpectedWorktreeSHA256 = strings.Repeat("a", 64)
				return f, request
			},
		},
		{
			name:     "REF_UPDATE_CONFLICT",
			endpoint: commitEndpoint,
			status:   http.StatusConflict,
			code:     "REF_UPDATE_CONFLICT",
			prepare: func(t *testing.T) (*phase4Fixture, any) {
				f := newPhase4Fixture(t)
				if _, err := f.repo.StageFile(context.Background(), stageAlphaFixture(t, f, "alpha staged behind a ref lock\n")); err != nil {
					t.Fatal(err)
				}
				request := f.commitRequest(t, "blocked ref update\n")
				// A held ref lock makes the update-ref compare-and-swap fail.
				lock := filepath.Join(f.dir, ".git", "refs", "heads", "main.lock")
				if err := os.WriteFile(lock, []byte(strings.Repeat("a", 40)+"\n"), 0o644); err != nil {
					t.Fatal(err)
				}
				return f, request
			},
			after: func(t *testing.T, f *phase4Fixture) {
				if got := f.head(t); got != f.snapshot(t).HeadCommit {
					t.Fatalf("blocked commit still advanced the branch to %q", got)
				}
			},
		},
		{
			name:     "UNSUPPORTED_TARGET",
			endpoint: stageEndpoint,
			status:   http.StatusUnprocessableEntity,
			code:     "UNSUPPORTED_TARGET",
			prepare: func(t *testing.T) (*phase4Fixture, any) {
				f := newPhase4Fixture(t)
				snapshot := f.snapshot(t)
				content := "brand new untracked file\n"
				f.write(t, "untracked.txt", content)
				return f, StageFileRequest{
					RepositoryID:           snapshot.RepositoryID,
					Path:                   "untracked.txt",
					ExpectedHeadCommit:     snapshot.HeadCommit,
					ExpectedBranch:         snapshot.Branch,
					ExpectedIndexSHA256:    snapshot.IndexSHA256,
					ExpectedWorktreeSHA256: contentSHA256([]byte(content)),
				}
			},
		},
		{
			name:     "UNSUPPORTED_GIT_STATE",
			endpoint: stageEndpoint,
			status:   http.StatusUnprocessableEntity,
			code:     "UNSUPPORTED_GIT_STATE",
			prepare: func(t *testing.T) (*phase4Fixture, any) {
				f := newPhase4Fixture(t)
				request := stageAlphaFixture(t, f, "alpha edited during a merge\n")
				merge := filepath.Join(f.dir, ".git", "MERGE_HEAD")
				if err := os.WriteFile(merge, []byte(strings.Repeat("a", 40)+"\n"), 0o644); err != nil {
					t.Fatal(err)
				}
				return f, request
			},
			after: func(t *testing.T, f *phase4Fixture) {
				requireNoChange(t, "staged after rejection", f.snapshot(t).Staged)
			},
		},
		{
			name:     "UNSAFE_GIT_ATTRIBUTES",
			endpoint: stageEndpoint,
			status:   http.StatusUnprocessableEntity,
			code:     "UNSAFE_GIT_ATTRIBUTES",
			prepare: func(t *testing.T) (*phase4Fixture, any) {
				f := newPhase4Fixture(t)
				f.write(t, "cleaned.dat", "data v1\n")
				gitCmd(t, f.dir, "add", "cleaned.dat")
				gitCmd(t, f.dir, "commit", "-m", "attribute target", "--no-gpg-sign")
				f.write(t, ".gitattributes", "cleaned.dat filter=tripscript\n")
				gitCmd(t, f.dir, "add", ".gitattributes")
				gitCmd(t, f.dir, "commit", "-m", "attribute rules", "--no-gpg-sign")
				f.write(t, "cleaned.dat", "data v2\n")
				return f, f.stageRequest(t, "cleaned.dat")
			},
			after: func(t *testing.T, f *phase4Fixture) {
				requireNoChange(t, "staged after rejection", f.snapshot(t).Staged)
			},
		},
		{
			name:     "NOTHING_TO_COMMIT",
			endpoint: commitEndpoint,
			status:   http.StatusUnprocessableEntity,
			code:     "NOTHING_TO_COMMIT",
			prepare: func(t *testing.T) (*phase4Fixture, any) {
				f := newPhase4Fixture(t)
				return f, f.commitRequest(t, "nothing staged\n")
			},
			after: func(t *testing.T, f *phase4Fixture) {
				if got := f.head(t); got != f.snapshot(t).HeadCommit {
					t.Fatalf("empty commit advanced HEAD to %q", got)
				}
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			f, request := tc.prepare(t)
			rr := postGitControl(t, phase4Server(t, f.repo), tc.endpoint, request)
			requireGitControlError(t, rr, tc.status, tc.code)
			if tc.after != nil {
				tc.after(t, f)
			}
		})
	}
}

func TestPhase4EndpointsRejectMalformedUnknownAndOversizedBodies(t *testing.T) {
	f := newPhase4Fixture(t)
	var logs bytes.Buffer
	srv := &Server{Store: newTestStore(t), Repository: f.repo, Log: log.New(&logs, "", 0)}

	for _, endpoint := range []string{stageEndpoint, unstageEndpoint, commitEndpoint} {
		t.Run("malformed "+endpoint, func(t *testing.T) {
			rr := postGitControlBody(t, srv, endpoint, []byte("{"))
			requireGitControlError(t, rr, http.StatusBadRequest, "INVALID_REQUEST")
			if !strings.Contains(logs.String(), "status=400 code=INVALID_REQUEST") {
				t.Fatalf("malformed request was not logged: %q", logs.String())
			}
		})
	}

	for _, endpoint := range []string{stageEndpoint, unstageEndpoint, commitEndpoint} {
		t.Run("unknown field "+endpoint, func(t *testing.T) {
			body := []byte(`{"repository_id":"deadbeef","not_a_phase4_field":"x"}`)
			rr := postGitControlBody(t, srv, endpoint, body)
			requireGitControlError(t, rr, http.StatusBadRequest, "INVALID_REQUEST")
			if !strings.Contains(rr.Body.String(), "not_a_phase4_field") {
				t.Fatalf("rejection does not name the unknown field: %s", rr.Body.String())
			}
		})
	}

	t.Run("oversized body", func(t *testing.T) {
		// The oversized field is otherwise well formed, so only the size gate
		// can produce this rejection.
		body, err := json.Marshal(map[string]string{
			"repository_id":        strings.Repeat("a", 3<<20),
			"expected_head_commit": strings.Repeat("b", 40),
		})
		if err != nil {
			t.Fatal(err)
		}
		rr := postGitControlBody(t, srv, stageEndpoint, body)
		envelope := requireGitControlError(t, rr, http.StatusBadRequest, "INVALID_REQUEST")
		if !strings.Contains(envelope.Error, "http: request body too large") {
			t.Fatalf("rejection is not attributable to the size gate: %q", envelope.Error)
		}
	})
}

func TestPhase4EndpointsRequireAConfiguredGitCommitController(t *testing.T) {
	f := newPhase4Fixture(t)
	srv := phase4Server(t, inspectOnlyRepository{snapshot: f.snapshot(t)})

	for _, endpoint := range []string{stageEndpoint, unstageEndpoint, commitEndpoint} {
		t.Run(endpoint, func(t *testing.T) {
			rr := postGitControl(t, srv, endpoint, StageFileRequest{Path: "alpha.txt"})
			requireGitControlError(t, rr, http.StatusNotFound, "WRITE_NOT_CONFIGURED")
		})
	}
}

func TestPhase4EndpointsLogIdentitiesAndNeverUserContent(t *testing.T) {
	const content = "top secret staging payload 8f3a\n"
	const message = "top secret commit message 4b21\n"

	f := newPhase4Fixture(t)
	var logs bytes.Buffer
	srv := &Server{Store: newTestStore(t), Repository: f.repo, Log: log.New(&logs, "", 0)}

	stageRequest := stageAlphaFixture(t, f, content)
	rr := postGitControl(t, srv, stageEndpoint, stageRequest)
	requireGitControlOK(t, rr)
	staged := decodeGitControlResult[StageFileResult](t, rr)

	if !strings.Contains(logs.String(), `POST /api/stage-file status=200 code=OK path="alpha.txt"`) {
		t.Fatalf("missing structured stage log line: %q", logs.String())
	}
	for _, want := range []string{
		"before_index_sha256=" + stageRequest.ExpectedIndexSHA256,
		"after_index_sha256=" + staged.AfterIndexSHA256,
		"expected_head_commit=" + stageRequest.ExpectedHeadCommit,
		"new_head_commit=" + staged.HeadCommit,
	} {
		if !strings.Contains(logs.String(), want) {
			t.Fatalf("stage log is missing %q: %q", want, logs.String())
		}
	}
	if strings.Contains(logs.String(), content) {
		t.Fatalf("stage log leaked written file content: %q", logs.String())
	}

	// A rejected request must be logged with the same identity fields.
	logs.Reset()
	conflicting := f.stageRequest(t, "alpha.txt")
	conflicting.ExpectedIndexSHA256 = strings.Repeat("a", 64)
	rr = postGitControl(t, srv, stageEndpoint, conflicting)
	requireGitControlError(t, rr, http.StatusConflict, "INDEX_CONFLICT")
	if !strings.Contains(logs.String(), `POST /api/stage-file status=409 code=INDEX_CONFLICT path="alpha.txt"`) {
		t.Fatalf("missing structured conflict log line: %q", logs.String())
	}
	if !strings.Contains(logs.String(), "expected_head_commit="+conflicting.ExpectedHeadCommit) {
		t.Fatalf("conflict log is missing the bound HEAD: %q", logs.String())
	}

	// A stage over an already-staged path cannot succeed, so unstage first and
	// stage again to leave exactly one staged modification for the commit.
	rr = postGitControl(t, srv, unstageEndpoint, f.unstageRequest(t, "alpha.txt"))
	requireGitControlOK(t, rr)
	rr = postGitControl(t, srv, stageEndpoint, f.stageRequest(t, "alpha.txt"))
	requireGitControlOK(t, rr)

	logs.Reset()
	rr = postGitControl(t, srv, commitEndpoint, f.commitRequest(t, message))
	requireGitControlOK(t, rr)
	committed := decodeGitControlResult[CommitStagedResult](t, rr)

	if !strings.Contains(logs.String(), "POST /api/commit status=200 code=OK") {
		t.Fatalf("missing structured commit log line: %q", logs.String())
	}
	for _, want := range []string{
		"before_index_sha256=" + committed.BeforeIndexSHA256,
		"after_index_sha256=" + committed.AfterIndexSHA256,
		"expected_head_commit=" + committed.ExpectedHeadCommit,
		"new_head_commit=" + committed.NewHeadCommit,
	} {
		if !strings.Contains(logs.String(), want) {
			t.Fatalf("commit log is missing %q: %q", want, logs.String())
		}
	}
	for _, secret := range []string{content, message, strings.TrimSpace(message)} {
		if strings.Contains(logs.String(), secret) {
			t.Fatalf("commit log leaked user content %q: %q", secret, logs.String())
		}
	}
}
