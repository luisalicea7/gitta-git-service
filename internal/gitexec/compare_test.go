package gitexec

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompareCommits(t *testing.T) {
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
	runGit(t, ctx, worktree, "branch", "feature")

	if err := os.WriteFile(filepath.Join(worktree, "README.md"), []byte("# hello\n\nmain only\n"), 0o600); err != nil {
		t.Fatalf("update main README: %v", err)
	}
	runGit(t, ctx, worktree, "add", "README.md")
	runGit(t, ctx, worktree, "commit", "-m", "main update")

	runGit(t, ctx, worktree, "checkout", "feature")
	if err := os.MkdirAll(filepath.Join(worktree, "src"), 0o700); err != nil {
		t.Fatalf("mkdir src: %v", err)
	}
	if err := os.WriteFile(filepath.Join(worktree, "src", "feature.go"), []byte("package feature\n"), 0o600); err != nil {
		t.Fatalf("write feature.go: %v", err)
	}
	runGit(t, ctx, worktree, "add", ".")
	runGit(t, ctx, worktree, "commit", "-m", "feature work")

	runGit(t, ctx, "", "init", "--bare", bareRepo)
	runGit(t, ctx, worktree, "push", bareRepo, "main:refs/heads/main", "feature:refs/heads/feature")

	compare, err := CompareCommits(ctx, bareRepo, "refs/heads/main", "refs/heads/feature")
	if err != nil {
		t.Fatalf("CompareCommits() error = %v", err)
	}
	if compare.BaseSHA == "" || compare.HeadSHA == "" || compare.MergeBaseSHA == "" {
		t.Fatalf("missing resolved shas: %#v", compare)
	}
	if len(compare.Commits) != 1 || compare.Commits[0].Subject != "feature work" {
		t.Fatalf("commits = %#v, want feature work only", compare.Commits)
	}
	if compare.Stats.FilesChanged != 1 || compare.Stats.Additions != 1 {
		t.Fatalf("stats = %#v, want one added file", compare.Stats)
	}
	if len(compare.Files) != 1 {
		t.Fatalf("len(files) = %d, want 1", len(compare.Files))
	}
	file := compare.Files[0]
	if file.Path != "src/feature.go" || file.Status != "added" {
		t.Fatalf("file = %#v, want added src/feature.go", file)
	}
	if file.Patch == nil || len(file.Patch.Hunks) == 0 {
		t.Fatalf("missing patch: %#v", file)
	}
	if !strings.Contains(file.Patch.Hunks[0].Lines[0].Content, "package feature") {
		t.Fatalf("patch lines = %#v", file.Patch.Hunks[0].Lines)
	}
}
