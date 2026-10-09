#!/usr/bin/env bash
# Fails when any listed ghcr.io/go-tangra image cannot be pulled anonymously.
# ghcr creates a module's package private on its first push, whatever the
# repository's visibility, and no API can change that: a person must make it
# public in the package settings. Without this check a module host only finds
# out at `docker pull` ("unauthorized").
set -euo pipefail
list="${1:-.github/public-images.txt}"
private=()
while read -r image; do
  [[ -z "$image" || "$image" == \#* ]] && continue
  # A private (or missing) package refuses even the anonymous pull token.
  token=$(curl -sS "https://ghcr.io/token?scope=repository:go-tangra/${image}:pull" | sed -n 's/.*"token":"\([^"]*\)".*/\1/p' || true)
  code=403
  if [[ -n "$token" ]]; then
    code=$(curl -s -o /dev/null -w '%{http_code}' -H "Authorization: Bearer ${token}" "https://ghcr.io/v2/go-tangra/${image}/tags/list")
  fi
  if [[ "$code" == 200 ]]; then
    echo "public:  ghcr.io/go-tangra/${image}"
  else
    echo "PRIVATE: ghcr.io/go-tangra/${image} (HTTP ${code})"
    private+=("$image")
  fi
done < "$list"
if (( ${#private[@]} )); then
  echo
  echo "Make these packages public (Package settings > Danger Zone > Change visibility > Public):"
  for image in "${private[@]}"; do
    echo "  https://github.com/orgs/go-tangra/packages/container/${image}/settings"
    echo "::error title=Private image::ghcr.io/go-tangra/${image} cannot be pulled anonymously; make the package public: https://github.com/orgs/go-tangra/packages/container/${image}/settings"
  done
  exit 1
fi
