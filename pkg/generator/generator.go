//
// Copyright (c) 2026 Red Hat, Inc.
// Licensed under the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package generator

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"text/template"
	"time"

	"github.com/tolusha/che-doc-generator/pkg/commands"
	"github.com/tolusha/che-doc-generator/pkg/config"
	"github.com/tolusha/che-doc-generator/pkg/github"
)

type Generator struct {
	ghClient     *github.Client
	timeout      time.Duration
	pollInterval time.Duration
	template     string
}

var (
	prURLPattern = regexp.MustCompile(`https://github\.com/eclipse-che/che-docs/pull/\d+`)
)

func New(
	ghClient *github.Client,
	cfg *config.Config,
) (*Generator, error) {
	data, err := os.ReadFile(cfg.PromptTemplatePath)
	if err != nil {
		return nil, fmt.Errorf("reading prompt template %w", err)
	}

	promptTemplate := strings.TrimSpace(string(data))
	if promptTemplate == "" {
		return nil, fmt.Errorf("prompt template %s is empty", cfg.PromptTemplatePath)
	}

	if !strings.Contains(promptTemplate, "{{.PRURL}}") {
		return nil, fmt.Errorf("prompt template %s must contain {{.PRURL}} placeholder", cfg.PromptTemplatePath)
	}

	return &Generator{
		ghClient:     ghClient,
		timeout:      cfg.GenerationTimeout,
		pollInterval: cfg.PollInterval,
		template:     promptTemplate,
	}, nil
}

func (g *Generator) Trigger(ctx context.Context, trigger *github.Trigger) {
	switch trigger.SubCommand {
	case commands.SubCommandDefault:
		g.handleGenerate(ctx, trigger)
	case commands.SubCommandHelp:
		g.handleHelp(ctx, trigger)
	default:
		g.handleUnknown(ctx, trigger)
	}
}

func (g *Generator) handleGenerate(ctx context.Context, trigger *github.Trigger) {
	log.Printf("[INFO] generating docs for %s/%s#%d", trigger.Owner, trigger.Repo, trigger.PRNumber)

	docPR, err := g.run(ctx, trigger)
	if err != nil {
		g.updateCommentWithFailureMessage(ctx, trigger, err)
		return
	}

	g.updateCommentWithDocPR(ctx, trigger, docPR)
}

func (g *Generator) updateCommentWithFailureMessage(
	ctx context.Context,
	trigger *github.Trigger,
	err error,
) {
	log.Printf("[ERROR] generation failed for %s/%s#%d: %v", trigger.Owner, trigger.Repo, trigger.PRNumber, err)

	msg := fmt.Sprintf("%s\n\nFailed to generate documentation.", trigger.CommentBody)
	if err := g.ghClient.UpdatePullRequestComment(
		ctx,
		trigger.Owner,
		trigger.Repo,
		trigger.CommentID,
		msg,
	); err != nil {
		log.Printf("[ERROR] error posting failure comment: %v", err)
	}
}

func (g *Generator) updateCommentWithDocPR(
	ctx context.Context,
	trigger *github.Trigger,
	docPR string,
) {
	log.Printf("docs generated for %s/%s#%d: %s", trigger.Owner, trigger.Repo, trigger.PRNumber, docPR)

	msg := fmt.Sprintf("%s\n\nDocumentation PR created: %s", trigger.CommentBody, docPR)
	if err := g.ghClient.UpdatePullRequestComment(
		ctx,
		trigger.Owner,
		trigger.Repo,
		trigger.CommentID,
		msg,
	); err != nil {
		log.Printf("[ERROR] error posting success comment: %v", err)
	}
}

func (g *Generator) run(ctx context.Context, trigger *github.Trigger) (string, error) {
	prompt, err := g.buildPrompt(trigger.PRURL)
	if err != nil {
		return "", err
	}

	log.Printf("[INFO] claude prompt:\n%s", prompt)

	ctx, cancel := context.WithTimeout(ctx, g.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "claude", "--dangerously-skip-permissions", "-p", prompt, "--output-format", "json")

	output, err := cmd.CombinedOutput()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return "", fmt.Errorf("timed out after %v", g.timeout)
	}

	if err != nil {
		return "", fmt.Errorf("claude exited with error: %w\noutput: %s", err, string(output))
	}

	log.Printf("claude output:\n%s", string(output))

	return parseDocPRURL(string(output))
}

func (g *Generator) handleHelp(ctx context.Context, trigger *github.Trigger) {
	if err := g.ghClient.PostPullRequestComment(
		ctx,
		trigger.Owner,
		trigger.Repo,
		trigger.PRNumber,
		commands.BuildWelcomeMessage(g.pollInterval),
	); err != nil {
		log.Printf("[ERROR] error posting help comment: %v", err)
	}
}

func (g *Generator) handleUnknown(_ context.Context, trigger *github.Trigger) {
	log.Printf("[WARN] unknown command %q on %s/%s#%d", trigger.SubCommand, trigger.Owner, trigger.Repo, trigger.PRNumber)
}

func (g *Generator) buildPrompt(prUrl string) (string, error) {
	tmpl, err := template.New("prompt").Parse(g.template)
	if err != nil {
		return "", fmt.Errorf("invalid prompt template: %w", err)
	}

	var buf strings.Builder
	data := map[string]string{"PRURL": prUrl}

	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("prompt template execution failed: %w", err)
	}

	return buf.String(), nil
}

func parseDocPRURL(output string) (string, error) {
	match := prURLPattern.FindString(output)
	if match == "" {
		return "", fmt.Errorf("no PR URL found in output")
	}

	return match, nil
}
