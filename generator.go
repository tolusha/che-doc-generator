package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"text/template"
	"time"
)

type Generator struct {
	Timeout      time.Duration
	PromptTemplate string
}

func (g *Generator) BuildPrompt(prURL, notes string) string {
	tmpl, err := template.New("prompt").Parse(g.PromptTemplate)
	if err != nil {
		panic(fmt.Sprintf("invalid prompt template: %v", err))
	}
	var buf strings.Builder
	data := map[string]string{"PRURL": prURL, "Notes": notes}
	if err := tmpl.Execute(&buf, data); err != nil {
		panic(fmt.Sprintf("prompt template execution failed: %v", err))
	}
	return buf.String()
}

func loadPromptTemplate(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading prompt template %s: %w", path, err)
	}
	content := strings.TrimSpace(string(data))
	if content == "" {
		return "", fmt.Errorf("prompt template %s is empty", path)
	}
	if !strings.Contains(content, "{{.PRURL}}") {
		return "", fmt.Errorf("prompt template %s must contain {{.PRURL}} placeholder", path)
	}
	return content, nil
}

func (g *Generator) Run(ctx context.Context, prURL, notes string) (string, error) {
	prompt := g.BuildPrompt(prURL, notes)
	log.Printf("claude prompt:\n%s", prompt)

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
