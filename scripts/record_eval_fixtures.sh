#!/usr/bin/env bash
set -euo pipefail

FIXTURE_DIR="${ASSISTANT_EVAL_FIXTURE_RECORD_DIR:-tests/evals/fixtures}"

cat <<EOF
Recording eval fixtures
  fixture_dir: $FIXTURE_DIR

This run expects the assistant process to have fixture recording enabled via:
  - config.yaml: eval_fixture_record_dir: "$FIXTURE_DIR"
  - or env var: ASSISTANT_EVAL_FIXTURE_RECORD_DIR="$FIXTURE_DIR"
EOF

export ASSISTANT_EVAL_FIXTURE_RECORD_DIR="$FIXTURE_DIR"
./scripts/eval_baseline.sh "$@"

fixture_count=0
if [[ -d "$FIXTURE_DIR" ]]; then
  fixture_count="$(find "$FIXTURE_DIR" -type f -name '*.json' | wc -l | awk '{print $1}')"
fi

echo "Fixtures recorded to $FIXTURE_DIR/ (json files: $fixture_count)"
