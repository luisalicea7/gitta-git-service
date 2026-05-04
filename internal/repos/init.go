package repos

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

func ExistsBare(repoPath string) bool {
	info, err := os.Stat(filepath.Join(repoPath, "HEAD"))
	if err != nil || info.IsDir() {
		return false
	}

	objects, err := os.Stat(filepath.Join(repoPath, "objects"))
	if err != nil || !objects.IsDir() {
		return false
	}

	refs, err := os.Stat(filepath.Join(repoPath, "refs"))
	return err == nil && refs.IsDir()
}

func EnsureBare(ctx context.Context, repoPath string) error {
	if ExistsBare(repoPath) {
		return nil
	}

	if _, err := os.Stat(repoPath); err == nil {
		return errors.New("repository path exists but is not a bare git repository")
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(repoPath), 0o750); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "init", "--bare", repoPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return errors.New("git init --bare failed: " + string(output))
	}

	return nil
}
