package gitexec

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"os/exec"
	"path"
	"strconv"
	"strings"
)

type TreeEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
	SHA  string `json:"sha"`
	Type string `json:"type"`
	Mode string `json:"mode"`
	Size *int64 `json:"size"`
}

type Blob struct {
	Path     string `json:"path"`
	SHA      string `json:"sha"`
	Size     int64  `json:"size"`
	Encoding string `json:"encoding"`
	Content  string `json:"content"`
}

func ListTree(ctx context.Context, repoPath string, revision string, treePath string) ([]TreeEntry, error) {
	spec := revision
	if treePath != "" {
		spec = revision + ":" + treePath
	}

	cmd := exec.CommandContext(
		ctx,
		"git",
		"--git-dir",
		repoPath,
		"ls-tree",
		"-z",
		"-l",
		spec,
	)

	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return nil, err
	}

	return ParseTree(stdout.String(), treePath), nil
}

func ReadBlob(ctx context.Context, repoPath string, revision string, blobPath string) (Blob, error) {
	if blobPath == "" {
		return Blob{}, errors.New("blob path is required")
	}

	spec := revision + ":" + blobPath
	objectType, err := gitOutput(ctx, repoPath, "cat-file", "-t", spec)
	if err != nil {
		return Blob{}, err
	}
	if strings.TrimSpace(objectType) != "blob" {
		return Blob{}, errors.New("object is not a blob")
	}

	sha, err := gitOutput(ctx, repoPath, "rev-parse", spec)
	if err != nil {
		return Blob{}, err
	}

	sizeOutput, err := gitOutput(ctx, repoPath, "cat-file", "-s", spec)
	if err != nil {
		return Blob{}, err
	}
	size, err := strconv.ParseInt(strings.TrimSpace(sizeOutput), 10, 64)
	if err != nil {
		return Blob{}, err
	}

	content, err := gitBytes(ctx, repoPath, "show", spec)
	if err != nil {
		return Blob{}, err
	}

	return Blob{
		Path:     blobPath,
		SHA:      strings.TrimSpace(sha),
		Size:     size,
		Encoding: "base64",
		Content:  base64.StdEncoding.EncodeToString(content),
	}, nil
}

func ParseTree(output string, basePath string) []TreeEntry {
	var entries []TreeEntry

	for _, record := range strings.Split(output, "\x00") {
		if record == "" {
			continue
		}

		meta, entryPath, ok := strings.Cut(record, "\t")
		if !ok {
			continue
		}

		fields := strings.Fields(meta)
		if len(fields) != 4 {
			continue
		}

		size := parseObjectSize(fields[3])
		entries = append(entries, TreeEntry{
			Name: path.Base(entryPath),
			Path: joinGitPath(basePath, path.Base(entryPath)),
			SHA:  fields[2],
			Type: treeEntryType(fields[1]),
			Mode: fields[0],
			Size: size,
		})
	}

	return entries
}

func gitOutput(ctx context.Context, repoPath string, args ...string) (string, error) {
	output, err := gitBytes(ctx, repoPath, args...)
	return string(output), err
}

func gitBytes(ctx context.Context, repoPath string, args ...string) ([]byte, error) {
	allArgs := append([]string{"--git-dir", repoPath}, args...)
	cmd := exec.CommandContext(ctx, "git", allArgs...)
	return cmd.Output()
}

func parseObjectSize(value string) *int64 {
	if value == "-" {
		return nil
	}

	size, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return nil
	}

	return &size
}

func treeEntryType(value string) string {
	switch value {
	case "blob", "tree", "commit":
		return value
	default:
		return "other"
	}
}

func joinGitPath(basePath string, name string) string {
	if basePath == "" {
		return name
	}

	return path.Join(basePath, name)
}
