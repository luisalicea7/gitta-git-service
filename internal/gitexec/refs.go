package gitexec

import (
	"bytes"
	"context"
	"os/exec"
	"strings"

	"github.com/luisalicea7/gitta-git-service/internal/api"
)

func ListRefs(ctx context.Context, repoPath string) ([]api.GitRef, error) {
	cmd := exec.CommandContext(
		ctx,
		"git",
		"--git-dir",
		repoPath,
		"for-each-ref",
		"--format=%(refname) %(objectname)",
	)

	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return nil, err
	}

	var refs []api.GitRef
	for _, line := range strings.Split(strings.TrimSpace(stdout.String()), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}

		name, sha, ok := strings.Cut(line, " ")
		if !ok {
			continue
		}

		refs = append(refs, api.GitRef{
			Name: name,
			SHA:  sha,
			Type: refType(name),
		})
	}

	return refs, nil
}

func refType(name string) string {
	switch {
	case strings.HasPrefix(name, "refs/heads/"):
		return "branch"
	case strings.HasPrefix(name, "refs/tags/"):
		return "tag"
	default:
		return "other"
	}
}
