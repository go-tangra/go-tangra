#!/usr/bin/env bash
# Extract one service's history from go-freya into a fresh repository (plan Phase C1).
#
#   tools/split/extract.sh [--dry-run] [--workdir DIR] <service>
#
# Result: <workdir>/<repo-name>/ - a filtered clone whose root is services/<x>/ plus the
# service's specs/NNN-* directories, with binaries/junk and blobs > 1 MB stripped from all
# history and AI trailers removed from commit messages. Nothing is pushed.
# The clone is taken from the committed state of the go-freya HEAD branch.
set -euo pipefail
# shellcheck source=tools/split/lib.sh
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

workdir=""
svc=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --dry-run) DRY_RUN=1 ;;
    --workdir) workdir="$2"; shift ;;
    -h|--help) sed -n '2,10p' "$0"; exit 0 ;;
    -*) echo "unknown flag $1" >&2; exit 2 ;;
    *) svc="$1" ;;
  esac
  shift
done
[[ -n "$svc" ]] || { echo "usage: $0 [--dry-run] [--workdir DIR] <service>" >&2; exit 2; }

require_filter_repo

dir="$(render -service "$svc" -print dir)"
repo="$(render -service "$svc" -print repo)"
mapfile -t specs < <(render -service "$svc" -print specs)
name="${repo#*/}"

if [[ -z "$workdir" ]]; then
  if ((DRY_RUN)); then workdir="<tmpdir>"; else workdir="$(mktemp -d -t "split-$svc.XXXXXX")"; fi
fi
dest="$workdir/$name"

keep_args=(--path "$dir/")
for s in "${specs[@]}"; do
  [[ -d "$FREYA_ROOT/$s" ]] || echo "warning: spec dir $s not in the working tree (history may still contain it)" >&2
  keep_args+=(--path "$s/")
done

echo "== extract $svc: $dir + ${specs[*]} -> $dest ($repo)"
((DRY_RUN)) || [[ ! -e "$dest" ]] || { echo "$dest already exists" >&2; exit 1; }

run git clone --no-local "$FREYA_ROOT" "$dest"
# Pass 1: keep the service + its specs, move the service to the root, strip big blobs,
# clean commit messages.
run git -C "$dest" filter-repo --force \
  "${keep_args[@]}" \
  --path-rename "$dir/:" \
  --strip-blobs-bigger-than 1M \
  --message-callback "$MESSAGE_CALLBACK"
# Pass 2: drop binaries and generated junk from every commit (paths are root-relative now).
run git -C "$dest" filter-repo --force --invert-paths "${JUNK_FILTER_ARGS[@]}"

if ((DRY_RUN)); then
  run "$SPLIT_DIR/guard.sh" "$dest"
  exit 0
fi

git -C "$dest" gc --prune=now --quiet
echo "== $dest: $(git -C "$dest" rev-list --count HEAD) commits, $(du -sh "$dest/.git" | cut -f1) .git"
"$SPLIT_DIR/guard.sh" "$dest" || { echo "guard failed - fix before merging into $repo" >&2; exit 1; }
echo "next: tools/split/README.md step 2 (merge into $repo keeping the v3 branch)"
