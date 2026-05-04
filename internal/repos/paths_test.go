package repos

import (
	"path/filepath"
	"testing"
)

func TestPath(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	got, err := Path(root, RepositoryIdentity{
		OwnerUserID: "owner-id",
		ID:          "repo-id",
	})
	if err != nil {
		t.Fatalf("Path() error = %v", err)
	}

	want := filepath.Join(root, "owner-id", "repo-id.git")
	if got != want {
		t.Fatalf("Path() = %q, want %q", got, want)
	}
}

func TestPathRejectsMissingInputs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		root string
		repo RepositoryIdentity
	}{
		{name: "missing root", repo: RepositoryIdentity{OwnerUserID: "owner", ID: "repo"}},
		{name: "missing owner", root: t.TempDir(), repo: RepositoryIdentity{ID: "repo"}},
		{name: "missing repo", root: t.TempDir(), repo: RepositoryIdentity{OwnerUserID: "owner"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if _, err := Path(tt.root, tt.repo); err == nil {
				t.Fatal("Path() error = nil, want error")
			}
		})
	}
}
