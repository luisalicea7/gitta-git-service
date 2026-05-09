package branches

import (
	"log/slog"
	"net/http"

	"github.com/luisalicea7/gitta-git-service/internal/gitexec"
)

const internalSecretHeader = "x-gitta-internal-secret"

type Handler struct {
	repoRoot string
	secret   string
	logger   *slog.Logger
}

func NewHandler(repoRoot string, secret string, logger *slog.Logger) *Handler {
	return &Handler{repoRoot: repoRoot, secret: secret, logger: logger}
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	input, repoPath, ok := parseRequest(w, r, h.repoRoot, h.secret, h.logger, true)
	if !ok {
		return
	}

	if err := gitexec.CreateBranch(r.Context(), repoPath, input.Name, input.SHA); err != nil {
		writeGitError(w, h.logger, err, "create branch failed", input)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	input, repoPath, ok := parseRequest(w, r, h.repoRoot, h.secret, h.logger, false)
	if !ok {
		return
	}

	if err := gitexec.DeleteBranch(r.Context(), repoPath, input.Name); err != nil {
		writeGitError(w, h.logger, err, "delete branch failed", input)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
