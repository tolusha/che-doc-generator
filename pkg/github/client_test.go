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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	gh "github.com/google/go-github/v68/github"

	"github.com/tolusha/che-doc-generator/pkg/commands"
)

func newTestClient(allowedUsers []string, baseURL string) *Client {
	c := gh.NewClient(nil)
	c.BaseURL, _ = c.BaseURL.Parse(baseURL + "/")

	return &Client{
		client:       c,
		allowedUsers: allowedUsers,
	}
}

func TestFindTriggerComment_FindsUnprocessed(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /repos/org/repo/issues/comments/100/reactions", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]*gh.Reaction{})
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := newTestClient([]string{"alice"}, srv.URL)

	pr := &gh.PullRequest{
		Number:  gh.Ptr(1),
		HTMLURL: gh.Ptr("https://github.com/org/repo/pull/1"),
	}
	comments := []*gh.IssueComment{
		{
			ID:   gh.Ptr(int64(100)),
			Body: gh.Ptr("/generate-che-doc"),
			User: &gh.User{Login: gh.Ptr("alice")},
		},
		{
			ID:   gh.Ptr(int64(101)),
			Body: gh.Ptr("just a regular comment"),
			User: &gh.User{Login: gh.Ptr("bob")},
		},
	}

	trigger, err := client.FindTriggerComment(context.Background(), "org", "repo", comments, pr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if trigger == nil {
		t.Fatal("expected a trigger, got nil")
	}
	if trigger.CommentID != 100 {
		t.Errorf("expected comment ID 100, got %d", trigger.CommentID)
	}
	if trigger.PRNumber != 1 {
		t.Errorf("expected PR number 1, got %d", trigger.PRNumber)
	}
	if trigger.CommentBody != "/generate-che-doc" {
		t.Errorf("expected comment body preserved, got %q", trigger.CommentBody)
	}
	if trigger.SubCommand != commands.SubCommandDefault {
		t.Errorf("expected generate command for bare trigger, got %q", trigger.SubCommand)
	}
}

func TestFindTriggerComment_ParsesSubcommand(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /repos/org/repo/issues/comments/100/reactions", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]*gh.Reaction{})
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := newTestClient([]string{"alice"}, srv.URL)

	pr := &gh.PullRequest{Number: gh.Ptr(1)}
	comments := []*gh.IssueComment{
		{
			ID:   gh.Ptr(int64(100)),
			Body: gh.Ptr("/generate-che-doc help"),
			User: &gh.User{Login: gh.Ptr("alice")},
		},
	}

	trigger, err := client.FindTriggerComment(context.Background(), "org", "repo", comments, pr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if trigger == nil {
		t.Fatal("expected a trigger, got nil")
	}
	if trigger.SubCommand != commands.SubCommandHelp {
		t.Errorf("expected help command, got %q", trigger.SubCommand)
	}
}

func TestFindTriggerComment_SkipsProcessed(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /repos/org/repo/issues/comments/100/reactions", func(w http.ResponseWriter, r *http.Request) {
		reactions := []*gh.Reaction{
			{Content: gh.Ptr("eyes")},
		}
		json.NewEncoder(w).Encode(reactions)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := newTestClient([]string{"alice"}, srv.URL)

	pr := &gh.PullRequest{Number: gh.Ptr(1)}
	comments := []*gh.IssueComment{
		{
			ID:   gh.Ptr(int64(100)),
			Body: gh.Ptr("/generate-che-doc"),
			User: &gh.User{Login: gh.Ptr("alice")},
		},
	}

	trigger, err := client.FindTriggerComment(context.Background(), "org", "repo", comments, pr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if trigger != nil {
		t.Fatalf("expected nil trigger (already processed), got %+v", trigger)
	}
}

func TestFindTriggerComment_SkipsUnauthorizedUser(t *testing.T) {
	srv := httptest.NewServer(http.NewServeMux())
	defer srv.Close()

	client := newTestClient([]string{"alice", "bob"}, srv.URL)

	pr := &gh.PullRequest{Number: gh.Ptr(1)}
	comments := []*gh.IssueComment{
		{
			ID:   gh.Ptr(int64(100)),
			Body: gh.Ptr("/generate-che-doc"),
			User: &gh.User{Login: gh.Ptr("mallory")},
		},
	}

	trigger, err := client.FindTriggerComment(context.Background(), "org", "repo", comments, pr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if trigger != nil {
		t.Fatalf("expected nil trigger (unauthorized user), got %+v", trigger)
	}
}

func TestIsPullRequestAuthorEligible(t *testing.T) {
	client := newTestClient([]string{"alice", "bob"}, "http://unused")

	allowed := &gh.PullRequest{User: &gh.User{Login: gh.Ptr("alice")}}
	if !client.IsPullRequestAuthorEligible(allowed) {
		t.Error("expected alice to be eligible")
	}

	denied := &gh.PullRequest{User: &gh.User{Login: gh.Ptr("mallory")}}
	if client.IsPullRequestAuthorEligible(denied) {
		t.Error("expected mallory to not be eligible")
	}
}

func TestHasBotComment(t *testing.T) {
	client := newTestClient(nil, "http://unused")

	withMarker := []*gh.IssueComment{
		{Body: gh.Ptr("regular comment")},
		{Body: gh.Ptr(commands.BuildWelcomeMessage(5 * time.Minute))},
	}
	if !client.HasWelcomeComment(withMarker) {
		t.Error("expected bot comment to be found")
	}

	withoutMarker := []*gh.IssueComment{
		{Body: gh.Ptr("regular comment")},
	}
	if client.HasWelcomeComment(withoutMarker) {
		t.Error("expected no bot comment")
	}
}

func TestPostWelcomeComment(t *testing.T) {
	var posted bool
	mux := http.NewServeMux()

	mux.HandleFunc("POST /repos/org/repo/issues/1/comments", func(w http.ResponseWriter, r *http.Request) {
		posted = true
		var body gh.IssueComment
		json.NewDecoder(r.Body).Decode(&body)
		if !strings.Contains(body.GetBody(), commands.WelcomeMarker) {
			t.Error("welcome comment should contain marker")
		}
		json.NewEncoder(w).Encode(&gh.IssueComment{ID: gh.Ptr(int64(200))})
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := newTestClient([]string{"alice"}, srv.URL)
	pr := &gh.PullRequest{Number: gh.Ptr(1)}
	err := client.PostWelcomeComment(context.Background(), "org", "repo", pr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !posted {
		t.Error("expected welcome comment to be posted")
	}
}

func TestUpdatePullRequestComment(t *testing.T) {
	var updated bool
	mux := http.NewServeMux()

	mux.HandleFunc("PATCH /repos/org/repo/issues/comments/100", func(w http.ResponseWriter, r *http.Request) {
		updated = true
		var body gh.IssueComment
		json.NewDecoder(r.Body).Decode(&body)
		if !strings.Contains(body.GetBody(), "Documentation PR created") {
			t.Error("updated body should contain new content")
		}
		json.NewEncoder(w).Encode(&gh.IssueComment{ID: gh.Ptr(int64(100))})
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := newTestClient(nil, srv.URL)
	err := client.UpdatePullRequestComment(context.Background(), "org", "repo", 100, "Documentation PR created: https://example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !updated {
		t.Error("expected comment to be updated")
	}
}
