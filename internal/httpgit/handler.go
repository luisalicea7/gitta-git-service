package httpgit

import (
	"log/slog"
	"net/http"

	"github.com/luisalicea7/gitta-git-service/internal/api"
	"github.com/luisalicea7/gitta-git-service/internal/gitexec"
	"github.com/luisalicea7/gitta-git-service/internal/httpx"
)

type Handler struct {
	service *Service
	logger  *slog.Logger
}

func NewHandler(apiClient *api.Client, repoRoot string, logger *slog.Logger) *Handler {
	return &Handler{
		service: NewService(apiClient, repoRoot, logger),
		logger:  logger,
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	route, err := ParseRoute(r.URL.Path)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if route.IsInfoRef {
		h.handleInfoRefs(w, r, route)
		return
	}

	if route.IsRPC {
		h.handleRPC(w, r, route)
		return
	}

	http.NotFound(w, r)
}

func (h *Handler) handleInfoRefs(w http.ResponseWriter, r *http.Request, route Route) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	service, ok := gitexec.ServiceFromGitName(r.URL.Query().Get("service"))
	if !ok {
		http.NotFound(w, r)
		return
	}

	auth, ok := h.authorize(w, r, route, service)
	if !ok {
		return
	}

	repoPath, ok := h.prepareRepo(w, r, auth, service)
	if !ok {
		return
	}

	w.Header().Set("content-type", service.AdvertisementContentType())
	httpx.WriteNoCacheHeaders(w)

	if err := gitexec.WriteAdvertisedRefs(r.Context(), w, service, repoPath, h.logger); err != nil {
		h.logger.Error("write advertised refs failed", "err", err)
	}
}

func (h *Handler) handleRPC(w http.ResponseWriter, r *http.Request, route Route) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	service, ok := gitexec.ServiceFromGitName(route.Service)
	if !ok {
		http.NotFound(w, r)
		return
	}

	auth, ok := h.authorize(w, r, route, service)
	if !ok {
		return
	}

	repoPath, ok := h.prepareRepo(w, r, auth, service)
	if !ok {
		return
	}

	w.Header().Set("content-type", service.ResultContentType())
	if err := gitexec.RunRPC(r.Context(), r.Body, w, service, repoPath, h.logger); err != nil {
		h.logger.Error("git rpc failed", "err", err, "service", service)
		return
	}

	if service == gitexec.ReceivePack {
		h.postReceive(r, auth, repoPath)
	}
}

func (h *Handler) authorize(
	w http.ResponseWriter,
	r *http.Request,
	route Route,
	service gitexec.Service,
) (api.AuthResponse, bool) {
	username, token, ok := r.BasicAuth()
	if !ok {
		httpx.WriteBasicAuthChallenge(w)
		return api.AuthResponse{}, false
	}

	auth, status, err := h.service.Authorize(r.Context(), route, service, username, token)
	if err != nil {
		h.logger.Error("api authorize failed", "err", err, "status", status)
		http.Error(w, "authorization failed", http.StatusBadGateway)
		return api.AuthResponse{}, false
	}

	if auth.Allowed {
		return auth, true
	}

	writeAuthDenied(w, auth, service)
	return api.AuthResponse{}, false
}

func (h *Handler) prepareRepo(
	w http.ResponseWriter,
	r *http.Request,
	auth api.AuthResponse,
	service gitexec.Service,
) (string, bool) {
	repoPath, result, err := h.service.PrepareRepository(r.Context(), auth, service)
	if err != nil {
		h.logger.Error("prepare repository failed", "err", err, "result", result)
		writePrepareFailure(w, result)
		return "", false
	}

	if result != PrepareOK {
		writePrepareFailure(w, result)
		return "", false
	}

	return repoPath, true
}

func (h *Handler) postReceive(r *http.Request, auth api.AuthResponse, repoPath string) {
	refs, err := gitexec.ListRefs(r.Context(), repoPath)
	if err != nil {
		h.logger.Error("list refs failed", "err", err, "repositoryId", auth.Repository.ID)
		return
	}

	result, status, err := h.service.PostReceive(r.Context(), auth, refs)
	if err != nil {
		h.logger.Error(
			"post-receive callback failed",
			"err",
			err,
			"status",
			status,
			"reason",
			result.Reason,
			"repositoryId",
			auth.Repository.ID,
		)
		return
	}

	h.logger.Debug(
		"post-receive synced refs",
		"repositoryId",
		auth.Repository.ID,
		"synced",
		result.Synced,
	)
}
