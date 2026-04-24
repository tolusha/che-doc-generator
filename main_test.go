package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func setTestPromptTemplate(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "prompt.tmpl")
	os.WriteFile(path, []byte("test prompt {{.PRURL}}"), 0644)
	t.Setenv("PROMPT_TEMPLATE", path)
}

func TestParseConfig_Defaults(t *testing.T) {
	t.Setenv("WATCH_REPOS", "org/repo1,org/repo2")
	setTestPromptTemplate(t)

	cfg, err := parseConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.WatchRepos) != 2 {
		t.Fatalf("expected 2 repos, got %d", len(cfg.WatchRepos))
	}
	if cfg.WatchRepos[0] != "org/repo1" {
		t.Errorf("expected org/repo1, got %s", cfg.WatchRepos[0])
	}
	if cfg.WatchRepos[1] != "org/repo2" {
		t.Errorf("expected org/repo2, got %s", cfg.WatchRepos[1])
	}
	if cfg.PollInterval != 10*time.Minute {
		t.Errorf("expected 10m default, got %v", cfg.PollInterval)
	}
	if cfg.GenerationTimeout != 1*time.Hour {
		t.Errorf("expected 1h default, got %v", cfg.GenerationTimeout)
	}
	if cfg.MaxConcurrent != 1 {
		t.Errorf("expected 1 default, got %d", cfg.MaxConcurrent)
	}
}

func TestParseConfig_CustomValues(t *testing.T) {
	t.Setenv("WATCH_REPOS", "org/repo1")
	t.Setenv("POLL_INTERVAL", "5m")
	t.Setenv("GENERATION_TIMEOUT", "1h")
	t.Setenv("MAX_CONCURRENT", "3")
	setTestPromptTemplate(t)

	cfg, err := parseConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.PollInterval != 5*time.Minute {
		t.Errorf("expected 5m, got %v", cfg.PollInterval)
	}
	if cfg.GenerationTimeout != time.Hour {
		t.Errorf("expected 1h, got %v", cfg.GenerationTimeout)
	}
	if cfg.MaxConcurrent != 3 {
		t.Errorf("expected 3, got %d", cfg.MaxConcurrent)
	}
}

func TestParseConfig_MissingRepos(t *testing.T) {
	_, err := parseConfig()
	if err == nil {
		t.Fatal("expected error when WATCH_REPOS is not set")
	}
}

func TestParseConfig_InvalidMaxConcurrent(t *testing.T) {
	t.Setenv("WATCH_REPOS", "org/repo1")
	t.Setenv("MAX_CONCURRENT", "0")
	setTestPromptTemplate(t)

	_, err := parseConfig()
	if err == nil {
		t.Fatal("expected error when MAX_CONCURRENT is 0")
	}
}

func TestParseConfig_MissingPromptTemplate(t *testing.T) {
	t.Setenv("WATCH_REPOS", "org/repo1")

	_, err := parseConfig()
	if err == nil {
		t.Fatal("expected error when PROMPT_TEMPLATE is not set")
	}
}

func TestParseConfig_TrimsWhitespace(t *testing.T) {
	t.Setenv("WATCH_REPOS", " org/repo1 , org/repo2 ")
	setTestPromptTemplate(t)

	cfg, err := parseConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.WatchRepos[0] != "org/repo1" {
		t.Errorf("expected trimmed org/repo1, got %q", cfg.WatchRepos[0])
	}
	if cfg.WatchRepos[1] != "org/repo2" {
		t.Errorf("expected trimmed org/repo2, got %q", cfg.WatchRepos[1])
	}
}
