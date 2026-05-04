package repos

import (
	"errors"
	"path/filepath"
	"strings"
)

type RepositoryIdentity struct {
	ID          string
	OwnerUserID string
}

func Path(repoRoot string, repo RepositoryIdentity) (string, error) {
	if repoRoot == "" {
		return "", errors.New("repo root is required")
	}
	if repo.ID == "" || repo.OwnerUserID == "" {
		return "", errors.New("repository id and owner user id are required")
	}

	root := filepath.Clean(repoRoot)
	path := filepath.Join(root, repo.OwnerUserID, repo.ID+".git")
	clean := filepath.Clean(path)

	rel, err := filepath.Rel(root, clean)
	if err != nil {
		return "", err
	}
	if rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
		return "", errors.New("repository path escapes repo root")
	}

	return clean, nil
}
