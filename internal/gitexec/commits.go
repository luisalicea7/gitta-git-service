package gitexec

import (
	"bytes"
	"context"
	"os/exec"
	"strconv"
	"strings"
)

type Commit struct {
	SHA         string   `json:"sha"`
	Parents     []string `json:"parents"`
	AuthorName  string   `json:"authorName"`
	AuthorEmail string   `json:"authorEmail"`
	AuthoredAt  string   `json:"authoredAt"`
	Subject     string   `json:"subject"`
	Body        string   `json:"body"`
}

func ListCommits(ctx context.Context, repoPath string, revision string, limit int) ([]Commit, error) {
	return ListCommitsForPath(ctx, repoPath, revision, limit, "")
}

func ListCommitsForPath(ctx context.Context, repoPath string, revision string, limit int, historyPath string) ([]Commit, error) {
	args := []string{
		"--git-dir",
		repoPath,
		"log",
		"-n",
		stringLimit(limit),
		"--format=%H%x00%P%x00%an%x00%ae%x00%aI%x00%s%x00%b%x1e",
		revision,
	}
	if historyPath != "" {
		args = append(args, "--", historyPath)
	}

	cmd := exec.CommandContext(ctx, "git", args...)

	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return nil, err
	}

	return ParseCommitLog(stdout.String()), nil
}

func ParseCommitLog(output string) []Commit {
	var commits []Commit

	for _, record := range strings.Split(output, "\x1e") {
		record = strings.Trim(record, "\n")
		if record == "" {
			continue
		}

		fields := strings.SplitN(record, "\x00", 7)
		if len(fields) != 7 {
			continue
		}

		commits = append(commits, Commit{
			SHA:         fields[0],
			Parents:     splitParents(fields[1]),
			AuthorName:  fields[2],
			AuthorEmail: fields[3],
			AuthoredAt:  fields[4],
			Subject:     fields[5],
			Body:        strings.TrimSuffix(fields[6], "\n"),
		})
	}

	return commits
}

func splitParents(value string) []string {
	if strings.TrimSpace(value) == "" {
		return []string{}
	}

	return strings.Fields(value)
}

func stringLimit(limit int) string {
	if limit <= 0 {
		return "50"
	}

	return strconv.Itoa(limit)
}
