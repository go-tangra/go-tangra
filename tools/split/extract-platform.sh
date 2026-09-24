#!/usr/bin/env bash
# Extract the platform history (framework, freyatest, contrib, ui, deploy, docs, scripts,
# tools, platform specs, .specify, root tooling files) into a fresh repository for
# go-tangra/go-tangra (plan Phase B).
#
#   tools/split/extract-platform.sh [--dry-run] [--workdir DIR]
#
# Everything is kept except: services/, .claude/, spec-router.json, .spec-router/,
# specs/* other than the platform specs listed in services.yaml (001, 013), go.work(.sum)
# and the usual binaries/junk; blobs > 1 MB are stripped; AI trailers are removed.
set -euo pipefail
# shellcheck source=tools/split/lib.sh
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

workdir=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --dry-run) DRY_RUN=1 ;;
    --workdir) workdir="$2"; shift ;;
    -h|--help) sed -n '2,12p' "$0"; exit 0 ;;
    *) echo "unknown argument $1" >&2; exit 2 ;;
  esac
  shift
done

require_filter_repo

repo="$(render -platform -print repo)"
mapfile -t specs < <(render -platform -print specs)
name="${repo#*/}"
if [[ -z "$workdir" ]]; then
  if ((DRY_RUN)); then workdir="<tmpdir>"; else workdir="$(mktemp -d -t split-platform.XXXXXX)"; fi
fi
dest="$workdir/$name"

# specs/ entries to drop = every spec dir ever in history that is not a platform spec.
keep_re=""
for s in "${specs[@]}"; do keep_re+="${keep_re:+|}${s#specs/}"; done
drop_args=(
  --path services/
  --path .claude/
  --path spec-router.json
  --path .spec-router/
  --path-regex "^specs/(?!(${keep_re})/).*"
)

echo "== extract platform -> $dest ($repo); platform specs: ${specs[*]}"
((DRY_RUN)) || [[ ! -e "$dest" ]] || { echo "$dest already exists" >&2; exit 1; }

run git clone --no-local "$FREYA_ROOT" "$dest"
# Pass 1: drop the services, agent tooling and non-platform specs; strip big blobs; clean messages.
run git -C "$dest" filter-repo --force --invert-paths \
  "${drop_args[@]}" \
  --strip-blobs-bigger-than 2M \
  --message-callback "$MESSAGE_CALLBACK"
# Pass 2: binaries / generated junk. ui/kit/bin/ holds npm bin scripts, so bin/ is matched
# only outside ui/kit here.
platform_junk=()
for a in "${JUNK_FILTER_ARGS[@]}"; do
  if [[ "$a" == '(^|.*/)bin/.*' ]]; then a='^(?!ui/kit/)(.*/)?bin/.*'; fi
  platform_junk+=("$a")
done
run git -C "$dest" filter-repo --force --invert-paths "${platform_junk[@]}"

if ((DRY_RUN)); then
  run "$SPLIT_DIR/guard.sh" "$dest"
  exit 0
fi

git -C "$dest" gc --prune=now --quiet
echo "== $dest: $(git -C "$dest" rev-list --count HEAD) commits, $(du -sh "$dest/.git" | cut -f1) .git"
"$SPLIT_DIR/guard.sh" "$dest" || { echo "guard failed - fix before pushing $repo" >&2; exit 1; }
echo "next: tools/split/README.md 'Platform' section"
