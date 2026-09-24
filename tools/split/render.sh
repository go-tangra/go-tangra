#!/usr/bin/env bash
# Render the standalone repo files for one service (or the platform) from services.yaml.
#
#   tools/split/render.sh ipam <repo-dir>        # Dockerfile, .dockerignore, .github/workflows/ci.yaml
#   tools/split/render.sh platform <repo-dir>    # .github/workflows/ci.yaml for go-tangra/go-tangra
#
# Runs from anywhere; uses the go-freya checkout that contains this script.
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
root="$(cd "$here/../.." && pwd)"

if [[ $# -lt 2 ]]; then
  echo "usage: $0 <service|platform> <out-dir> [extra render flags]" >&2
  exit 2
fi
target="$1"; out="$2"; shift 2
mkdir -p "$out"
out="$(cd "$out" && pwd)"

cd "$root"
if [[ "$target" == "platform" ]]; then
  GOWORK=off go run ./tools/split/cmd/render -platform -out "$out" "$@"
else
  GOWORK=off go run ./tools/split/cmd/render -service "$target" -out "$out" "$@"
fi
