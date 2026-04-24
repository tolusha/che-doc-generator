package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/go-github/v68/github"
)

func TestFindTriggerComments_FindsUnprocessed(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /repos/org/repo/pulls", func(w http.ResponseWriter, r *http.Request) {
		prs := []*github.PullRequest{
			{Number: github.Ptr(1)},
		}
		json.NewEncoder(w).Encode(prs)
	})

	mux.HandleFunc("GET /repos/org/repo/issues/1/comments", func(w http.ResponseWriter, r *http.Request) {
		comments := []*github.IssueComment{
			{
				ID:   github.Ptr(int64(100)),
				Body: github.Ptr("please /generate-che-doc for this PR"),
			},
			{
				ID:   github.Ptr(int64(101)),
				Body: github.Ptr("just a regular comment"),
			},
		}
		json.NewEncoder(w).Encode(comments)
	})

	mux.HandleFunc("GET /repos/org/repo/issues/comments/100/reactions", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]*github.Reaction{})
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := newGitHubClient("fake-token", srv.URL)
	triggers, err := client.FindTriggerComments("org", "repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(triggers) != 1 {
		t.Fatalf("expected 1 trigger, got %d", len(triggers))
	}
	if triggers[0].CommentID != 100 {
		t.Errorf("expected comment ID 100, got %d", triggers[0].CommentID)
	}
	if triggers[0].PRNumber != 1 {
		t.Errorf("expected PR number 1, got %d", triggers[0].PRNumber)
	}
	if triggers[0].CommentBody != "please /generate-che-doc for this PR" {
		t.Errorf("expected comment body preserved, got %q", triggers[0].CommentBody)
	}
}

func TestFindTriggerComments_SkipsProcessed(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /repos/org/repo/pulls", func(w http.ResponseWriter, r *http.Request) {
		prs := []*github.PullRequest{
			{Number: github.Ptr(1)},
		}
		json.NewEncoder(w).Encode(prs)
	})

	mux.HandleFunc("GET /repos/org/repo/issues/1/comments", func(w http.ResponseWriter, r *http.Request) {
		comments := []*github.IssueComment{
			{
				ID:   github.Ptr(int64(100)),
				Body: github.Ptr("/generate-che-doc"),
			},
		}
		json.NewEncoder(w).Encode(comments)
	})

	mux.HandleFunc("GET /repos/org/repo/issues/comments/100/reactions", func(w http.ResponseWriter, r *http.Request) {
		reactions := []*github.Reaction{
			{Content: github.Ptr("eyes")},
		}
		json.NewEncoder(w).Encode(reactions)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := newGitHubClient("fake-token", srv.URL)
	triggers, err := client.FindTriggerComments("org", "repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(triggers) != 0 {
		t.Fatalf("expected 0 triggers (already processed), got %d", len(triggers))
	}
}

func TestUpdateComment(t *testing.T) {
	var updated bool
	mux := http.NewServeMux()

	mux.HandleFunc("PATCH /repos/org/repo/issues/comments/100", func(w http.ResponseWriter, r *http.Request) {
		updated = true
		var body github.IssueComment
		json.NewDecoder(r.Body).Decode(&body)
		if !strings.Contains(body.GetBody(), "Documentation PR created") {
			t.Error("updated body should contain new content")
		}
		json.NewEncoder(w).Encode(&github.IssueComment{ID: github.Ptr(int64(100))})
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := newGitHubClient("fake-token", srv.URL)
	err := client.UpdateComment(t.Context(), "org", "repo", 100, "Documentation PR created: https://example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !updated {
		t.Error("expected comment to be updated")
	}
}
