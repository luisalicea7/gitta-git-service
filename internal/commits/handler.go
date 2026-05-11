package commits

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/luisalicea7/gitta-git-service/internal/gitexec"
	"github.com/luisalicea7/gitta-git-service/internal/repos"
)

const internalSecretHeader = "x-gitta-internal-secret"

type Handler struct {
	repoRoot string
	secret   string
	logger   *slog.Logger
}

type listRequest struct {
	RepositoryID string `json:"repositoryId"`
	OwnerUserID  string `json:"ownerUserId"`
	SHA          string `json:"sha"`
	Limit        int    `json:"limit"`
	Path         string `json:"path"`
}

type listResponse struct {
	Commits []gitexec.Commit `json:"commits"`
}

type detailRequest struct {
	RepositoryID string `json:"repositoryId"`
	OwnerUserID  string `json:"ownerUserId"`
	SHA          string `json:"sha"`
}

type detailResponse struct {
	Commit gitexec.Commit        `json:"commit"`
	Stats  gitexec.CommitStats   `json:"stats"`
	Files  []gitexec.ChangedFile `json:"files"`
}

type compareRequest struct {
	RepositoryID string `json:"repositoryId"`
	OwnerUserID  string `json:"ownerUserId"`
	BaseSHA      string `json:"baseSha"`
	HeadSHA      string `json:"headSha"`
}

type mergeRequest struct {
	RepositoryID string `json:"repositoryId"`
	OwnerUserID  string `json:"ownerUserId"`
	TargetBranch string `json:"targetBranch"`
	BaseSHA      string `json:"baseSha"`
	HeadSHA      string `json:"headSha"`
}

func NewHandler(repoRoot string, secret string, logger *slog.Logger) *Handler {
	return &Handler{repoRoot: repoRoot, secret: secret, logger: logger}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get(internalSecretHeader) != h.secret {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	var input listRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}

	if input.RepositoryID == "" || input.OwnerUserID == "" || input.SHA == "" {
		http.Error(w, "repositoryId, ownerUserId, and sha are required", http.StatusBadRequest)
		return
	}
	if input.Limit <= 0 || input.Limit > 100 {
		http.Error(w, "limit must be between 1 and 100", http.StatusBadRequest)
		return
	}

	repoPath, err := repos.Path(h.repoRoot, repos.RepositoryIdentity{
		ID:          input.RepositoryID,
		OwnerUserID: input.OwnerUserID,
	})
	if err != nil {
		h.logger.Error("commit repo path failed", "err", err)
		http.Error(w, "invalid repository", http.StatusBadRequest)
		return
	}

	if !repos.ExistsBare(repoPath) {
		http.Error(w, "repository not found", http.StatusNotFound)
		return
	}

	commits, err := gitexec.ListCommitsForPath(r.Context(), repoPath, input.SHA, input.Limit, input.Path)
	if err != nil {
		h.logger.Error("git log failed", "err", err, "repositoryId", input.RepositoryID)
		http.Error(w, "git log failed", http.StatusBadGateway)
		return
	}

	w.Header().Set("content-type", "application/json")
	if err := json.NewEncoder(w).Encode(listResponse{Commits: commits}); err != nil {
		h.logger.Error("write commits response failed", "err", err)
	}
}

func (h *Handler) Detail(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get(internalSecretHeader) != h.secret {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	var input detailRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}

	if input.RepositoryID == "" || input.OwnerUserID == "" || input.SHA == "" {
		http.Error(w, "repositoryId, ownerUserId, and sha are required", http.StatusBadRequest)
		return
	}

	repoPath, err := repos.Path(h.repoRoot, repos.RepositoryIdentity{
		ID:          input.RepositoryID,
		OwnerUserID: input.OwnerUserID,
	})
	if err != nil {
		h.logger.Error("commit detail repo path failed", "err", err)
		http.Error(w, "invalid repository", http.StatusBadRequest)
		return
	}

	if !repos.ExistsBare(repoPath) {
		http.Error(w, "repository not found", http.StatusNotFound)
		return
	}

	detail, err := gitexec.ReadCommitDetail(r.Context(), repoPath, input.SHA)
	if err != nil {
		h.logger.Error("git commit detail failed", "err", err, "repositoryId", input.RepositoryID)
		http.Error(w, "commit not found", http.StatusNotFound)
		return
	}

	w.Header().Set("content-type", "application/json")
	if err := json.NewEncoder(w).Encode(detailResponse{
		Commit: detail.Commit,
		Stats:  detail.Stats,
		Files:  detail.Files,
	}); err != nil {
		h.logger.Error("write commit detail response failed", "err", err)
	}
}

func (h *Handler) Compare(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get(internalSecretHeader) != h.secret {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	var input compareRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}

	if input.RepositoryID == "" || input.OwnerUserID == "" || input.BaseSHA == "" || input.HeadSHA == "" {
		http.Error(w, "repositoryId, ownerUserId, baseSha, and headSha are required", http.StatusBadRequest)
		return
	}

	repoPath, err := repos.Path(h.repoRoot, repos.RepositoryIdentity{
		ID:          input.RepositoryID,
		OwnerUserID: input.OwnerUserID,
	})
	if err != nil {
		h.logger.Error("compare repo path failed", "err", err)
		http.Error(w, "invalid repository", http.StatusBadRequest)
		return
	}

	if !repos.ExistsBare(repoPath) {
		http.Error(w, "repository not found", http.StatusNotFound)
		return
	}

	compare, err := gitexec.CompareCommits(r.Context(), repoPath, input.BaseSHA, input.HeadSHA)
	if err != nil {
		h.logger.Error("git compare failed", "err", err, "repositoryId", input.RepositoryID)
		http.Error(w, "compare not found", http.StatusNotFound)
		return
	}

	w.Header().Set("content-type", "application/json")
	if err := json.NewEncoder(w).Encode(compare); err != nil {
		h.logger.Error("write compare response failed", "err", err)
	}
}

func (h *Handler) Merge(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get(internalSecretHeader) != h.secret {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	var input mergeRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}

	if input.RepositoryID == "" || input.OwnerUserID == "" || input.TargetBranch == "" || input.BaseSHA == "" || input.HeadSHA == "" {
		http.Error(w, "repositoryId, ownerUserId, targetBranch, baseSha, and headSha are required", http.StatusBadRequest)
		return
	}

	repoPath, err := repos.Path(h.repoRoot, repos.RepositoryIdentity{
		ID:          input.RepositoryID,
		OwnerUserID: input.OwnerUserID,
	})
	if err != nil {
		h.logger.Error("merge repo path failed", "err", err)
		http.Error(w, "invalid repository", http.StatusBadRequest)
		return
	}

	if !repos.ExistsBare(repoPath) {
		http.Error(w, "repository not found", http.StatusNotFound)
		return
	}

	result, err := gitexec.MergeCommits(r.Context(), repoPath, input.TargetBranch, input.BaseSHA, input.HeadSHA)
	if err != nil {
		h.logger.Error("git merge failed", "err", err, "repositoryId", input.RepositoryID)
		http.Error(w, "merge failed", http.StatusBadGateway)
		return
	}

	w.Header().Set("content-type", "application/json")
	if err := json.NewEncoder(w).Encode(result); err != nil {
		h.logger.Error("write merge response failed", "err", err)
	}
}
