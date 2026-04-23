package main

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"time"
)

type Generator struct {
	Timeout time.Duration
}

func (g *Generator) BuildPrompt(prURL string) string {
	return fmt.Sprintf(`You are an automated documentation generator. Follow these steps exactly:

1. Using che-mcp-server, start a DevWorkspace.
2. In the DevWorkspace, clone git@github.com:eclipse-che/che-docs.git
3. Install plugins: /plugin install https://github.com/tolusha/claude-plugins
4. Add marketplace: /plugin marketplace add https://github.com/redhat-documentation/redhat-docs-agent-tools.git
5. Install the plugin from redhat-docs-agent-tools marketplace.
6. Using the che-docs-from-pr skill, generate documentation for this PR: %s
7. Return ONLY the created documentation PR URL on a line by itself.
8. Delete the DevWorkspace using che-mcp-server.`, prURL)
}

func (g *Generator) Run(ctx context.Context, prURL string) (string, error) {
	prompt := g.BuildPrompt(prURL)

	ctx, cancel := context.WithTimeout(ctx, g.Timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "claude",
		"--dangerously-skip-permissions",
		"-p", prompt,
		"--output-format", "json",
	)

	output, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("timed out after %v", g.Timeout)
	}
	if err != nil {
		return "", fmt.Errorf("claude exited with error: %w\noutput: %s", err, string(output))
	}

	return parseDocPRURL(string(output))
}

var prURLPattern = regexp.MustCompile(`https://github\.com/eclipse-che/che-docs/pull/\d+`)

func parseDocPRURL(output string) (string, error) {
	match := prURLPattern.FindString(output)
	if match == "" {
		return "", fmt.Errorf("no PR URL found in output")
	}
	return match, nil
}
