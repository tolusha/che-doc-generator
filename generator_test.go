package main

import (
	"strings"
	"testing"
	"time"
)

func TestBuildPrompt(t *testing.T) {
	gen := &Generator{Timeout: 30 * time.Minute}
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
	if !strings.Contains(prompt, "tolusha/claude-plugins") {
		t.Error("prompt should reference tolusha/claude-plugins")
	}
	if !strings.Contains(prompt, "redhat-docs-agent-tools") {
		t.Error("prompt should reference redhat-docs-agent-tools marketplace")
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

func TestBuildPrompt_ContainsAllSteps(t *testing.T) {
	gen := &Generator{Timeout: 30 * time.Minute}
	prompt := gen.BuildPrompt("https://github.com/org/repo/pull/1")

	required := []string{
		"start a DevWorkspace",
		"clone git@github.com:eclipse-che/che-docs.git",
		"/plugin install https://github.com/tolusha/claude-plugins",
		"/plugin marketplace add https://github.com/redhat-documentation/redhat-docs-agent-tools.git",
		"redhat-docs-agent-tools marketplace",
		"che-docs-from-pr",
		"Delete the DevWorkspace",
	}

	for _, r := range required {
		if !strings.Contains(prompt, r) {
			t.Errorf("prompt missing required content: %q", r)
		}
	}
}
