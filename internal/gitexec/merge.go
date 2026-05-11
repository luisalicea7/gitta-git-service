package gitexec

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type MergeResult struct {
	Status       string         `json:"status"`
	MergeSHA     string         `json:"mergeSha,omitempty"`
	TargetBranch string         `json:"targetBranch"`
	Files        []ConflictFile `json:"files,omitempty"`
	Reason       string         `json:"reason,omitempty"`
}

type ConflictFile struct {
	Path   string `json:"path"`
	Status string `json:"status"`
}

func MergeCommits(ctx context.Context, repoPath string, targetBranch string, base string, head string) (MergeResult, error) {
	baseSHA, err := ResolveCommit(ctx, repoPath, base)
	if err != nil {
		return MergeResult{}, err
	}

	headSHA, err := ResolveCommit(ctx, repoPath, head)
	if err != nil {
		return MergeResult{}, err
	}

	worktreeParent, err := os.MkdirTemp("", "gitta-merge-*")
	if err != nil {
		return MergeResult{}, err
	}
	worktreePath := filepath.Join(worktreeParent, "worktree")
	defer func() {
		_ = gitRun(ctx, "", "--git-dir", repoPath, "worktree", "remove", "--force", worktreePath)
		_ = gitRun(ctx, "", "--git-dir", repoPath, "worktree", "prune")
		_ = os.RemoveAll(worktreeParent)
	}()

	if err := gitRun(ctx, "", "--git-dir", repoPath, "worktree", "add", "--detach", worktreePath, baseSHA); err != nil {
		return MergeResult{}, err
	}

	_ = gitRun(ctx, worktreePath, "config", "user.email", "gitta@gitta.local")
	_ = gitRun(ctx, worktreePath, "config", "user.name", "Gitta")

	if err := gitRun(ctx, worktreePath, "merge", "--no-ff", "--no-edit", headSHA); err != nil {
		conflicts, conflictErr := listConflictFiles(ctx, worktreePath)
		if conflictErr != nil {
			return MergeResult{}, err
		}

		if len(conflicts) > 0 {
			return MergeResult{
				Status:       "conflict",
				TargetBranch: targetBranch,
				Files:        conflicts,
			}, nil
		}

		return MergeResult{
			Status:       "blocked",
			TargetBranch: targetBranch,
			Reason:       "Merge failed",
		}, nil
	}

	mergeSHA, err := gitOutputFromWorktree(ctx, worktreePath, "rev-parse", "HEAD")
	if err != nil {
		return MergeResult{}, err
	}
	mergeSHA = strings.TrimSpace(mergeSHA)

	targetRef := "refs/heads/" + targetBranch
	if err := gitRun(ctx, "", "--git-dir", repoPath, "update-ref", targetRef, mergeSHA, baseSHA); err != nil {
		return MergeResult{
			Status:       "blocked",
			TargetBranch: targetBranch,
			Reason:       "Target branch moved",
		}, nil
	}

	return MergeResult{
		Status:       "merged",
		MergeSHA:     mergeSHA,
		TargetBranch: targetBranch,
	}, nil
}

func listConflictFiles(ctx context.Context, worktreePath string) ([]ConflictFile, error) {
	output, err := gitOutputFromWorktree(ctx, worktreePath, "status", "--porcelain")
	if err != nil {
		return nil, err
	}

	var conflicts []ConflictFile
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if strings.TrimSpace(line) == "" || len(line) < 4 {
			continue
		}

		code := line[:2]
		if !isConflictStatus(code) {
			continue
		}

		conflicts = append(conflicts, ConflictFile{
			Path:   strings.TrimSpace(line[3:]),
			Status: mapConflictStatus(code),
		})
	}

	return conflicts, nil
}

func isConflictStatus(code string) bool {
	switch code {
	case "DD", "AU", "UD", "UA", "DU", "AA", "UU":
		return true
	default:
		return false
	}
}

func mapConflictStatus(code string) string {
	switch code {
	case "UU":
		return "both_modified"
	case "AA":
		return "both_added"
	case "UD":
		return "deleted_by_them"
	case "DU":
		return "deleted_by_us"
	default:
		return "unknown"
	}
}

func gitOutputFromWorktree(ctx context.Context, worktreePath string, args ...string) (string, error) {
	var stdout bytes.Buffer
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", worktreePath}, args...)...)
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return "", err
	}

	return stdout.String(), nil
}

func gitRun(ctx context.Context, worktreePath string, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", buildGitRunArgs(worktreePath, args...)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			return err
		}
		return errors.New(message)
	}

	return nil
}

func buildGitRunArgs(worktreePath string, args ...string) []string {
	if worktreePath == "" {
		return args
	}

	return append([]string{"-C", worktreePath}, args...)
}
