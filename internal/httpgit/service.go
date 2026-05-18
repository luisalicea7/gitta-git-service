package httpgit

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/luisalicea7/gitta-git-service/internal/api"
	"github.com/luisalicea7/gitta-git-service/internal/gitexec"
	"github.com/luisalicea7/gitta-git-service/internal/repos"
)

type Service struct {
	apiClient *api.Client
	repoRoot  string
	logger    *slog.Logger
}

type AuthorizedRepository struct {
	Auth     api.AuthResponse
	RepoPath string
}

func NewService(apiClient *api.Client, repoRoot string, logger *slog.Logger) *Service {
	return &Service{
		apiClient: apiClient,
		repoRoot:  repoRoot,
		logger:    logger,
	}
}

func (s *Service) Authorize(
	ctx context.Context,
	route Route,
	gitService gitexec.Service,
	username string,
	token string,
) (api.AuthResponse, int, error) {
	return s.apiClient.Authorize(ctx, api.AuthRequest{
		Username:  username,
		Token:     token,
		Owner:     route.Owner,
		Repo:      route.Repo,
		Operation: api.GitOperation(gitService.ShortName()),
	})
}

func (s *Service) PrepareRepository(
	ctx context.Context,
	auth api.AuthResponse,
	gitService gitexec.Service,
) (string, PrepareResult, error) {
	repoPath, err := repos.Path(s.repoRoot, repos.RepositoryIdentity{
		ID:          auth.Repository.ID,
		OwnerUserID: auth.Repository.OwnerUserID,
	})
	if err != nil {
		return "", PrepareInternalError, err
	}

	if gitService == gitexec.ReceivePack {
		if err := repos.EnsureBare(ctx, repoPath); err != nil {
			return "", PrepareInternalError, err
		}
		if err := repos.EnsurePreReceiveHook(repoPath); err != nil {
			return "", PrepareInternalError, err
		}
		return repoPath, PrepareOK, nil
	}

	if !repos.ExistsBare(repoPath) {
		return "", PrepareNotFound, nil
	}

	return repoPath, PrepareOK, nil
}

func (s *Service) PostReceive(
	ctx context.Context,
	auth api.AuthResponse,
	refs []api.GitRef,
) (api.PostReceiveResponse, int, error) {
	return s.apiClient.PostReceive(ctx, api.PostReceiveRequest{
		RepositoryID: auth.Repository.ID,
		Refs:         refs,
	})
}

func (s *Service) GitEnv(auth api.AuthResponse) []string {
	return []string{
		fmt.Sprintf("GITTA_API_URL=%s", s.apiClient.BaseURL()),
		fmt.Sprintf("GITTA_INTERNAL_SECRET=%s", s.apiClient.Secret()),
		fmt.Sprintf("GITTA_REPOSITORY_ID=%s", auth.Repository.ID),
		fmt.Sprintf("GITTA_USER_ID=%s", auth.UserID),
		fmt.Sprintf("GITTA_PERMISSION=%s", auth.Permission),
	}
}

type PrepareResult string

const (
	PrepareOK            PrepareResult = "ok"
	PrepareNotFound      PrepareResult = "not_found"
	PrepareInternalError PrepareResult = "internal_error"
)
