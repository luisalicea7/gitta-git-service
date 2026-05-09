package gitexec

import (
	"context"
	"errors"
	"os/exec"
	"strings"
)

var ErrInvalidBranchRef = errors.New("invalid branch ref")

func CreateBranch(ctx context.Context, repoPath string, name string, sha string) error {
	if !ValidBranchRef(name) || strings.TrimSpace(sha) == "" {
		return ErrInvalidBranchRef
	}

	cmd := exec.CommandContext(ctx, "git", "--git-dir", repoPath, "update-ref", name, sha)
	return cmd.Run()
}

func DeleteBranch(ctx context.Context, repoPath string, name string) error {
	if !ValidBranchRef(name) {
		return ErrInvalidBranchRef
	}

	cmd := exec.CommandContext(ctx, "git", "--git-dir", repoPath, "update-ref", "-d", name)
	return cmd.Run()
}

func ValidBranchRef(name string) bool {
	if !strings.HasPrefix(name, "refs/heads/") {
		return false
	}

	shortName := strings.TrimPrefix(name, "refs/heads/")
	if shortName == "" {
		return false
	}
	if strings.HasPrefix(shortName, "/") || strings.HasSuffix(shortName, "/") {
		return false
	}
	if strings.HasPrefix(shortName, ".") || strings.HasSuffix(shortName, ".") {
		return false
	}
	if strings.HasPrefix(shortName, "-") || strings.HasSuffix(shortName, ".lock") {
		return false
	}
	if strings.Contains(shortName, "..") || strings.Contains(shortName, "//") || strings.Contains(shortName, "@{") {
		return false
	}
	if strings.ContainsAny(shortName, " ~^:?*[\\") {
		return false
	}

	for _, r := range shortName {
		if r < 32 || r == 127 {
			return false
		}
	}

	return true
}
