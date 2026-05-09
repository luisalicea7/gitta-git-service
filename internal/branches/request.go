package branches

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/luisalicea7/gitta-git-service/internal/gitexec"
	"github.com/luisalicea7/gitta-git-service/internal/repos"
)

func parseRequest(
	w http.ResponseWriter,
	r *http.Request,
	repoRoot string,
	secret string,
	logger *slog.Logger,
	requireSHA bool,
) (branchRequest, string, bool) {
	if r.Header.Get(internalSecretHeader) != secret {
		http.Error(w, "forbidden", http.StatusForbidden)
		return branchRequest{}, "", false
	}

	var input branchRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return branchRequest{}, "", false
	}

	if input.RepositoryID == "" || input.OwnerUserID == "" || input.Name == "" {
		http.Error(w, "repositoryId, ownerUserId, and name are required", http.StatusBadRequest)
		return branchRequest{}, "", false
	}
	if requireSHA && input.SHA == "" {
		http.Error(w, "sha is required", http.StatusBadRequest)
		return branchRequest{}, "", false
	}
	if !gitexec.ValidBranchRef(input.Name) {
		http.Error(w, "invalid branch ref", http.StatusBadRequest)
		return branchRequest{}, "", false
	}

	repoPath, err := repos.Path(repoRoot, repos.RepositoryIdentity{
		ID:          input.RepositoryID,
		OwnerUserID: input.OwnerUserID,
	})
	if err != nil {
		logger.Error("branch repo path failed", "err", err)
		http.Error(w, "invalid repository", http.StatusBadRequest)
		return branchRequest{}, "", false
	}

	if !repos.ExistsBare(repoPath) {
		http.Error(w, "repository not found", http.StatusNotFound)
		return branchRequest{}, "", false
	}

	return input, repoPath, true
}
