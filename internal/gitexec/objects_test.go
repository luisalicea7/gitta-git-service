package gitexec

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
)

func TestParseTree(t *testing.T) {
	t.Parallel()

	output := "100644 blob abc123 12\tREADME.md\x00040000 tree def456 -\tsrc\x00"

	entries := ParseTree(output, "")
	if len(entries) != 2 {
		t.Fatalf("len(entries) = %d, want 2", len(entries))
	}
	if entries[0].Name != "README.md" || entries[0].Path != "README.md" {
		t.Fatalf("first entry = %#v", entries[0])
	}
	if entries[0].Size == nil || *entries[0].Size != 12 {
		t.Fatalf("first size = %#v, want 12", entries[0].Size)
	}
	if entries[1].Type != "tree" || entries[1].Size != nil {
		t.Fatalf("second entry = %#v", entries[1])
	}
}

func TestListTreeAndReadBlob(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	worktree := filepath.Join(t.TempDir(), "worktree")
	bareRepo := filepath.Join(t.TempDir(), "repo.git")

	runGit(t, ctx, "", "init", worktree)
	runGit(t, ctx, worktree, "config", "user.email", "test@example.com")
	runGit(t, ctx, worktree, "config", "user.name", "Test User")
	if err := os.MkdirAll(filepath.Join(worktree, "src"), 0o700); err != nil {
		t.Fatalf("mkdir src: %v", err)
	}
	if err := os.WriteFile(filepath.Join(worktree, "README.md"), []byte("# hello\n"), 0o600); err != nil {
		t.Fatalf("write README: %v", err)
	}
	if err := os.WriteFile(filepath.Join(worktree, "src", "main.go"), []byte("package main\n"), 0o600); err != nil {
		t.Fatalf("write main.go: %v", err)
	}
	runGit(t, ctx, worktree, "add", ".")
	runGit(t, ctx, worktree, "commit", "-m", "initial")
	runGit(t, ctx, "", "init", "--bare", bareRepo)
	runGit(t, ctx, worktree, "push", bareRepo, "HEAD:refs/heads/main")

	entries, err := ListTree(ctx, bareRepo, "refs/heads/main", "")
	if err != nil {
		t.Fatalf("ListTree() error = %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("len(entries) = %d, want 2", len(entries))
	}

	srcEntries, err := ListTree(ctx, bareRepo, "refs/heads/main", "src")
	if err != nil {
		t.Fatalf("ListTree(src) error = %v", err)
	}
	if len(srcEntries) != 1 || srcEntries[0].Path != "src/main.go" {
		t.Fatalf("src entries = %#v", srcEntries)
	}

	blob, err := ReadBlob(ctx, bareRepo, "refs/heads/main", "README.md")
	if err != nil {
		t.Fatalf("ReadBlob() error = %v", err)
	}
	if blob.Path != "README.md" || blob.Size != 8 {
		t.Fatalf("blob = %#v", blob)
	}
	if blob.Content != base64.StdEncoding.EncodeToString([]byte("# hello\n")) {
		t.Fatalf("blob content = %q", blob.Content)
	}
}
