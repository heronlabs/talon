#!/usr/bin/env bash
# Fails unless every statement in internal/ is covered, testkit and tool mains excluded.
set -euo pipefail

profile="$1"
filtered="$(mktemp)"
trap 'rm -f "$filtered"' EXIT

grep -v -E '/internal/testkit/|/internal/tools/testshape/cmd/' "$profile" >"$filtered"
total="$(go tool cover -func="$filtered" | awk '/^total:/ {print $3}')"
echo "coverage: $total"
[ "$total" = "100.0%" ] || { echo "coverage below 100%" >&2; exit 1; }
