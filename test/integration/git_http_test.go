package integration

import (
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
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

	apiServer := fakeAPIServer(t, authRule{
		token:    "write-token",
		allowed:  true,
		repo:     repo,
		username: "luis",
	})
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
	runGit(t, work, "remote", "add", "origin", basicAuthURL(gitServer.URL, "luis", "write-token")+"/luis/my-repo.git")
	runGit(t, work, "push", "origin", "main")

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

	cloneDir := filepath.Join(t.TempDir(), "clone")
	runGit(t, "", "clone", basicAuthURL(gitServer.URL, "luis", "write-token")+"/luis/my-repo.git", cloneDir)

	originalHead := gitOutput(t, work, "rev-parse", "HEAD")
	clonedHead := gitOutput(t, cloneDir, "rev-parse", "HEAD")
	if originalHead != clonedHead {
		t.Fatalf("cloned HEAD = %q, want %q", clonedHead, originalHead)
	}
}

func TestGitHTTPRejectsMissingAuth(t *testing.T) {
	apiServer := fakeAPIServer(t, authRule{allowed: true})
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
	})
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
	runGit(t, work, "remote", "add", "origin", basicAuthURL(gitServer.URL, "luis", "read-token")+"/luis/my-repo.git")

	cmd := gitCmd(work, "push", "origin", "main")
	if err := cmd.Run(); err == nil {
		t.Fatal("git push succeeded, want failure")
	}
}

func TestGitHTTPCloneBeforeFirstPushReturnsNotFound(t *testing.T) {
	apiServer := fakeAPIServer(t, authRule{
		token:    "write-token",
		allowed:  true,
		repo:     api.Repository{ID: "repo-1", OwnerUserID: "owner-1", Owner: "luis", Slug: "my-repo"},
		username: "luis",
	})
	defer apiServer.Close()

	gitServer := gitHTTPServer(apiServer.URL, t.TempDir())
	defer gitServer.Close()

	res, err := http.Get(basicAuthURL(gitServer.URL, "luis", "write-token") + "/luis/my-repo.git/info/refs?service=git-upload-pack")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", res.StatusCode, http.StatusNotFound)
	}
}

func fakeAPIServer(t *testing.T, rule authRule) *httptest.Server {
	t.Helper()

	return newLocalServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

func gitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()

	cmd := gitCmd(dir, args...)
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v failed: %v", args, err)
	}
	return string(output)
}

func gitCmd(dir string, args ...string) *exec.Cmd {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	return cmd
}

func basicAuthURL(baseURL, username, password string) string {
	return "http://" + username + ":" + password + "@" + baseURL[len("http://"):]
}
