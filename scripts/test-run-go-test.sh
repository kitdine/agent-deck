#!/usr/bin/env bash

if [ "${AGENTDECK_FAKE_GO:-0}" = 1 ]; then
  printf 'FAKE_INVOCATION\n'
  printf 'ARGS:%s\n' "$*"
  printf 'GOCACHE:%s\n' "${GOCACHE:-}"
  printf 'STDOUT_MARKER\n'
  printf 'STDERR_MARKER\n' >&2

  if [ "${AGENTDECK_FAKE_GO_STATUS:-0}" -ne 0 ]; then
    printf '%s\n' '--- FAIL: TestSyntheticFailure (0.00s)'
    printf '%s\n' 'panic: test timed out after 1s' >&2
    printf '%s\n' 'FAIL fake/package 1.000s'
  fi

  exit "${AGENTDECK_FAKE_GO_STATUS:-0}"
fi

set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
runner="$root/scripts/run-go-test.sh"
fake_go="$root/scripts/test-run-go-test.sh"
temporary=$(mktemp -d "${TMPDIR:-/tmp}/agentdeck-go-test-runner.XXXXXX")
trap 'rm -rf "$temporary"' EXIT

fail() {
  printf 'run-go-test self-test failed: %s\n' "$1" >&2
  exit 1
}

success_log="$temporary/success.log"
success_output="$temporary/success-output.log"
if ! (
  unset GOCACHE
  AGENTDECK_FAKE_GO=1 \
    AGENTDECK_FAKE_GO_STATUS=0 \
    AGENTDECK_GO_TEST_LOG="$success_log" \
    GO_TEST_BIN="$fake_go" \
    "$runner" ./fake/package
) >"$success_output" 2>&1; then
  fail 'success path returned non-zero'
fi

grep -Fq 'ARGS:test -mod=vendor -count=1 -v ./fake/package' "$success_log" ||
  fail 'success path did not forward the expected Go test arguments'
grep -Fq 'GOCACHE:/private/tmp/agent-deck-go-build' "$success_log" ||
  fail 'success path did not set the managed Go build cache'
grep -Fq "go test passed; full log: $success_log" "$success_output" ||
  fail 'success path did not report its retained log'
[ "$(stat -f '%Lp' "$success_log")" = 600 ] ||
  fail 'success log permissions are not 0600'

automatic_output="$temporary/automatic-output.log"
if ! AGENTDECK_FAKE_GO=1 \
  AGENTDECK_FAKE_GO_STATUS=0 \
  AGENTDECK_GO_TEST_LOG_DIR="$temporary" \
  GO_TEST_BIN="$fake_go" \
  "$runner" ./fake/automatic >"$automatic_output" 2>&1; then
  fail 'automatic log path returned non-zero'
fi

automatic_log=$(sed -n 's/^go test log: //p' "$automatic_output")
[ -n "$automatic_log" ] ||
  fail 'automatic log path was not reported'
[ -f "$automatic_log" ] ||
  fail 'automatic log file was not created'
[ "$(stat -f '%Lp' "$automatic_log")" = 600 ] ||
  fail 'automatic log permissions are not 0600'

failure_log="$temporary/failure.log"
failure_output="$temporary/failure-output.log"
set +e
(
  unset GOCACHE
  AGENTDECK_FAKE_GO=1 \
    AGENTDECK_FAKE_GO_STATUS=7 \
    AGENTDECK_GO_TEST_LOG="$failure_log" \
    GO_TEST_BIN="$fake_go" \
    "$runner" -race ./fake/package
) >"$failure_output" 2>&1
failure_status=$?
set -e

[ "$failure_status" -eq 7 ] ||
  fail "failure path returned $failure_status instead of the Go exit status"
[ "$(grep -c '^FAKE_INVOCATION$' "$failure_log")" -eq 1 ] ||
  fail 'failure path invoked Go more than once'
grep -Fq 'ARGS:test -mod=vendor -count=1 -v -race ./fake/package' "$failure_log" ||
  fail 'failure path did not forward the expected Go test arguments'
grep -Fq 'STDOUT_MARKER' "$failure_log" ||
  fail 'failure log omitted stdout'
grep -Fq 'STDERR_MARKER' "$failure_log" ||
  fail 'failure log omitted stderr'
grep -Fq "go test failed with status 7; full log: $failure_log" "$failure_output" ||
  fail 'failure path did not report its exit status and retained log'
grep -Fq -- '--- FAIL: TestSyntheticFailure' "$failure_output" ||
  fail 'failure summary omitted the failing test'
grep -Fq 'panic: test timed out after 1s' "$failure_output" ||
  fail 'failure summary omitted timeout context'
grep -Fq 'Last 80 log lines:' "$failure_output" ||
  fail 'failure summary omitted the log tail'

printf 'run-go-test self-test passed\n'
