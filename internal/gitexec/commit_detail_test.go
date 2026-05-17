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

	files := []ChangedFile{{Path: "README.md"}, {Path: "image.png"}, {Path: "src/new.go"}}
	ApplyNumstat(files, "2\t1\tREADME.md\n-\t-\timage.png\n3\t2\tsrc/{old.go => new.go}\n")

	if files[0].Additions != 2 || files[0].Deletions != 1 || files[0].IsBinary {
		t.Fatalf("README stats = %#v", files[0])
	}
	if !files[1].IsBinary || files[1].Additions != 0 || files[1].Deletions != 0 {
		t.Fatalf("binary stats = %#v", files[1])
	}
	if files[2].Additions != 3 || files[2].Deletions != 2 {
		t.Fatalf("rename stats = %#v", files[2])
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

func TestParsePatchDeletedFile(t *testing.T) {
	t.Parallel()

	output := `diff --git a/old.txt b/old.txt
deleted file mode 100644
index abc..0000000
--- a/old.txt
+++ /dev/null
@@ -1,2 +0,0 @@
-old
-
`
	patches := ParsePatch(output)
	patch, ok := patches["old.txt"]
	if !ok {
		t.Fatalf("old.txt patch missing: %#v", patches)
	}
	if len(patch.Hunks) != 1 {
		t.Fatalf("len(hunks) = %d, want 1", len(patch.Hunks))
	}
	lines := patch.Hunks[0].Lines
	if len(lines) != 2 {
		t.Fatalf("len(lines) = %d, want 2", len(lines))
	}
	if lines[1].Type != "delete" || lines[1].Content != "" {
		t.Fatalf("empty delete line = %#v", lines[1])
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

func TestReadCommitDetailRootCommit(t *testing.T) {
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

	sha := strings.TrimSpace(string(runGitOutput(t, ctx, worktree, "rev-parse", "HEAD")))
	detail, err := ReadCommitDetail(ctx, bareRepo, sha)
	if err != nil {
		t.Fatalf("ReadCommitDetail() error = %v", err)
	}
	if len(detail.Commit.Parents) != 0 {
		t.Fatalf("Parents = %#v, want empty", detail.Commit.Parents)
	}
	if detail.Stats.FilesChanged != 1 || detail.Files[0].Status != "added" {
		t.Fatalf("root detail = %#v", detail)
	}
	if detail.Files[0].Patch == nil || len(detail.Files[0].Patch.Hunks) == 0 {
		t.Fatalf("root patch missing: %#v", detail.Files[0])
	}
}

func TestReadCommitDetailRenameDeleteAndBinary(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	worktree := filepath.Join(t.TempDir(), "worktree")
	bareRepo := filepath.Join(t.TempDir(), "repo.git")

	runGit(t, ctx, "", "init", worktree)
	runGit(t, ctx, worktree, "config", "user.email", "test@example.com")
	runGit(t, ctx, worktree, "config", "user.name", "Test User")
	if err := os.WriteFile(filepath.Join(worktree, "old.txt"), []byte("old\n"), 0o600); err != nil {
		t.Fatalf("write old.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(worktree, "delete-me.txt"), []byte("gone\n"), 0o600); err != nil {
		t.Fatalf("write delete-me.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(worktree, "image.bin"), []byte{0, 1, 2, 3}, 0o600); err != nil {
		t.Fatalf("write image.bin: %v", err)
	}
	runGit(t, ctx, worktree, "add", ".")
	runGit(t, ctx, worktree, "commit", "-m", "initial")
	if err := os.Rename(filepath.Join(worktree, "old.txt"), filepath.Join(worktree, "new.txt")); err != nil {
		t.Fatalf("rename old.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(worktree, "new.txt"), []byte("old\nnew\n"), 0o600); err != nil {
		t.Fatalf("update new.txt: %v", err)
	}
	if err := os.Remove(filepath.Join(worktree, "delete-me.txt")); err != nil {
		t.Fatalf("remove delete-me.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(worktree, "image.bin"), []byte{0, 1, 2, 3, 4, 5}, 0o600); err != nil {
		t.Fatalf("update image.bin: %v", err)
	}
	runGit(t, ctx, worktree, "add", "-A")
	runGit(t, ctx, worktree, "commit", "-m", "rename delete binary")
	runGit(t, ctx, "", "init", "--bare", bareRepo)
	runGit(t, ctx, worktree, "push", bareRepo, "HEAD:refs/heads/main")

	sha := strings.TrimSpace(string(runGitOutput(t, ctx, worktree, "rev-parse", "HEAD")))
	detail, err := ReadCommitDetail(ctx, bareRepo, sha)
	if err != nil {
		t.Fatalf("ReadCommitDetail() error = %v", err)
	}

	byPath := map[string]ChangedFile{}
	for _, file := range detail.Files {
		byPath[file.Path] = file
	}
	if byPath["new.txt"].Status != "renamed" || byPath["new.txt"].OldPath == nil || *byPath["new.txt"].OldPath != "old.txt" {
		t.Fatalf("rename file = %#v", byPath["new.txt"])
	}
	if byPath["delete-me.txt"].Status != "deleted" || byPath["delete-me.txt"].Patch == nil {
		t.Fatalf("delete file = %#v", byPath["delete-me.txt"])
	}
	if !byPath["image.bin"].IsBinary || byPath["image.bin"].Patch != nil {
		t.Fatalf("binary file = %#v", byPath["image.bin"])
	}
}

func TestReadCommitDetailMergeCommit(t *testing.T) {
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
	runGit(t, ctx, worktree, "checkout", "-b", "feature")
	if err := os.WriteFile(filepath.Join(worktree, "feature.txt"), []byte("feature\n"), 0o600); err != nil {
		t.Fatalf("write feature: %v", err)
	}
	runGit(t, ctx, worktree, "add", "feature.txt")
	runGit(t, ctx, worktree, "commit", "-m", "feature work")
	runGit(t, ctx, worktree, "checkout", "main")
	runGit(t, ctx, worktree, "merge", "--no-ff", "--no-edit", "feature")
	runGit(t, ctx, "", "init", "--bare", bareRepo)
	runGit(t, ctx, worktree, "push", bareRepo, "HEAD:refs/heads/main")

	sha := strings.TrimSpace(string(runGitOutput(t, ctx, worktree, "rev-parse", "HEAD")))
	detail, err := ReadCommitDetail(ctx, bareRepo, sha)
	if err != nil {
		t.Fatalf("ReadCommitDetail() error = %v", err)
	}
	if len(detail.Commit.Parents) != 2 {
		t.Fatalf("Parents = %#v, want two parents", detail.Commit.Parents)
	}
	if detail.Stats.FilesChanged != 1 || len(detail.Files) != 1 {
		t.Fatalf("merge detail = %#v, want one changed file", detail)
	}
	if detail.Files[0].Path != "feature.txt" || detail.Files[0].Status != "added" {
		t.Fatalf("merge file = %#v, want added feature.txt", detail.Files[0])
	}
	if detail.Files[0].Patch == nil || len(detail.Files[0].Patch.Hunks) == 0 {
		t.Fatalf("merge patch missing: %#v", detail.Files[0])
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
