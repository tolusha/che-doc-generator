package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	WatchRepos        []string
	PollInterval      time.Duration
	GenerationTimeout time.Duration
	MaxConcurrent     int
}

func parseConfig() (Config, error) {
	repos := os.Getenv("WATCH_REPOS")
	if repos == "" {
		return Config{}, fmt.Errorf("WATCH_REPOS environment variable is required")
	}

	pollInterval := 10 * time.Minute
	if v := os.Getenv("POLL_INTERVAL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid POLL_INTERVAL: %w", err)
		}
		pollInterval = d
	}

	genTimeout := 30 * time.Minute
	if v := os.Getenv("GENERATION_TIMEOUT"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid GENERATION_TIMEOUT: %w", err)
		}
		genTimeout = d
	}

	maxConcurrent := 1
	if v := os.Getenv("MAX_CONCURRENT"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid MAX_CONCURRENT: %w", err)
		}
		maxConcurrent = n
	}

	var repoList []string
	for _, r := range strings.Split(repos, ",") {
		r = strings.TrimSpace(r)
		if r != "" {
			repoList = append(repoList, r)
		}
	}

	return Config{
		WatchRepos:        repoList,
		PollInterval:      pollInterval,
		GenerationTimeout: genTimeout,
		MaxConcurrent:     maxConcurrent,
	}, nil
}

func main() {
	cfg, err := parseConfig()
	if err != nil {
		log.Fatalf("configuration error: %v", err)
	}
	log.Printf("watching repos: %v, poll interval: %v", cfg.WatchRepos, cfg.PollInterval)
}
