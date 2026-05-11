package gitexec

import (
	"context"
	"strings"
)

type CompareDetail struct {
	BaseSHA      string        `json:"baseSha"`
	HeadSHA      string        `json:"headSha"`
	MergeBaseSHA string        `json:"mergeBaseSha"`
	Commits      []Commit      `json:"commits"`
	Stats        CommitStats   `json:"stats"`
	Files        []ChangedFile `json:"files"`
}

func CompareCommits(ctx context.Context, repoPath string, base string, head string) (CompareDetail, error) {
	baseSHA, err := ResolveCommit(ctx, repoPath, base)
	if err != nil {
		return CompareDetail{}, err
	}

	headSHA, err := ResolveCommit(ctx, repoPath, head)
	if err != nil {
		return CompareDetail{}, err
	}

	mergeBaseOutput, err := gitOutput(ctx, repoPath, "merge-base", baseSHA, headSHA)
	if err != nil {
		return CompareDetail{}, err
	}
	mergeBaseSHA := strings.TrimSpace(mergeBaseOutput)

	commits, err := ListCommits(ctx, repoPath, mergeBaseSHA+".."+headSHA, 100)
	if err != nil {
		return CompareDetail{}, err
	}

	nameStatusOutput, err := gitOutput(ctx, repoPath, "diff", "--name-status", "-M", "-C", mergeBaseSHA, headSHA)
	if err != nil {
		return CompareDetail{}, err
	}
	files := ParseNameStatus(nameStatusOutput)

	numstatOutput, err := gitOutput(ctx, repoPath, "diff", "--numstat", "-M", "-C", mergeBaseSHA, headSHA)
	if err != nil {
		return CompareDetail{}, err
	}
	ApplyNumstat(files, numstatOutput)

	patchOutput, err := gitOutput(ctx, repoPath, "diff", "--patch", "--find-renames", "--find-copies", mergeBaseSHA, headSHA)
	if err != nil {
		return CompareDetail{}, err
	}
	patches := ParsePatch(patchOutput)
	for index := range files {
		if patch, ok := patches[files[index].Path]; ok {
			files[index].Patch = &patch
		}
	}

	stats := CommitStats{FilesChanged: len(files)}
	for _, file := range files {
		stats.Additions += file.Additions
		stats.Deletions += file.Deletions
	}

	return CompareDetail{
		BaseSHA:      baseSHA,
		HeadSHA:      headSHA,
		MergeBaseSHA: mergeBaseSHA,
		Commits:      commits,
		Stats:        stats,
		Files:        files,
	}, nil
}
