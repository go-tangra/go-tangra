#!/usr/bin/env bash
# One-command bring-up of the containerized freya platform stack.
# If docker needs a group switch on this host, run:  sg docker -c 'deploy/stack/up.sh'
set -euo pipefail
cd "$(dirname "$0")/../.."
export OPERATOR_EMAIL="${OPERATOR_EMAIL:-admin@example.org}"
# The dns service restarts the PowerDNS containers through the Docker socket;
# it joins the socket's group (compose group_add).
export DOCKER_GID="${DOCKER_GID:-$(stat -c %g /var/run/docker.sock 2>/dev/null || echo 999)}"
C=(docker compose -p freya-stack -f deploy/stack/compose.yaml)

"${C[@]}" up -d --build

echo
echo "Stack up (operator: $OPERATOR_EMAIL). Accept link:"
link=$("${C[@]}" logs auth-bootstrap 2>/dev/null | grep -oE 'https://[^" ]*invite/accept[^" ]*' | tail -1 || true)
if [ -n "$link" ]; then
  echo "  $link"
else
  echo "  (operator already provisioned, or see: ${C[*]} logs auth-bootstrap)"
fi
echo "Then set password + TOTP and sign in at https://localhost:8443"
