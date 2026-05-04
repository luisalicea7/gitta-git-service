package config

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

type Config struct {
	Port                     string
	APIInternalURL           string
	GitServiceInternalSecret string
	RepoRoot                 string
	LogLevel                 string
}

func Load() (Config, error) {
	_ = godotenv.Load(".env")

	cfg := Config{
		Port:           getEnv("PORT", "4001"),
		APIInternalURL: getEnv("API_INTERNAL_URL", "http://localhost:3000"),
		RepoRoot:       getEnv("REPO_ROOT", ".data/repos"),
		LogLevel:       getEnv("LOG_LEVEL", "debug"),
	}
	cfg.GitServiceInternalSecret = os.Getenv("GIT_SERVICE_INTERNAL_SECRET")
	if cfg.GitServiceInternalSecret == "" {
		return Config{}, errors.New("GIT_SERVICE_INTERNAL_SECRET is required")
	}

	absRepoRoot, err := filepath.Abs(cfg.RepoRoot)
	if err != nil {
		return Config{}, err
	}
	cfg.RepoRoot = filepath.Clean(absRepoRoot)

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
