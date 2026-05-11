package gitexec

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMergeCommitsCreatesMergeCommitAndUpdatesTarget(t *testing.T) {
	ctx := context.Background()
	bareRepo := filepath.Join(t.TempDir(), "repo.git")
	worktree := filepath.Join(t.TempDir(), "worktree")

	runGit(t, ctx, "", "init", "--bare", bareRepo)
	runGit(t, ctx, "", "init", worktree)
	runGit(t, ctx, worktree, "config", "user.email", "test@example.com")
	runGit(t, ctx, worktree, "config", "user.name", "Test User")
	writeFile(t, worktree, "README.md", "# hello\n")
	runGit(t, ctx, worktree, "add", "README.md")
	runGit(t, ctx, worktree, "commit", "-m", "initial")
	runGit(t, ctx, worktree, "branch", "feature")

	writeFile(t, worktree, "main.txt", "main\n")
	runGit(t, ctx, worktree, "add", "main.txt")
	runGit(t, ctx, worktree, "commit", "-m", "main work")
	mainSHA := strings.TrimSpace(string(runGitOutput(t, ctx, worktree, "rev-parse", "HEAD")))

	runGit(t, ctx, worktree, "checkout", "feature")
	writeFile(t, worktree, "feature.txt", "feature\n")
	runGit(t, ctx, worktree, "add", "feature.txt")
	runGit(t, ctx, worktree, "commit", "-m", "feature work")
	featureSHA := strings.TrimSpace(string(runGitOutput(t, ctx, worktree, "rev-parse", "HEAD")))
	runGit(t, ctx, worktree, "push", bareRepo, "main:refs/heads/main", "feature:refs/heads/feature")

	result, err := MergeCommits(ctx, bareRepo, "main", mainSHA, featureSHA)
	if err != nil {
		t.Fatalf("merge commits: %v", err)
	}

	if result.Status != "merged" {
		t.Fatalf("expected merged status, got %q", result.Status)
	}
	if result.MergeSHA == "" {
		t.Fatal("expected merge sha")
	}

	updatedMain := strings.TrimSpace(string(runGitOutput(t, ctx, "", "--git-dir", bareRepo, "rev-parse", "refs/heads/main")))
	if updatedMain != result.MergeSHA {
		t.Fatalf("expected target ref to point at merge sha, got %s", updatedMain)
	}
}

func TestMergeCommitsReturnsConflictFiles(t *testing.T) {
	ctx := context.Background()
	bareRepo := filepath.Join(t.TempDir(), "repo.git")
	worktree := filepath.Join(t.TempDir(), "worktree")

	runGit(t, ctx, "", "init", "--bare", bareRepo)
	runGit(t, ctx, "", "init", worktree)
	runGit(t, ctx, worktree, "config", "user.email", "test@example.com")
	runGit(t, ctx, worktree, "config", "user.name", "Test User")
	writeFile(t, worktree, "README.md", "base\n")
	runGit(t, ctx, worktree, "add", "README.md")
	runGit(t, ctx, worktree, "commit", "-m", "initial")
	runGit(t, ctx, worktree, "branch", "feature")

	writeFile(t, worktree, "README.md", "main\n")
	runGit(t, ctx, worktree, "add", "README.md")
	runGit(t, ctx, worktree, "commit", "-m", "main work")
	mainSHA := strings.TrimSpace(string(runGitOutput(t, ctx, worktree, "rev-parse", "HEAD")))

	runGit(t, ctx, worktree, "checkout", "feature")
	writeFile(t, worktree, "README.md", "feature\n")
	runGit(t, ctx, worktree, "add", "README.md")
	runGit(t, ctx, worktree, "commit", "-m", "feature work")
	featureSHA := strings.TrimSpace(string(runGitOutput(t, ctx, worktree, "rev-parse", "HEAD")))
	runGit(t, ctx, worktree, "push", bareRepo, "main:refs/heads/main", "feature:refs/heads/feature")

	result, err := MergeCommits(ctx, bareRepo, "main", mainSHA, featureSHA)
	if err != nil {
		t.Fatalf("merge commits: %v", err)
	}

	if result.Status != "conflict" {
		t.Fatalf("expected conflict status, got %q", result.Status)
	}
	if len(result.Files) != 1 {
		t.Fatalf("expected one conflict file, got %d", len(result.Files))
	}
	if result.Files[0].Path != "README.md" || result.Files[0].Status != "both_modified" {
		t.Fatalf("unexpected conflict file: %#v", result.Files[0])
	}
}

func writeFile(t *testing.T, root string, name string, content string) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
