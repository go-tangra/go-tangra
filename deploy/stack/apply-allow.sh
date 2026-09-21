#!/usr/bin/env bash
# Re-apply the gateway registration allow-list (module SPIFFE id -> route prefixes; name).
# Not persisted across DB resets, so run this after any `up` that recreated the gateway DB.
set -euo pipefail
P=${1:-freya-stack}
sg docker -c "docker exec ${P}-gateway-1 gatewaysvc bootstrap -config deploy/container.yaml \
  -allow 'spiffe://example.org/svc/auth=/api/v1,/authorize,/.well-known,/console;auth' \
  -allow 'spiffe://example.org/svc/lcm=/api/lcm;lcm' \
  -allow 'spiffe://example.org/svc/notification=/api/notification;notification' \
  -allow 'spiffe://example.org/svc/warden=/api/warden,/warden/share;warden'"
echo 'allow-list applied'
