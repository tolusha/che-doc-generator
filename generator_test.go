package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const testTemplate = `You are a doc generator.
Analyze this PR: {{.PRURL}}
Use che-mcp-server to start a DevWorkspace.
Clone git@github.com:eclipse-che/che-docs.git
Delete the DevWorkspace when done.`

func TestBuildPrompt(t *testing.T) {
	gen := &Generator{Timeout: 30 * time.Minute, PromptTemplate: testTemplate}
	prompt := gen.BuildPrompt("https://github.com/org/repo/pull/42")

	if !strings.Contains(prompt, "https://github.com/org/repo/pull/42") {
		t.Error("prompt should contain the PR URL")
	}
	if !strings.Contains(prompt, "che-mcp-server") {
		t.Error("prompt should reference che-mcp-server")
	}
	if !strings.Contains(prompt, "che-docs.git") {
		t.Error("prompt should reference che-docs repo")
	}
	if !strings.Contains(prompt, "Delete the DevWorkspace") {
		t.Error("prompt should include workspace cleanup step")
	}
}

func TestParseDocPRURL_Success(t *testing.T) {
	output := `{"result": "I created the documentation PR at https://github.com/eclipse-che/che-docs/pull/99. The workspace has been deleted."}`

	url, err := parseDocPRURL(output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url != "https://github.com/eclipse-che/che-docs/pull/99" {
		t.Errorf("expected che-docs PR URL, got %s", url)
	}
}

func TestParseDocPRURL_NoPR(t *testing.T) {
	output := `{"result": "I tried but failed to generate docs."}`

	_, err := parseDocPRURL(output)
	if err == nil {
		t.Fatal("expected error when no PR URL in output")
	}
}

func TestParseDocPRURL_IgnoresSourcePR(t *testing.T) {
	output := `{"result": "I analyzed https://github.com/org/repo/pull/42 and created https://github.com/eclipse-che/che-docs/pull/55."}`

	url, err := parseDocPRURL(output)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url != "https://github.com/eclipse-che/che-docs/pull/55" {
		t.Errorf("expected che-docs PR URL, got %s", url)
	}
}

func TestBuildPrompt_SubstitutesURL(t *testing.T) {
	tmpl := "Generate docs for {{.PRURL}} now."
	gen := &Generator{Timeout: 30 * time.Minute, PromptTemplate: tmpl}
	prompt := gen.BuildPrompt("https://github.com/org/repo/pull/1")

	expected := "Generate docs for https://github.com/org/repo/pull/1 now."
	if prompt != expected {
		t.Errorf("got %q, want %q", prompt, expected)
	}
}

func TestLoadPromptTemplate(t *testing.T) {
	dir := t.TempDir()

	t.Run("valid", func(t *testing.T) {
		path := filepath.Join(dir, "valid.tmpl")
		os.WriteFile(path, []byte("Do something with {{.PRURL}}"), 0644)
		tmpl, err := loadPromptTemplate(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(tmpl, "{{.PRURL}}") {
			t.Error("template should contain placeholder")
		}
	})

	t.Run("missing file", func(t *testing.T) {
		_, err := loadPromptTemplate(filepath.Join(dir, "nope.tmpl"))
		if err == nil {
			t.Fatal("expected error for missing file")
		}
	})

	t.Run("empty", func(t *testing.T) {
		path := filepath.Join(dir, "empty.tmpl")
		os.WriteFile(path, []byte("  \n  "), 0644)
		_, err := loadPromptTemplate(path)
		if err == nil {
			t.Fatal("expected error for empty template")
		}
	})

	t.Run("no placeholder", func(t *testing.T) {
		path := filepath.Join(dir, "noplaceholder.tmpl")
		os.WriteFile(path, []byte("Do something without a URL"), 0644)
		_, err := loadPromptTemplate(path)
		if err == nil {
			t.Fatal("expected error when placeholder is missing")
		}
	})
}
