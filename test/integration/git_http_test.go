package integration

import (
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/luisalicea7/gitta-git-service/internal/api"
	"github.com/luisalicea7/gitta-git-service/internal/httpgit"
	"github.com/luisalicea7/gitta-git-service/internal/repos"
)

const internalSecret = "test-secret"

type authRule struct {
	token    string
	allowed  bool
	reason   string
	repo     api.Repository
	username string
}

type fakeAPIState struct {
	mu              sync.Mutex
	postReceive     []api.PostReceiveRequest
	preReceiveDeny  bool
	preReceiveMsg   string
	preReceiveCalls int
}

func TestGitHTTPPushAndClone(t *testing.T) {
	repoRoot := t.TempDir()
	repo := api.Repository{
		ID:            "repo-1",
		OwnerUserID:   "owner-1",
		Owner:         "luis",
		Name:          "My Repo",
		Slug:          "my-repo",
		DefaultBranch: "main",
	}

	state := &fakeAPIState{}
	apiServer := fakeAPIServer(t, authRule{
		token:    "write-token",
		allowed:  true,
		repo:     repo,
		username: "luis",
	}, state)
	defer apiServer.Close()

	gitServer := gitHTTPServer(apiServer.URL, repoRoot)
	defer gitServer.Close()

	work := t.TempDir()
	runGit(t, work, "init")
	runGit(t, work, "config", "user.email", "test@example.com")
	runGit(t, work, "config", "user.name", "Test User")
	if err := os.WriteFile(filepath.Join(work, "README.md"), []byte("# Test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, work, "add", "README.md")
	runGit(t, work, "commit", "-m", "initial")
	runGit(t, work, "branch", "-M", "main")
	runGit(t, work, "remote", "add", "origin", gitServer.URL+"/luis/my-repo.git")
	runGitWithCredentials(t, work, "luis", "write-token", "push", "origin", "main")

	repoPath, err := repos.Path(repoRoot, repos.RepositoryIdentity{
		OwnerUserID: repo.OwnerUserID,
		ID:          repo.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !repos.ExistsBare(repoPath) {
		t.Fatalf("expected bare repo at %s", repoPath)
	}

	originalHead := gitOutput(t, work, "rev-parse", "HEAD")
	requests := state.postReceiveRequests()
	if len(requests) != 1 {
		t.Fatalf("post-receive requests = %d, want 1", len(requests))
	}
	if requests[0].RepositoryID != repo.ID {
		t.Fatalf("post-receive repository id = %q, want %q", requests[0].RepositoryID, repo.ID)
	}
	if len(requests[0].Refs) != 1 {
		t.Fatalf("post-receive refs = %d, want 1", len(requests[0].Refs))
	}
	if requests[0].Refs[0] != (api.GitRef{Name: "refs/heads/main", SHA: originalHead, Type: "branch"}) {
		t.Fatalf("post-receive ref = %#v, want main at %s", requests[0].Refs[0], originalHead)
	}

	cloneDir := filepath.Join(t.TempDir(), "clone")
	runGitWithCredentials(t, "", "luis", "write-token", "clone", gitServer.URL+"/luis/my-repo.git", cloneDir)

	clonedHead := gitOutput(t, cloneDir, "rev-parse", "HEAD")
	if originalHead != clonedHead {
		t.Fatalf("cloned HEAD = %q, want %q", clonedHead, originalHead)
	}
}

func TestGitHTTPRejectsMissingAuth(t *testing.T) {
	apiServer := fakeAPIServer(t, authRule{allowed: true}, nil)
	defer apiServer.Close()

	gitServer := gitHTTPServer(apiServer.URL, t.TempDir())
	defer gitServer.Close()

	res, err := http.Get(gitServer.URL + "/luis/my-repo.git/info/refs?service=git-upload-pack")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusUnauthorized)
	}
	if got := res.Header.Get("WWW-Authenticate"); got == "" {
		t.Fatal("missing WWW-Authenticate header")
	}
}

func TestGitHTTPRejectsReadOnlyPush(t *testing.T) {
	apiServer := fakeAPIServer(t, authRule{
		token:    "read-token",
		allowed:  false,
		reason:   "insufficient_scope",
		username: "luis",
	}, nil)
	defer apiServer.Close()

	gitServer := gitHTTPServer(apiServer.URL, t.TempDir())
	defer gitServer.Close()

	work := t.TempDir()
	runGit(t, work, "init")
	runGit(t, work, "config", "user.email", "test@example.com")
	runGit(t, work, "config", "user.name", "Test User")
	if err := os.WriteFile(filepath.Join(work, "README.md"), []byte("# Test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, work, "add", "README.md")
	runGit(t, work, "commit", "-m", "initial")
	runGit(t, work, "branch", "-M", "main")
	runGit(t, work, "remote", "add", "origin", gitServer.URL+"/luis/my-repo.git")

	cmd := gitCmdWithEnv(work, credentialPromptEnv(t, "luis", "read-token"), "push", "origin", "main")
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("git push succeeded, want failure")
	}
	if !strings.Contains(string(output), "token does not have repo:write scope") {
		t.Fatalf("git push output missing scope error:\n%s", string(output))
	}
}

func TestGitHTTPRejectsProtectedBranchPush(t *testing.T) {
	repoRoot := t.TempDir()
	repo := api.Repository{
		ID:            "repo-1",
		OwnerUserID:   "owner-1",
		Owner:         "luis",
		Name:          "My Repo",
		Slug:          "my-repo",
		DefaultBranch: "main",
	}

	state := &fakeAPIState{
		preReceiveDeny: true,
		preReceiveMsg:  "protected branch: main must be updated through a pull request",
	}
	apiServer := fakeAPIServer(t, authRule{
		token:    "write-token",
		allowed:  true,
		repo:     repo,
		username: "luis",
	}, state)
	defer apiServer.Close()

	gitServer := gitHTTPServer(apiServer.URL, repoRoot)
	defer gitServer.Close()

	work := t.TempDir()
	runGit(t, work, "init")
	runGit(t, work, "config", "user.email", "test@example.com")
	runGit(t, work, "config", "user.name", "Test User")
	if err := os.WriteFile(filepath.Join(work, "README.md"), []byte("# Test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, work, "add", "README.md")
	runGit(t, work, "commit", "-m", "initial")
	runGit(t, work, "branch", "-M", "main")
	runGit(t, work, "remote", "add", "origin", gitServer.URL+"/luis/my-repo.git")

	cmd := gitCmdWithEnv(work, credentialPromptEnv(t, "luis", "write-token"), "push", "origin", "main")
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("git push succeeded, want protected branch rejection")
	}
	if !strings.Contains(string(output), state.preReceiveMsg) {
		t.Fatalf("git push output missing protected branch message:\n%s", string(output))
	}
	if state.preReceiveCallCount() == 0 {
		t.Fatal("expected pre-receive API call")
	}
}

func (s *fakeAPIState) preReceiveCallCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.preReceiveCalls
}

func TestGitHTTPCloneBeforeFirstPushReturnsNotFound(t *testing.T) {
	apiServer := fakeAPIServer(t, authRule{
		token:    "write-token",
		allowed:  true,
		repo:     api.Repository{ID: "repo-1", OwnerUserID: "owner-1", Owner: "luis", Slug: "my-repo"},
		username: "luis",
	}, nil)
	defer apiServer.Close()

	gitServer := gitHTTPServer(apiServer.URL, t.TempDir())
	defer gitServer.Close()

	res, err := http.Get(basicAuthURL(gitServer.URL, "luis", "write-token") + "/luis/my-repo.git/info/refs?service=git-upload-pack")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	body := readResponseBody(t, res)
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusNotFound)
	}
	if !strings.Contains(body, "repository not found") {
		t.Fatalf("body = %q, want repository not found", body)
	}
}

func fakeAPIServer(t *testing.T, rule authRule, state *fakeAPIState) *httptest.Server {
	t.Helper()

	return newLocalServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/internal/git/post-receive" {
			handlePostReceive(w, r, state)
			return
		}
		if r.URL.Path == "/internal/git/pre-receive" {
			handlePreReceive(w, r, state)
			return
		}

		if r.URL.Path != "/internal/git/auth" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("x-gitta-internal-secret") != internalSecret {
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(api.AuthResponse{Allowed: false, Reason: "unauthorized"})
			return
		}

		var req api.AuthRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if req.Username != rule.username || req.Token != rule.token {
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(api.AuthResponse{Allowed: false, Reason: "invalid_credentials"})
			return
		}

		if !rule.allowed {
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(api.AuthResponse{Allowed: false, Reason: rule.reason})
			return
		}

		_ = json.NewEncoder(w).Encode(api.AuthResponse{
			Allowed:    true,
			UserID:     "user-1",
			Repository: rule.repo,
			Permission: "owner",
		})
	}))
}

func handlePreReceive(w http.ResponseWriter, r *http.Request, state *fakeAPIState) {
	if r.Header.Get("x-gitta-internal-secret") != internalSecret {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(api.PreReceiveResponse{Allowed: false, Reason: "unauthorized"})
		return
	}

	if state != nil {
		state.mu.Lock()
		state.preReceiveCalls++
		deny := state.preReceiveDeny
		msg := state.preReceiveMsg
		state.mu.Unlock()

		if deny {
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(api.PreReceiveResponse{
				Allowed: false,
				Reason:  "protected_branch",
				Violations: []api.PreReceiveViolation{
					{Ref: "refs/heads/main", Branch: "main", RulePattern: "main", Message: msg},
				},
			})
			return
		}
	}

	_ = json.NewEncoder(w).Encode(api.PreReceiveResponse{Allowed: true})
}

func handlePostReceive(w http.ResponseWriter, r *http.Request, state *fakeAPIState) {
	if r.Header.Get("x-gitta-internal-secret") != internalSecret {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(api.PostReceiveResponse{Status: "error", Reason: "unauthorized"})
		return
	}

	var req api.PostReceiveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if state != nil {
		state.mu.Lock()
		state.postReceive = append(state.postReceive, req)
		state.mu.Unlock()
	}

	_ = json.NewEncoder(w).Encode(api.PostReceiveResponse{Status: "ok", Synced: len(req.Refs)})
}

func (s *fakeAPIState) postReceiveRequests() []api.PostReceiveRequest {
	s.mu.Lock()
	defer s.mu.Unlock()

	requests := make([]api.PostReceiveRequest, len(s.postReceive))
	copy(requests, s.postReceive)
	return requests
}

func gitHTTPServer(apiURL string, repoRoot string) *httptest.Server {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	client := api.NewClient(apiURL, internalSecret)
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	server := &httptest.Server{
		Listener: listener,
		Config:   &http.Server{Handler: httpgit.NewHandler(client, repoRoot, logger)},
	}
	server.Start()
	return server
}

func newLocalServer(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()

	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := &httptest.Server{
		Listener: listener,
		Config:   &http.Server{Handler: handler},
	}
	server.Start()
	return server
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()

	cmd := gitCmd(dir, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, string(output))
	}
}

func runGitWithCredentials(t *testing.T, dir string, username string, password string, args ...string) {
	t.Helper()

	cmd := gitCmdWithEnv(dir, credentialPromptEnv(t, username, password), args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, string(output))
	}
}

func gitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()

	cmd := gitCmd(dir, args...)
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v failed: %v", args, err)
	}
	return strings.TrimSpace(string(output))
}

func gitCmd(dir string, args ...string) *exec.Cmd {
	return gitCmdWithEnv(dir, nil, args...)
}

func gitCmdWithEnv(dir string, extraEnv []string, args ...string) *exec.Cmd {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	cmd.Env = append(cmd.Env, extraEnv...)
	return cmd
}

func basicAuthURL(baseURL, username, password string) string {
	return "http://" + username + ":" + password + "@" + baseURL[len("http://"):]
}

func credentialPromptEnv(t *testing.T, username string, password string) []string {
	t.Helper()

	scriptPath := filepath.Join(t.TempDir(), "git-askpass")
	script := `#!/bin/sh
case "$1" in
*Username*) printf '%s\n' "$GITTA_TEST_USERNAME" ;;
*Password*) printf '%s\n' "$GITTA_TEST_PASSWORD" ;;
*) printf '\n' ;;
esac
`
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	return []string{
		"GIT_ASKPASS=" + scriptPath,
		"GITTA_TEST_USERNAME=" + username,
		"GITTA_TEST_PASSWORD=" + password,
	}
}

func readResponseBody(t *testing.T, res *http.Response) string {
	t.Helper()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}

	return string(body)
}
