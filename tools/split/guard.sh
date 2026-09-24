#!/usr/bin/env bash
# Pre-push guard for a go-tangra v4 repository (plan Phase C4). Run before every push.
#
#   tools/split/guard.sh <repo-dir> [base-ref]
#
# Fails (exit 1) and lists every finding when:
#   1. a tracked file is larger than 1 MB, or has binary MIME encoding
#      (`file --mime-encoding` = binary) and is not on the explicit allow-list below;
#   2. a tracked path matches a junk pattern (node_modules, dist, coverage output,
#      *.test, bin/, go.work);
#   3. a blob introduced in base..HEAD is larger than 1 MB or has a junk path
#      (catches files that were added and later deleted - they would still be pushed);
#   4. a commit message in base..HEAD contains "co-authored-by" or "claude"
#      (case-insensitive).
# Without base-ref the whole history reachable from HEAD is checked.
set -euo pipefail

MAX_BYTES=$((1024 * 1024))
# Allow-listed baselines (screenshots) may be up to 2 MB; nothing else over 1 MB.
MAX_ALLOWED_BYTES=$((2 * 1024 * 1024))

# Binary files that are allowed (extended regex on the repo-relative path). Keep explicit.
BINARY_ALLOW=(
  '(^|/)testdata/.+\.(png|jpe?g|webp|gif|bin|txt)$'   # fuzz / fixture inputs
  '(^|/)__screenshots__/.+\.png$'                     # Playwright visual baselines
)

# Junk path patterns (extended regex on the repo-relative path).
JUNK=(
  '(^|/)node_modules/'
  '(^|/)dist(-remote)?/'
  '(^|/)coverage/'
  '(^|/)coverage\.out$'
  '\.test$'
  '(^|/)bin/'
  '(^|/)go\.work(\.sum)?$'
)
# Paths that match a junk pattern but are legitimate sources (explicit).
JUNK_ALLOW=(
  '^ui/kit/bin/[^/]+\.mjs$'   # @go-tangra/ui npm "bin" scripts (platform repo)
)

usage() { echo "usage: $0 <repo-dir> [base-ref]" >&2; exit 2; }
[[ $# -ge 1 && $# -le 2 ]] || usage
repo="$1"
base="${2:-}"
git -C "$repo" rev-parse --git-dir >/dev/null 2>&1 || { echo "guard: $repo is not a git repository" >&2; exit 2; }
command -v file >/dev/null || { echo "guard: the 'file' command is required" >&2; exit 2; }

if [[ -n "$base" ]]; then
  git -C "$repo" rev-parse --verify --quiet "$base^{commit}" >/dev/null || { echo "guard: unknown base ref $base" >&2; exit 2; }
  range="$base..HEAD"
else
  range="HEAD"
fi

matches_any() { # <path> <regex>...
  local p="$1"; shift
  local re
  for re in "$@"; do
    [[ "$p" =~ $re ]] && return 0
  done
  return 1
}

is_junk() { matches_any "$1" "${JUNK[@]}" && ! matches_any "$1" "${JUNK_ALLOW[@]}"; }

findings=()
add() { findings+=("$1"); }

# 1 + 2: tracked files (index).
tracked=()
while IFS= read -r -d '' p; do tracked+=("$p"); done < <(git -C "$repo" ls-files -z)

if ((${#tracked[@]})); then
  # sizes of the index blobs ("<mode> <sha> <stage>\t<path>" -> "<sha> <path>")
  while read -r _ size path; do
    limit=$MAX_BYTES
    if matches_any "$path" "${BINARY_ALLOW[@]}"; then limit=$MAX_ALLOWED_BYTES; fi
    if ((size > limit)); then add "LARGE     $path ($size bytes)"; fi
  done < <(git -C "$repo" ls-files -s \
            | awk -F'\t' '{split($1, a, " "); print a[2] " " $2}' \
            | git -C "$repo" cat-file --batch-check='%(objectname) %(objectsize) %(rest)')

  for p in "${tracked[@]}"; do
    if is_junk "$p"; then add "JUNK-PATH $p"; fi
  done

  # binary MIME (working tree content of tracked files; empty files are ignored)
  while IFS=$'\t' read -r p enc; do
    enc="${enc# }"
    [[ "$enc" == "binary" ]] || continue
    [[ -s "$repo/$p" ]] || continue
    matches_any "$p" "${BINARY_ALLOW[@]}" && continue
    add "BINARY    $p"
  done < <(cd "$repo" && printf '%s\0' "${tracked[@]}" | xargs -0 -r file -N -F $'\t' --mime-encoding --)
fi

# 3: blobs introduced in the range (history, including later-deleted files).
while read -r type size path; do
  [[ "$type" == "blob" ]] || continue
  limit=$MAX_BYTES
  if matches_any "$path" "${BINARY_ALLOW[@]}"; then limit=$MAX_ALLOWED_BYTES; fi
  if ((size > limit)); then add "HIST-LARGE $path ($size bytes, in $range)"; fi
  if [[ -n "$path" ]] && is_junk "$path"; then add "HIST-JUNK  $path (in $range)"; fi
done < <(git -C "$repo" rev-list --objects "$range" \
          | git -C "$repo" cat-file --batch-check='%(objecttype) %(objectsize) %(rest)')

# 4: commit messages.
while IFS= read -r -d '' rec; do
  rec="${rec#$'\n'}"   # tformat terminates each record with a newline after the NUL
  sha="${rec%%$'\n'*}"
  body="${rec#*$'\n'}"
  while IFS= read -r line; do
    add "MESSAGE   ${sha:0:12}: $line"
  done < <(grep -iE 'co-authored-by|claude' <<<"$body" || true)
done < <(git -C "$repo" log --format='%H%n%B%x00' "$range")

if ((${#findings[@]})); then
  echo "guard: FAILED - ${#findings[@]} finding(s) in $repo (range $range):" >&2
  printf '  %s\n' "${findings[@]}" | sort -u >&2
  exit 1
fi
echo "guard: OK - $repo (range $range): no large/binary/junk files, no AI trailers"
