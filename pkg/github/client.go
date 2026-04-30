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

package github

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/google/go-github/v68/github"
	"github.com/tolusha/che-doc-generator/pkg/config"
	"golang.org/x/oauth2"

	"github.com/tolusha/che-doc-generator/pkg/commands"
)

type Trigger struct {
	Owner       string
	Repo        string
	PRNumber    int
	PRURL       string
	CommentID   int64
	CommentBody string
	SubCommand  commands.SubCommandType
}

type Client struct {
	client       *github.Client
	allowedUsers []string
	pollInterval time.Duration
}

const (
	eyesReaction = "eyes"
)

func New(cfg *config.Config) (*Client, error) {
	token := os.Getenv("CHE_DOC_GENERATOR_GITHUB_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("CHE_DOC_GENERATOR_GITHUB_TOKEN environment variable is required")
	}

	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	httpClient := oauth2.NewClient(context.Background(), tokenSource)
	client := github.NewClient(httpClient)

	return &Client{client: client, allowedUsers: cfg.AllowedUsers, pollInterval: cfg.PollInterval}, nil
}

func (g *Client) FindTriggerComment(
	ctx context.Context,
	owner, repo string,
	comments []*github.IssueComment,
	pullRequest *github.PullRequest,
) (*Trigger, error) {
	for i := len(comments) - 1; i >= 0; i-- {
		comment := comments[i]

		ok, subCommand := commands.Parse(comment.GetBody())
		if !ok {
			continue
		}

		if !g.IsIssueCommentAuthorEligible(comment) {
			continue
		}

		hasEyeReaction, err := g.HasIssueCommentEyesReaction(ctx, owner, repo, comment.GetID())
		if err != nil {
			return nil, err
		}

		// Already triggered for this PR
		if hasEyeReaction {
			break
		}

		return &Trigger{
			Owner:       owner,
			Repo:        repo,
			CommentID:   comment.GetID(),
			PRNumber:    pullRequest.GetNumber(),
			PRURL:       pullRequest.GetHTMLURL(),
			CommentBody: comment.GetBody(),
			SubCommand:  subCommand,
		}, nil
	}

	return nil, nil
}

func (g *Client) GetPullRequests(
	ctx context.Context,
	owner, repo string,
) ([]*github.PullRequest, error) {

	var result []*github.PullRequest

	opts := &github.PullRequestListOptions{
		State:       "open",
		ListOptions: github.ListOptions{PerPage: 100},
	}

	for {
		pullRequests, response, err := g.client.PullRequests.List(ctx, owner, repo, opts)
		if err != nil {
			return nil, err
		}

		result = append(result, pullRequests...)

		if response.NextPage == 0 {
			break
		}
		opts.Page = response.NextPage
	}

	return result, nil
}

func (g *Client) GetComments(
	ctx context.Context,
	owner, repo string,
	pullRequestNumber int,
) ([]*github.IssueComment, error) {

	var result []*github.IssueComment

	opts := &github.IssueListCommentsOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}

	for {
		comments, resp, err := g.client.Issues.ListComments(ctx, owner, repo, pullRequestNumber, opts)
		if err != nil {
			return nil, err
		}

		result = append(result, comments...)

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return result, nil
}

func (g *Client) PostWelcomeComment(
	ctx context.Context,
	owner, repo string,
	pullRequest *github.PullRequest,
) error {
	return g.PostPullRequestComment(
		ctx,
		owner,
		repo,
		pullRequest.GetNumber(),
		commands.BuildWelcomeMessage(g.pollInterval),
	)
}

func (g *Client) IsPullRequestAuthorEligible(pullRequest *github.PullRequest) bool {
	return slices.Contains(g.allowedUsers, pullRequest.GetUser().GetLogin())
}

func (g *Client) IsIssueCommentAuthorEligible(comment *github.IssueComment) bool {
	return slices.Contains(g.allowedUsers, comment.GetUser().GetLogin())
}

func (g *Client) HasWelcomeComment(comments []*github.IssueComment) bool {
	return slices.ContainsFunc(comments, func(c *github.IssueComment) bool {
		return strings.Contains(c.GetBody(), commands.WelcomeMarker)
	})
}

func (g *Client) PostPullRequestComment(
	ctx context.Context,
	owner, repo string,
	pullRequestNumber int,
	body string,
) error {
	_, _, err := g.client.Issues.CreateComment(
		ctx,
		owner,
		repo,
		pullRequestNumber,
		&github.IssueComment{
			Body: github.Ptr(body),
		},
	)

	return err
}

func (g *Client) UpdatePullRequestComment(
	ctx context.Context,
	owner, repo string,
	commentID int64,
	body string,
) error {
	_, _, err := g.client.Issues.EditComment(
		ctx,
		owner,
		repo,
		commentID,
		&github.IssueComment{
			Body: github.Ptr(body),
		},
	)

	return err
}

func (g *Client) AddIssueCommentEyesReaction(
	ctx context.Context,
	owner, repo string,
	commentID int64,
) error {
	_, _, err := g.client.Reactions.CreateIssueCommentReaction(
		ctx,
		owner,
		repo,
		commentID,
		eyesReaction,
	)

	return err
}

func (g *Client) HasIssueCommentEyesReaction(
	ctx context.Context,
	owner, repo string,
	commentID int64,
) (bool, error) {
	opts := &github.ListOptions{PerPage: 100}

	for {
		reactions, resp, err := g.client.Reactions.ListIssueCommentReactions(ctx, owner, repo, commentID, opts)
		if err != nil {
			return false, err
		}

		for _, r := range reactions {
			if r.GetContent() == eyesReaction {
				return true, nil
			}
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return false, nil
}
