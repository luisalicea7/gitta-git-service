package gitexec

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseNameStatus(t *testing.T) {
	t.Parallel()

	files := ParseNameStatus("A\tREADME.md\nM\tsrc/main.go\nD\told.txt\nR100\tbefore.txt\tafter.txt\n")
	if len(files) != 4 {
		t.Fatalf("len(files) = %d, want 4", len(files))
	}
	if files[0].Status != "added" || files[0].Path != "README.md" {
		t.Fatalf("added file = %#v", files[0])
	}
	if files[3].Status != "renamed" || files[3].OldPath == nil || *files[3].OldPath != "before.txt" || files[3].Path != "after.txt" {
		t.Fatalf("renamed file = %#v", files[3])
	}
}

func TestApplyNumstat(t *testing.T) {
	t.Parallel()

	files := []ChangedFile{{Path: "README.md"}, {Path: "image.png"}}
	ApplyNumstat(files, "2\t1\tREADME.md\n-\t-\timage.png\n")

	if files[0].Additions != 2 || files[0].Deletions != 1 || files[0].IsBinary {
		t.Fatalf("README stats = %#v", files[0])
	}
	if !files[1].IsBinary || files[1].Additions != 0 || files[1].Deletions != 0 {
		t.Fatalf("binary stats = %#v", files[1])
	}
}

func TestParsePatch(t *testing.T) {
	t.Parallel()

	output := `diff --git a/README.md b/README.md
index abc..def 100644
--- a/README.md
+++ b/README.md
@@ -1,2 +1,2 @@
 hello
-old
+new
`
	patches := ParsePatch(output)
	patch, ok := patches["README.md"]
	if !ok {
		t.Fatalf("README patch missing: %#v", patches)
	}
	if len(patch.Hunks) != 1 {
		t.Fatalf("len(hunks) = %d, want 1", len(patch.Hunks))
	}
	lines := patch.Hunks[0].Lines
	if len(lines) != 3 {
		t.Fatalf("len(lines) = %d, want 3", len(lines))
	}
	if lines[1].Type != "delete" || lines[1].OldLineNumber == nil || *lines[1].OldLineNumber != 2 {
		t.Fatalf("delete line = %#v", lines[1])
	}
	if lines[2].Type != "add" || lines[2].NewLineNumber == nil || *lines[2].NewLineNumber != 2 {
		t.Fatalf("add line = %#v", lines[2])
	}
}

func TestReadCommitDetail(t *testing.T) {
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
	if err := os.MkdirAll(filepath.Join(worktree, "src"), 0o700); err != nil {
		t.Fatalf("mkdir src: %v", err)
	}
	if err := os.WriteFile(filepath.Join(worktree, "README.md"), []byte("# hello\n\nupdated\n"), 0o600); err != nil {
		t.Fatalf("update README: %v", err)
	}
	if err := os.WriteFile(filepath.Join(worktree, "src", "main.go"), []byte("package main\n"), 0o600); err != nil {
		t.Fatalf("write main.go: %v", err)
	}
	runGit(t, ctx, worktree, "add", ".")
	runGit(t, ctx, worktree, "commit", "-m", "update readme")
	runGit(t, ctx, "", "init", "--bare", bareRepo)
	runGit(t, ctx, worktree, "push", bareRepo, "HEAD:refs/heads/main")

	sha := strings.TrimSpace(string(runGitOutput(t, ctx, worktree, "rev-parse", "HEAD")))
	detail, err := ReadCommitDetail(ctx, bareRepo, sha)
	if err != nil {
		t.Fatalf("ReadCommitDetail() error = %v", err)
	}
	if detail.Commit.Subject != "update readme" {
		t.Fatalf("Subject = %q, want update readme", detail.Commit.Subject)
	}
	if detail.Stats.FilesChanged != 2 {
		t.Fatalf("FilesChanged = %d, want 2", detail.Stats.FilesChanged)
	}
	if len(detail.Files) != 2 {
		t.Fatalf("len(files) = %d, want 2", len(detail.Files))
	}
	if detail.Files[0].Patch == nil && detail.Files[1].Patch == nil {
		t.Fatalf("expected at least one patch: %#v", detail.Files)
	}
}

func runGitOutput(t *testing.T, ctx context.Context, dir string, args ...string) []byte {
	t.Helper()

	cmd := exec.CommandContext(ctx, "git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, string(output))
	}
	return output
}
