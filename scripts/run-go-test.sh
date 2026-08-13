#!/usr/bin/env bash

set -uo pipefail
umask 077

go_test_bin=${GO_TEST_BIN:-go}
log_dir=${AGENTDECK_GO_TEST_LOG_DIR:-${TMPDIR:-/tmp}}
tail_lines=${AGENTDECK_GO_TEST_TAIL_LINES:-80}

case "$tail_lines" in
  ''|*[!0-9]*|0)
    printf 'AGENTDECK_GO_TEST_TAIL_LINES must be a positive integer\n' >&2
    exit 2
    ;;
esac

if [ "$#" -eq 0 ]; then
  set -- ./...
fi

if [ -n "${AGENTDECK_GO_TEST_LOG:-}" ]; then
  log_file=$AGENTDECK_GO_TEST_LOG
else
  log_file=$(mktemp "${log_dir%/}/agentdeck-go-test.XXXXXX") || {
    printf 'go test setup failed: unable to create log in %s\n' "$log_dir" >&2
    exit 2
  }
fi

if ! : >"$log_file" || ! chmod 600 "$log_file"; then
  printf 'go test setup failed: unable to write log %s\n' "$log_file" >&2
  exit 2
fi

export GOCACHE=${GOCACHE:-/private/tmp/agent-deck-go-build}

printf 'go test log: %s\n' "$log_file" >&2

"$go_test_bin" test -mod=vendor -count=1 -v "$@" >"$log_file" 2>&1
test_status=$?

if [ "$test_status" -eq 0 ]; then
  printf 'go test passed; full log: %s\n' "$log_file" >&2
  exit 0
fi

printf 'go test failed with status %d; full log: %s\n' \
  "$test_status" "$log_file" >&2
printf '\nFailure summary:\n' >&2

failure_pattern='--- FAIL:|^FAIL([[:space:]]|$)|panic:|fatal error:|test timed out after|\[build failed\]'
if command -v rg >/dev/null 2>&1; then
  rg -n -C 8 -- "$failure_pattern" "$log_file" >&2 || true
else
  grep -n -E -C 8 -- "$failure_pattern" "$log_file" >&2 || true
fi

printf '\nLast %d log lines:\n' "$tail_lines" >&2
tail -n "$tail_lines" "$log_file" >&2

exit "$test_status"
