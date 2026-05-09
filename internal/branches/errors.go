package branches

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/luisalicea7/gitta-git-service/internal/gitexec"
)

func writeGitError(w http.ResponseWriter, logger *slog.Logger, err error, message string, input branchRequest) {
	if errors.Is(err, gitexec.ErrInvalidBranchRef) {
		http.Error(w, "invalid branch ref", http.StatusBadRequest)
		return
	}

	logger.Error(message, "err", err, "repositoryId", input.RepositoryID, "name", input.Name)
	http.Error(w, message, http.StatusBadGateway)
}
