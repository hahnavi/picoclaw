---
name: github
description: "Interact with GitHub using the `gh` CLI. Use `gh issue`, `gh pr`, `gh run`, and `gh api` for issues, PRs, CI runs, and advanced queries."
emoji: "🐙"
primaryEnv: shell
os: [linux, darwin, windows]
requires:
  bins: [gh]
userInvocable: true
disableModelInvocation: false
---

# GitHub Skill

Use the `gh` CLI to interact with GitHub. Always specify `--repo owner/repo` when not in a git directory, or use URLs directly.

## Prerequisites

This skill requires the GitHub CLI (`gh`) to be installed. Install it via:
- **macOS**: `brew install gh`
- **Linux**: `sudo apt install gh` or equivalent
- **Windows**: `winget install --id GitHub.cli`

You also need to authenticate: `gh auth login`

## Pull Requests

Check CI status on a PR:
```bash
gh pr checks 55 --repo owner/repo
```

List recent workflow runs:
```bash
gh run list --repo owner/repo --limit 10
```

View a run and see which steps failed:
```bash
gh run view <run-id> --repo owner/repo
```

View logs for failed steps only:
```bash
gh run view <run-id> --repo owner/repo --log-failed
```

## Issues

List issues in a repository:
```bash
gh issue list --repo owner/repo --limit 20
```

View issue details:
```bash
gh issue view 123 --repo owner/repo
```

## API for Advanced Queries

The `gh api` command is useful for accessing data not available through other subcommands.

Get PR with specific fields:
```bash
gh api repos/owner/repo/pulls/55 --jq '.title, .state, .user.login'
```

## JSON Output

Most commands support `--json` for structured output. You can use `--jq` to filter:

```bash
gh issue list --repo owner/repo --json number,title --jq '.[] | "\(.number): \(.title)"'
```
