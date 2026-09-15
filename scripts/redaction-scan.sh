#!/usr/bin/env bash
# SC-008: run the integration suite with output capture, then scan every captured
# log, trace, metric and error body for secret material.
set -euo pipefail
ART="${ARTIFACTS:-.artifacts}"
export FREYA_CAPTURE_DIR="$ART/capture"
mkdir -p "$FREYA_CAPTURE_DIR"
find "$FREYA_CAPTURE_DIR" -type f -name '*.log' -delete
go test -count=1 ./tests/integration/... -v > "$ART/integration.log" 2>&1 || { tail -50 "$ART/integration.log"; exit 1; }
cp "$ART/integration.log" "$FREYA_CAPTURE_DIR/suite.log"
matches=0
for pat in '-----BEGIN' 'PRIVATE KEY' 'Bearer '; do
  n=$(grep -rc -- "$pat" "$FREYA_CAPTURE_DIR" | awk -F: '{s+=$2} END {print s+0}')
  echo "redaction-scan: pattern '$pat': $n matches"
  matches=$((matches + n))
done
if [[ "$matches" -ne 0 ]]; then
  echo "redaction-scan: FAIL ($matches matches)" >&2; exit 1
fi
echo "redaction-scan: 0 matches"
