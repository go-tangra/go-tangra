#!/usr/bin/env bash
# Shared helpers for extract.sh / extract-platform.sh (sourced, not executed).
# shellcheck shell=bash

SPLIT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FREYA_ROOT="$(cd "$SPLIT_DIR/../.." && pwd)"

# Commit-message cleanup used by every extraction: drop Co-Authored-By trailers and
# "Generated with [Claude Code]" lines (case-insensitive), trim trailing blank lines.
# shellcheck disable=SC2016  # python source, not shell
MESSAGE_CALLBACK='import re
kept = [l for l in message.split(b"\n")
        if not re.match(rb"(?i)^\s*co-authored-by:", l)
        and not re.search(rb"(?i)generated with \[claude code\]", l)]
return b"\n".join(kept).rstrip() + b"\n"'

# Junk removed from every extracted history (second filter-repo pass, --invert-paths,
# paths are already repo-root relative). Keep in sync with guard.sh JUNK.
JUNK_FILTER_ARGS=(
  --path-glob '*.test'
  --path-regex '(^|.*/)coverage\.out$'
  --path-regex '(^|.*/)coverage/.*'
  --path-regex '(^|.*/)bin/.*'
  --path-regex '(^|.*/)node_modules/.*'
  --path-regex '(^|.*/)dist(-remote)?/.*'
  --path-regex '(^|.*/)(test-results|playwright-report)/.*'
  --path-regex '^[^/]+svc$'
  --path assetsvc
  --path inventory-agent
  --path inventory-agent.exe
  --path lcm-devca
  --path go.work
  --path go.work.sum
)

DRY_RUN=0

# run <cmd...>: execute, or print shell-quoted when DRY_RUN=1.
run() {
  if ((DRY_RUN)); then
    printf '+'
    printf ' %q' "$@"
    printf '\n'
  else
    "$@"
  fi
}

require_filter_repo() {
  if git filter-repo --version >/dev/null 2>&1; then
    return 0
  fi
  cat >&2 <<'MSG'
git filter-repo is not installed. Install it with one of:
  pipx install git-filter-repo
  pip install --user git-filter-repo
(then make sure ~/.local/bin is on PATH and `git filter-repo --version` works)
MSG
  if ((DRY_RUN)); then
    echo "(dry-run: continuing without it)" >&2
    return 0
  fi
  return 1
}

render() { (cd "$FREYA_ROOT" && GOWORK=off go run ./tools/split/cmd/render "$@"); }
