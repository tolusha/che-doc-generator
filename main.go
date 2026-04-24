package main

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type Config struct {
	WatchRepos        []string
	PollInterval      time.Duration
	GenerationTimeout time.Duration
	MaxConcurrent     int
	PromptTemplate    string
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

	genTimeout := 1 * time.Hour
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
		if n <= 0 {
			return Config{}, fmt.Errorf("MAX_CONCURRENT must be positive, got %d", n)
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

	promptFile := os.Getenv("PROMPT_TEMPLATE")
	if promptFile == "" {
		return Config{}, fmt.Errorf("PROMPT_TEMPLATE environment variable is required")
	}
	promptTemplate, err := loadPromptTemplate(promptFile)
	if err != nil {
		return Config{}, err
	}

	return Config{
		WatchRepos:        repoList,
		PollInterval:      pollInterval,
		GenerationTimeout: genTimeout,
		MaxConcurrent:     maxConcurrent,
		PromptTemplate:    promptTemplate,
	}, nil
}

func main() {
	cfg, err := parseConfig()
	if err != nil {
		log.Fatalf("configuration error: %v", err)
	}

	ghToken := os.Getenv("GITHUB_TOKEN")
	if ghToken == "" {
		log.Fatal("GITHUB_TOKEN environment variable is required")
	}
	ghClient := newGitHubClient(ghToken)
	gen := &Generator{Timeout: cfg.GenerationTimeout, PromptTemplate: cfg.PromptTemplate}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	sem := make(chan struct{}, cfg.MaxConcurrent)
	var wg sync.WaitGroup

	log.Printf("starting che-doc-generator: watching %v, poll every %v", cfg.WatchRepos, cfg.PollInterval)

	ticker := time.NewTicker(cfg.PollInterval)
	defer ticker.Stop()

	poll := func() {
		for _, repo := range cfg.WatchRepos {
			slug := repo
			if u, err := url.Parse(slug); err == nil && u.Host != "" {
				slug = strings.TrimPrefix(u.Path, "/")
			}
			parts := strings.SplitN(slug, "/", 2)
			if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
				log.Printf("invalid repo format: %s (expected owner/repo or https://github.com/owner/repo)", repo)
				continue
			}
			owner, repoName := parts[0], parts[1]

			triggers, err := ghClient.FindTriggerComments(owner, repoName)
			if err != nil {
				log.Printf("error scanning %s: %v", repo, err)
				continue
			}

			for _, trigger := range triggers {
				if err := ghClient.AddEyesReaction(ctx, trigger.Owner, trigger.Repo, trigger.CommentID); err != nil {
					log.Printf("error adding reaction to comment %d: %v", trigger.CommentID, err)
					continue
				}

				wg.Add(1)
				go func(t TriggerComment) {
					defer wg.Done()
					select {
					case sem <- struct{}{}:
						defer func() { <-sem }()
					case <-ctx.Done():
						return
					}

					log.Printf("generating docs for %s/%s#%d", t.Owner, t.Repo, t.PRNumber)
					docPRURL, err := gen.Run(ctx, t.PRURL)
					if err != nil {
						log.Printf("generation failed for %s/%s#%d: %v", t.Owner, t.Repo, t.PRNumber, err)
						msg := "Failed to generate documentation. See pod logs for details."
						if commentErr := ghClient.UpsertComment(ctx, t.Owner, t.Repo, t.PRNumber, msg); commentErr != nil {
							log.Printf("error posting failure comment: %v", commentErr)
						}
						return
					}

					log.Printf("docs generated for %s/%s#%d: %s", t.Owner, t.Repo, t.PRNumber, docPRURL)
					msg := fmt.Sprintf("Documentation PR created: %s", docPRURL)
					if commentErr := ghClient.UpsertComment(ctx, t.Owner, t.Repo, t.PRNumber, msg); commentErr != nil {
						log.Printf("error posting success comment: %v", commentErr)
					}
				}(trigger)
			}
		}
	}

	poll()

	for {
		select {
		case <-ticker.C:
			poll()
		case <-sigCh:
			log.Println("shutdown signal received, waiting for in-progress generations...")
			cancel()
			wg.Wait()
			log.Println("shutdown complete")
			return
		}
	}
}
