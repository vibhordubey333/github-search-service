package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port           string
	GithubToken    string
	RequestTimeout time.Duration
	MaxConcurrency int
	LogLevel       string
	GithubBaseURL  string
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func Load() (*Config, error) {
	cfg := &Config{
		Port:          getEnv("PORT", "50051"),
		GithubToken:   os.Getenv("GITHUB_TOKEN"),
		LogLevel:      getEnv("LOG_LEVEL", "info"),
		GithubBaseURL: getEnv("GITHUB_BASE_URL", "https://api.github.com"),
	}

	//GITHUB_TOKEN is mandatory, else fail-fast
	if cfg.GithubToken == "" {
		return nil, fmt.Errorf("GITHUB_TOKEN is required")
	}

	timeout, err := time.ParseDuration(getEnv("REQUEST_TIMEOUT", "10s"))
	if err != nil {
		return nil, fmt.Errorf("Invalid Request Time out: %w", err)
	}
	cfg.RequestTimeout = timeout

	maxConc, err := strconv.Atoi(getEnv("MAX_CONCURRENCY", "5"))
	if err != nil || maxConc < 1 {
		return nil, fmt.Errorf("Invalid max concurrency: should be +ve int")
	}
	cfg.MaxConcurrency = maxConc

	return cfg, nil
}
