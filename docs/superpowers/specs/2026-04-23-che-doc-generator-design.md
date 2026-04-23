# che-doc-generator Design Spec

## Overview

A Go program that runs as a long-running Kubernetes pod, polling GitHub PRs for `@generate-che-doc` trigger comments. When triggered, it invokes Claude Code CLI to provision a DevWorkspace, install doc-generation plugins, and produce a documentation PR against `eclipse-che/che-docs`.

## Architecture

Two layers:

1. **PR Watcher (Go)** — Polls configured repos, detects triggers, deduplicates via emoji reactions, dispatches doc generation, reports results.
2. **Doc Generator (Claude Code CLI)** — Invoked as a subprocess with a crafted prompt. Uses che-mcp-server (pre-configured MCP tool) to manage DevWorkspaces. Installs plugins and runs the `che-docs-from-pr` skill to generate documentation.

```
┌─────────────────────────────────────┐
│         PR Watcher (Go Pod)         │
│                                     │
│  ┌───────────┐    ┌──────────────┐  │
│  │  Poller   │───▶│  Dispatcher  │  │
│  │ (per repo)│    │  (async,     │  │
│  └───────────┘    │  semaphore)  │  │
│                   └──────┬───────┘  │
│                          │          │
│                    ┌─────▼───────┐  │
│                    │ Claude Code │  │
│                    │   CLI       │  │
│                    └─────┬───────┘  │
│                          │          │
│                    ┌─────▼───────┐  │
│                    │che-mcp-server│  │
│                    │(DevWorkspace)│  │
│                    └─────────────┘  │
└─────────────────────────────────────┘
```

## Configuration

All configuration via environment variables:

| Variable | Default | Description |
|---|---|---|
| `WATCH_REPOS` | (required) | Comma-separated GitHub repos to watch (e.g., `org/repo1,org/repo2`) |
| `PROMPT_TEMPLATE` | (required) | Path to the prompt template file (Go `text/template` with `{{.PRURL}}` placeholder) |
| `POLL_INTERVAL` | `10m` | Polling interval (Go duration string) |
| `GENERATION_TIMEOUT` | `30m` | Max time for a Claude Code subprocess |
| `MAX_CONCURRENT` | `1` | Max parallel doc generations |

Authentication (GitHub token, Anthropic API key) is assumed to be pre-configured in the pod environment.

## Polling & Trigger Flow

Each poll cycle, for every configured repo:

1. List open PRs via GitHub API (`GET /repos/{owner}/{repo}/pulls?state=open`).
2. For each PR, fetch comments (`GET /repos/{owner}/{repo}/issues/{pr}/comments`).
3. For each comment containing `@generate-che-doc`:
   - Check if the comment already has an :eyes: reaction. If yes, skip.
   - React with :eyes: to claim the comment.
   - Dispatch doc generation asynchronously (subject to `MAX_CONCURRENT` semaphore).

## Deduplication

- The :eyes: reaction on a comment is the deduplication marker. A comment with :eyes: is never re-processed.
- On both success and failure, the reaction stays — preventing infinite retry loops.
- To retry, users post a new `@generate-che-doc` comment.

## Claude Code Invocation

On trigger, the Go program runs:

```bash
claude --dangerously-skip-permissions -p "<prompt>" --output-format json
```

The prompt is assembled from the PR URL and contains the full instruction sequence:

```
1. Using che-mcp-server, start a DevWorkspace.
2. In the DevWorkspace, clone git@github.com:eclipse-che/che-docs.git
3. Install plugins: /plugin install https://github.com/tolusha/claude-plugins
4. Add marketplace: /plugin marketplace add https://github.com/redhat-documentation/redhat-docs-agent-tools.git
5. Install plugin from redhat-docs-agent-tools marketplace (plugin name determined after marketplace is added)
6. Using che-docs-from-pr skill, generate documentation for PR: <pr-url>
7. Return the created doc PR URL.
8. Delete the DevWorkspace using che-mcp-server.
```

Output is parsed from JSON to extract the doc PR URL.

## Async Dispatch

Doc generation runs in a goroutine so the poller continues while generation is in progress. A semaphore (buffered channel of size `MAX_CONCURRENT`) limits parallelism.

## Error Handling & Reporting

**Success:**
- Post a comment on the source PR: `Documentation PR created: <doc-pr-url>`
- Log the success.

**Failure (non-zero exit, timeout, no PR URL in output):**
- Post a comment on the source PR: `Failed to generate documentation: <brief error summary>`
- Log full error details (stderr, exit code) to pod logs.

**Graceful shutdown:**
- Listen for SIGTERM.
- Wait for in-progress Claude Code subprocesses to finish (up to a grace period).
- Exit cleanly.

## GitHub API

Use `google/go-github` library for all GitHub interactions:
- List open PRs
- List issue comments
- List and create reactions
- Create issue comments

Handles pagination and rate limiting natively.

## Project Structure

```
che-doc-generator/
├── main.go              # Entry point, env config, polling loop, signal handling
├── github.go            # GitHub API client wrapper (PRs, comments, reactions)
├── generator.go         # Claude Code CLI invocation, prompt assembly, output parsing
├── Dockerfile           # Container image for K8s deployment
├── go.mod
└── go.sum
```

## Workspace Lifecycle

The DevWorkspace is created and destroyed within a single Claude Code session:
- Created at the start of the prompt via che-mcp-server.
- Deleted at the end of the prompt (step 8) via che-mcp-server.
- If the session fails mid-way, the workspace may be orphaned. This is acceptable — no separate cleanup mechanism is needed initially.
