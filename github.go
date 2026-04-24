package main

import (
	"context"
	"strings"

	"github.com/google/go-github/v68/github"
	"golang.org/x/oauth2"
)

const triggerPhrase = "/generate-che-doc"

type TriggerComment struct {
	Owner       string
	Repo        string
	PRNumber    int
	CommentID   int64
	PRURL       string
	CommentBody string
}

type GitHubClient struct {
	client *github.Client
}

func newGitHubClient(token string, baseURL ...string) *GitHubClient {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	httpClient := oauth2.NewClient(ctx, ts)

	client := github.NewClient(httpClient)
	if len(baseURL) > 0 && baseURL[0] != "" {
		// For testing: set BaseURL directly to the test server
		client.BaseURL, _ = client.BaseURL.Parse(baseURL[0] + "/")
	}

	return &GitHubClient{client: client}
}

func (g *GitHubClient) FindTriggerComments(owner, repo string) ([]TriggerComment, error) {
	ctx := context.Background()
	var triggers []TriggerComment

	prOpts := &github.PullRequestListOptions{
		State:       "open",
		ListOptions: github.ListOptions{PerPage: 100},
	}
	for {
		prs, resp, err := g.client.PullRequests.List(ctx, owner, repo, prOpts)
		if err != nil {
			return nil, err
		}

		for _, pr := range prs {
			commentOpts := &github.IssueListCommentsOptions{
				ListOptions: github.ListOptions{PerPage: 100},
			}
			for {
				comments, commentResp, err := g.client.Issues.ListComments(ctx, owner, repo, pr.GetNumber(), commentOpts)
				if err != nil {
					return nil, err
				}

				for _, comment := range comments {
					if !strings.Contains(comment.GetBody(), triggerPhrase) {
						continue
					}

					processed, err := g.hasEyesReaction(ctx, owner, repo, comment.GetID())
					if err != nil {
						return nil, err
					}
					if processed {
						continue
					}

					triggers = append(triggers, TriggerComment{
						Owner:       owner,
						Repo:        repo,
						PRNumber:    pr.GetNumber(),
						CommentID:   comment.GetID(),
						PRURL:       pr.GetHTMLURL(),
						CommentBody: comment.GetBody(),
					})
				}

				if commentResp.NextPage == 0 {
					break
				}
				commentOpts.Page = commentResp.NextPage
			}
		}

		if resp.NextPage == 0 {
			break
		}
		prOpts.Page = resp.NextPage
	}

	return triggers, nil
}

func (g *GitHubClient) hasEyesReaction(ctx context.Context, owner, repo string, commentID int64) (bool, error) {
	opts := &github.ListOptions{PerPage: 100}
	for {
		reactions, resp, err := g.client.Reactions.ListIssueCommentReactions(ctx, owner, repo, commentID, opts)
		if err != nil {
			return false, err
		}
		for _, r := range reactions {
			if r.GetContent() == "eyes" {
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

func (g *GitHubClient) AddEyesReaction(ctx context.Context, owner, repo string, commentID int64) error {
	_, _, err := g.client.Reactions.CreateIssueCommentReaction(ctx, owner, repo, commentID, "eyes")
	return err
}

func (g *GitHubClient) UpdateComment(ctx context.Context, owner, repo string, commentID int64, body string) error {
	_, _, err := g.client.Issues.EditComment(ctx, owner, repo, commentID, &github.IssueComment{
		Body: github.Ptr(body),
	})
	return err
}
