package gitexec

import (
	"context"
	"errors"
	"strconv"
	"strings"
)

type CommitStats struct {
	FilesChanged int `json:"filesChanged"`
	Additions    int `json:"additions"`
	Deletions    int `json:"deletions"`
}

type ChangedFile struct {
	Path      string     `json:"path"`
	OldPath   *string    `json:"oldPath"`
	Status    string     `json:"status"`
	Additions int        `json:"additions"`
	Deletions int        `json:"deletions"`
	IsBinary  bool       `json:"isBinary"`
	OldSHA    *string    `json:"oldSha"`
	NewSHA    *string    `json:"newSha"`
	Patch     *FilePatch `json:"patch"`
}

type FilePatch struct {
	Hunks []DiffHunk `json:"hunks"`
}

type DiffHunk struct {
	Header   string     `json:"header"`
	OldStart int        `json:"oldStart"`
	OldLines int        `json:"oldLines"`
	NewStart int        `json:"newStart"`
	NewLines int        `json:"newLines"`
	Lines    []DiffLine `json:"lines"`
}

type DiffLine struct {
	Type          string `json:"type"`
	OldLineNumber *int   `json:"oldLineNumber"`
	NewLineNumber *int   `json:"newLineNumber"`
	Content       string `json:"content"`
}

type CommitDetail struct {
	Commit Commit        `json:"commit"`
	Stats  CommitStats   `json:"stats"`
	Files  []ChangedFile `json:"files"`
}

type fileIdentity struct {
	Path    string
	OldPath *string
	Status  string
}

func ReadCommitDetail(ctx context.Context, repoPath string, sha string) (CommitDetail, error) {
	fullSHA, err := ResolveCommit(ctx, repoPath, sha)
	if err != nil {
		return CommitDetail{}, err
	}

	commits, err := ListCommits(ctx, repoPath, fullSHA, 1)
	if err != nil {
		return CommitDetail{}, err
	}
	if len(commits) == 0 {
		return CommitDetail{}, errors.New("commit not found")
	}

	nameStatusOutput, err := gitOutput(ctx, repoPath, "show", "--format=", "--name-status", "-M", "-C", "--root", fullSHA)
	if err != nil {
		return CommitDetail{}, err
	}
	files := ParseNameStatus(nameStatusOutput)

	numstatOutput, err := gitOutput(ctx, repoPath, "show", "--format=", "--numstat", "-M", "-C", "--root", fullSHA)
	if err != nil {
		return CommitDetail{}, err
	}
	ApplyNumstat(files, numstatOutput)

	patchOutput, err := gitOutput(ctx, repoPath, "show", "--format=", "--patch", "--find-renames", "--find-copies", "--root", fullSHA)
	if err != nil {
		return CommitDetail{}, err
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

	return CommitDetail{
		Commit: commits[0],
		Stats:  stats,
		Files:  files,
	}, nil
}

func ResolveCommit(ctx context.Context, repoPath string, sha string) (string, error) {
	output, err := gitOutput(ctx, repoPath, "rev-parse", "--verify", sha+"^{commit}")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(output), nil
}

func ParseNameStatus(output string) []ChangedFile {
	var files []ChangedFile
	for _, rawLine := range strings.Split(strings.TrimSpace(output), "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 2 {
			continue
		}

		statusCode := parts[0]
		status := statusFromCode(statusCode)
		if (strings.HasPrefix(statusCode, "R") || strings.HasPrefix(statusCode, "C")) && len(parts) >= 3 {
			oldPath := parts[1]
			files = append(files, ChangedFile{
				Path:    parts[2],
				OldPath: &oldPath,
				Status:  status,
			})
			continue
		}

		files = append(files, ChangedFile{
			Path:   parts[1],
			Status: status,
		})
	}
	return files
}

func ApplyNumstat(files []ChangedFile, output string) {
	index := map[string]int{}
	for i, file := range files {
		index[file.Path] = i
	}

	for _, rawLine := range strings.Split(strings.TrimSpace(output), "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 3 {
			continue
		}
		path := parts[len(parts)-1]
		i, ok := index[path]
		if !ok {
			continue
		}
		if parts[0] == "-" || parts[1] == "-" {
			files[i].IsBinary = true
			continue
		}
		files[i].Additions, _ = strconv.Atoi(parts[0])
		files[i].Deletions, _ = strconv.Atoi(parts[1])
	}
}

func ParsePatch(output string) map[string]FilePatch {
	patches := map[string]FilePatch{}
	var currentPath string
	var currentHunk *DiffHunk
	oldLine := 0
	newLine := 0

	flushHunk := func() {
		if currentPath == "" || currentHunk == nil {
			return
		}
		patch := patches[currentPath]
		patch.Hunks = append(patch.Hunks, *currentHunk)
		patches[currentPath] = patch
		currentHunk = nil
	}

	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, "diff --git ") {
			flushHunk()
			currentPath = parseDiffGitPath(line)
			continue
		}
		if strings.HasPrefix(line, "+++ b/") {
			currentPath = strings.TrimPrefix(line, "+++ b/")
			continue
		}
		if strings.HasPrefix(line, "@@ ") {
			flushHunk()
			hunk := parseHunkHeader(line)
			currentHunk = &hunk
			oldLine = hunk.OldStart
			newLine = hunk.NewStart
			continue
		}
		if currentPath == "" || currentHunk == nil {
			continue
		}
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, `\ No newline at end of file`) {
			continue
		}

		switch {
		case strings.HasPrefix(line, "+"):
			number := newLine
			currentHunk.Lines = append(currentHunk.Lines, DiffLine{Type: "add", NewLineNumber: &number, Content: strings.TrimPrefix(line, "+")})
			newLine++
		case strings.HasPrefix(line, "-"):
			number := oldLine
			currentHunk.Lines = append(currentHunk.Lines, DiffLine{Type: "delete", OldLineNumber: &number, Content: strings.TrimPrefix(line, "-")})
			oldLine++
		default:
			content := line
			if strings.HasPrefix(line, " ") {
				content = strings.TrimPrefix(line, " ")
			}
			oldNumber := oldLine
			newNumber := newLine
			currentHunk.Lines = append(currentHunk.Lines, DiffLine{Type: "context", OldLineNumber: &oldNumber, NewLineNumber: &newNumber, Content: content})
			oldLine++
			newLine++
		}
	}
	flushHunk()

	return patches
}

func statusFromCode(code string) string {
	switch {
	case strings.HasPrefix(code, "A"):
		return "added"
	case strings.HasPrefix(code, "M"):
		return "modified"
	case strings.HasPrefix(code, "D"):
		return "deleted"
	case strings.HasPrefix(code, "R"):
		return "renamed"
	case strings.HasPrefix(code, "C"):
		return "copied"
	case strings.HasPrefix(code, "T"):
		return "type_changed"
	default:
		return "unknown"
	}
}

func parseDiffGitPath(line string) string {
	parts := strings.Fields(line)
	if len(parts) < 4 {
		return ""
	}
	return strings.TrimPrefix(parts[3], "b/")
}

func parseHunkHeader(line string) DiffHunk {
	fields := strings.Fields(line)
	hunk := DiffHunk{Header: line}
	if len(fields) < 3 {
		return hunk
	}
	hunk.OldStart, hunk.OldLines = parseHunkRange(strings.TrimPrefix(fields[1], "-"))
	hunk.NewStart, hunk.NewLines = parseHunkRange(strings.TrimPrefix(fields[2], "+"))
	return hunk
}

func parseHunkRange(value string) (int, int) {
	startText, linesText, ok := strings.Cut(value, ",")
	start, _ := strconv.Atoi(startText)
	if !ok {
		return start, 1
	}
	lines, _ := strconv.Atoi(linesText)
	return start, lines
}
