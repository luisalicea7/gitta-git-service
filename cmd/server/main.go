package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/luisalicea7/gitta-git-service/internal/api"
	"github.com/luisalicea7/gitta-git-service/internal/branches"
	"github.com/luisalicea7/gitta-git-service/internal/commits"
	"github.com/luisalicea7/gitta-git-service/internal/config"
	"github.com/luisalicea7/gitta-git-service/internal/health"
	"github.com/luisalicea7/gitta-git-service/internal/httpgit"
	"github.com/luisalicea7/gitta-git-service/internal/logging"
	"github.com/luisalicea7/gitta-git-service/internal/objects"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}

	logger := logging.New(cfg.LogLevel)
	if err := run(cfg, logger); err != nil {
		logger.Error("server stopped", "err", err)
		os.Exit(1)
	}
}

func run(cfg config.Config, logger *slog.Logger) error {
	mux := http.NewServeMux()
	apiClient := api.NewClient(cfg.APIInternalURL, cfg.GitServiceInternalSecret)
	gitHandler := httpgit.NewHandler(apiClient, cfg.RepoRoot, logger)
	commitsHandler := commits.NewHandler(cfg.RepoRoot, cfg.GitServiceInternalSecret, logger)
	objectsHandler := objects.NewHandler(cfg.RepoRoot, cfg.GitServiceInternalSecret, logger)
	branchesHandler := branches.NewHandler(cfg.RepoRoot, cfg.GitServiceInternalSecret, logger)

	mux.HandleFunc("GET /health", health.Handler)
	mux.HandleFunc("GET /health/live", health.Handler)
	mux.HandleFunc("GET /health/ready", health.Handler)
	mux.Handle("POST /internal/repos/commits", commitsHandler)
	mux.HandleFunc("POST /internal/repos/commit", commitsHandler.Detail)
	mux.HandleFunc("POST /internal/repos/compare", commitsHandler.Compare)
	mux.HandleFunc("POST /internal/repos/merge", commitsHandler.Merge)
	mux.HandleFunc("POST /internal/repos/tree", objectsHandler.Tree)
	mux.HandleFunc("POST /internal/repos/blob", objectsHandler.Blob)
	mux.HandleFunc("POST /internal/repos/branches", branchesHandler.Create)
	mux.HandleFunc("DELETE /internal/repos/branches", branchesHandler.Delete)
	mux.Handle("/", gitHandler)

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		logger.Info("gitta git service listening", "addr", server.Addr, "repoRoot", cfg.RepoRoot)
	}()

	errCh := make(chan error, 1)
	go func() {
		err := server.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return err
	case <-signalCh:
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return server.Shutdown(ctx)
	}
}
