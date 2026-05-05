package gitexec

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestParseCommitLog(t *testing.T) {
	t.Parallel()

	output := "abc123\x00parent1 parent2\x00Ada Lovelace\x00ada@example.com\x002026-05-04T12:00:00Z\x00initial commit\x00body line\n\x1e"

	commits := ParseCommitLog(output)
	if len(commits) != 1 {
		t.Fatalf("len(commits) = %d, want 1", len(commits))
	}

	commit := commits[0]
	if commit.SHA != "abc123" {
		t.Fatalf("SHA = %q, want abc123", commit.SHA)
	}
	if len(commit.Parents) != 2 || commit.Parents[0] != "parent1" || commit.Parents[1] != "parent2" {
		t.Fatalf("Parents = %#v, want parent1,parent2", commit.Parents)
	}
	if commit.AuthorName != "Ada Lovelace" || commit.AuthorEmail != "ada@example.com" {
		t.Fatalf("author = %q <%q>", commit.AuthorName, commit.AuthorEmail)
	}
	if commit.Body != "body line" {
		t.Fatalf("Body = %q, want body line", commit.Body)
	}
}

func TestParseCommitLogRootCommit(t *testing.T) {
	t.Parallel()

	output := "abc123\x00\x00Ada Lovelace\x00ada@example.com\x002026-05-04T12:00:00Z\x00initial commit\x00\x1e"

	commits := ParseCommitLog(output)
	if len(commits) != 1 {
		t.Fatalf("len(commits) = %d, want 1", len(commits))
	}
	if len(commits[0].Parents) != 0 {
		t.Fatalf("Parents = %#v, want empty", commits[0].Parents)
	}
}

func TestListCommits(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	worktree := filepath.Join(t.TempDir(), "worktree")
	bareRepo := filepath.Join(t.TempDir(), "repo.git")

	runGit(t, ctx, "", "init", worktree)
	runGit(t, ctx, worktree, "config", "user.email", "test@example.com")
	runGit(t, ctx, worktree, "config", "user.name", "Test User")
	if err := os.WriteFile(filepath.Join(worktree, "README.md"), []byte("# hello\n"), 0o600); err != nil {
		t.Fatalf("write README: %v", err)
	}
	runGit(t, ctx, worktree, "add", "README.md")
	runGit(t, ctx, worktree, "commit", "-m", "initial")
	runGit(t, ctx, "", "init", "--bare", bareRepo)
	runGit(t, ctx, worktree, "push", bareRepo, "HEAD:refs/heads/main")

	commits, err := ListCommits(ctx, bareRepo, "refs/heads/main", 10)
	if err != nil {
		t.Fatalf("ListCommits() error = %v", err)
	}
	if len(commits) != 1 {
		t.Fatalf("len(commits) = %d, want 1", len(commits))
	}
	if commits[0].Subject != "initial" {
		t.Fatalf("Subject = %q, want initial", commits[0].Subject)
	}
	if commits[0].AuthorEmail != "test@example.com" {
		t.Fatalf("AuthorEmail = %q, want test@example.com", commits[0].AuthorEmail)
	}
}

func TestListCommitsForPath(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	worktree := filepath.Join(t.TempDir(), "worktree")
	bareRepo := filepath.Join(t.TempDir(), "repo.git")

	runGit(t, ctx, "", "init", worktree)
	runGit(t, ctx, worktree, "config", "user.email", "test@example.com")
	runGit(t, ctx, worktree, "config", "user.name", "Test User")
	if err := os.WriteFile(filepath.Join(worktree, "README.md"), []byte("# hello\n"), 0o600); err != nil {
		t.Fatalf("write README: %v", err)
	}
	runGit(t, ctx, worktree, "add", "README.md")
	runGit(t, ctx, worktree, "commit", "-m", "initial readme")
	if err := os.MkdirAll(filepath.Join(worktree, "src"), 0o700); err != nil {
		t.Fatalf("mkdir src: %v", err)
	}
	if err := os.WriteFile(filepath.Join(worktree, "src", "main.go"), []byte("package main\n"), 0o600); err != nil {
		t.Fatalf("write main.go: %v", err)
	}
	runGit(t, ctx, worktree, "add", "src/main.go")
	runGit(t, ctx, worktree, "commit", "-m", "add main")
	if err := os.WriteFile(filepath.Join(worktree, "README.md"), []byte("# hello\nupdated\n"), 0o600); err != nil {
		t.Fatalf("update README: %v", err)
	}
	runGit(t, ctx, worktree, "add", "README.md")
	runGit(t, ctx, worktree, "commit", "-m", "update readme")
	runGit(t, ctx, "", "init", "--bare", bareRepo)
	runGit(t, ctx, worktree, "push", bareRepo, "HEAD:refs/heads/main")

	commits, err := ListCommitsForPath(ctx, bareRepo, "refs/heads/main", 10, "src/main.go")
	if err != nil {
		t.Fatalf("ListCommitsForPath() error = %v", err)
	}
	if len(commits) != 1 {
		t.Fatalf("len(commits) = %d, want 1", len(commits))
	}
	if commits[0].Subject != "add main" {
		t.Fatalf("Subject = %q, want add main", commits[0].Subject)
	}
}

func runGit(t *testing.T, ctx context.Context, dir string, args ...string) {
	t.Helper()

	cmd := exec.CommandContext(ctx, "git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, string(output))
	}
}
