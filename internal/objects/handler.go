package objects

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/luisalicea7/gitta-git-service/internal/gitexec"
	"github.com/luisalicea7/gitta-git-service/internal/repos"
)

const internalSecretHeader = "x-gitta-internal-secret"

type Handler struct {
	repoRoot string
	secret   string
	logger   *slog.Logger
}

type objectRequest struct {
	RepositoryID string `json:"repositoryId"`
	OwnerUserID  string `json:"ownerUserId"`
	SHA          string `json:"sha"`
	Path         string `json:"path"`
}

type treeResponse struct {
	Entries []gitexec.TreeEntry `json:"entries"`
}

type blobResponse struct {
	Blob gitexec.Blob `json:"blob"`
}

func NewHandler(repoRoot string, secret string, logger *slog.Logger) *Handler {
	return &Handler{repoRoot: repoRoot, secret: secret, logger: logger}
}

func (h *Handler) Tree(w http.ResponseWriter, r *http.Request) {
	input, repoPath, ok := h.parseRequest(w, r)
	if !ok {
		return
	}

	entries, err := gitexec.ListTree(r.Context(), repoPath, input.SHA, input.Path)
	if err != nil {
		h.logger.Error("git ls-tree failed", "err", err, "repositoryId", input.RepositoryID)
		http.Error(w, "tree not found", http.StatusNotFound)
		return
	}

	writeJSON(w, treeResponse{Entries: entries}, h.logger)
}

func (h *Handler) Blob(w http.ResponseWriter, r *http.Request) {
	input, repoPath, ok := h.parseRequest(w, r)
	if !ok {
		return
	}

	blob, err := gitexec.ReadBlob(r.Context(), repoPath, input.SHA, input.Path)
	if err != nil {
		h.logger.Error("git blob read failed", "err", err, "repositoryId", input.RepositoryID)
		http.Error(w, "blob not found", http.StatusNotFound)
		return
	}

	writeJSON(w, blobResponse{Blob: blob}, h.logger)
}

func (h *Handler) parseRequest(w http.ResponseWriter, r *http.Request) (objectRequest, string, bool) {
	if r.Header.Get(internalSecretHeader) != h.secret {
		http.Error(w, "forbidden", http.StatusForbidden)
		return objectRequest{}, "", false
	}

	var input objectRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return objectRequest{}, "", false
	}

	if input.RepositoryID == "" || input.OwnerUserID == "" || input.SHA == "" {
		http.Error(w, "repositoryId, ownerUserId, and sha are required", http.StatusBadRequest)
		return objectRequest{}, "", false
	}
	if !validGitPath(input.Path) {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return objectRequest{}, "", false
	}

	repoPath, err := repos.Path(h.repoRoot, repos.RepositoryIdentity{
		ID:          input.RepositoryID,
		OwnerUserID: input.OwnerUserID,
	})
	if err != nil {
		h.logger.Error("object repo path failed", "err", err)
		http.Error(w, "invalid repository", http.StatusBadRequest)
		return objectRequest{}, "", false
	}

	if !repos.ExistsBare(repoPath) {
		http.Error(w, "repository not found", http.StatusNotFound)
		return objectRequest{}, "", false
	}

	return input, repoPath, true
}

func validGitPath(value string) bool {
	if strings.HasPrefix(value, "/") {
		return false
	}

	for _, part := range strings.Split(value, "/") {
		if part == ".." {
			return false
		}
	}

	return true
}

func writeJSON(w http.ResponseWriter, body any, logger *slog.Logger) {
	w.Header().Set("content-type", "application/json")
	if err := json.NewEncoder(w).Encode(body); err != nil {
		logger.Error("write object response failed", "err", err)
	}
}
