#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
temporary=$(mktemp -d /private/tmp/agentdeck-shell-acceptance.XXXXXX)
cleanup() {
  local status=$?
  if test "$status" -ne 0 && test "${KEEP_FAILED-}" = 1; then
    printf 'preserved failed shell acceptance fixture: %s\n' "$temporary" >&2
  else
    rm -rf "$temporary"
  fi
  return "$status"
}
trap cleanup EXIT

dist="$temporary/dist"
binary="$dist/agentdeck"
fake_bin="$temporary/bin"
client_bin="$temporary/clients"
base_path="/usr/bin:/bin:/usr/sbin:/sbin"

make -C "$root" DIST_DIR="$dist" build >/dev/null
mkdir -p "$fake_bin" "$client_bin"

cat >"$client_bin/codex" <<'EOF'
#!/bin/sh
printf 'codex|%s|%s\n' "${HEADROOM_PROJECT-}" "$*"
EOF
cat >"$client_bin/claude" <<'EOF'
#!/bin/sh
printf 'claude|%s|%s\n' "${ANTHROPIC_CUSTOM_HEADERS-}" "$*"
EOF
cat >"$fake_bin/agentdeck" <<'EOF'
#!/bin/sh
printf '%s\n' "$*" >>"$AGENTDECK_TRACE"
exec "$REAL_AGENTDECK" "$@"
EOF
cat >"$fake_bin/ps" <<'EOF'
#!/bin/sh
case "$*" in
  *comm=*) printf '%s\n' "$SHELL" ;;
  *ppid=*) printf '1\n' ;;
  *) exit 2 ;;
esac
EOF
chmod 0755 \
  "$client_bin/codex" \
  "$client_bin/claude" \
  "$fake_bin/agentdeck" \
  "$fake_bin/ps"

script_style=bsd
if ! script -q /dev/null /usr/bin/true </dev/null >/dev/null 2>&1; then
  script_style=util-linux
fi

shell_path() {
  command -v "$1"
}

startup_path() {
  local shell=$1
  local home=$2
  case "$shell" in
    bash) printf '%s\n' "$home/.bashrc" ;;
    zsh) printf '%s\n' "$home/.zshrc" ;;
    fish) printf '%s\n' "$home/.config/fish/config.fish" ;;
  esac
}

write_startup_fixture() {
  local shell=$1
  local home=$2
  local rc
  rc=$(startup_path "$shell" "$home")
  mkdir -p "$(dirname "$rc")"
  cat >"$rc" <<'EOF'
# unrelated startup content
# >>> agentdeck completion >>>
# completion sentinel
# <<< agentdeck completion <<<
EOF
}

run_agentdeck() {
  local home=$1
  shift
  env \
    HOME="$home" \
    ZDOTDIR="$home" \
    XDG_CONFIG_HOME="$home/.config" \
    SHELL="${AGENTDECK_TEST_SHELL-$(shell_path bash)}" \
    PATH="$fake_bin:$client_bin:$base_path" \
    REAL_AGENTDECK="$binary" \
    AGENTDECK_TRACE="${AGENTDECK_TRACE_FILE-/dev/null}" \
    "$binary" "$@"
}

prepare_provider() {
  local home=$1
  local select_direct=$2
  mkdir -p "$home"
  printf 'model = "synthetic"\n' >"$home/codex.toml"
  printf '{}\n' >"$home/claude.json"
  chmod 0600 "$home/codex.toml" "$home/claude.json"
  printf 'synthetic-secret\n' |
    run_agentdeck "$home" provider add helper \
      --endpoint https://provider.example --clients codex,claude >/dev/null
  run_agentdeck "$home" provider set-wrapper helper \
    --url https://wrapper.example --kind headroom >/dev/null 2>&1
  if test "$select_direct" = true; then
    run_agentdeck "$home" provider use helper \
      --client codex --config-path "$home/codex.toml" >/dev/null 2>&1
    run_agentdeck "$home" provider use helper \
      --client claude --config-path "$home/claude.json" >/dev/null 2>&1
  fi
}

run_pty() {
  local transcript=$1
  shift
  local driver_stderr="$transcript.driver.stderr"
  : >"$driver_stderr"
  if test "$script_style" = bsd; then
    if ! script -q "$transcript" "$@" </dev/null >/dev/null 2>"$driver_stderr"; then
      sed 's/\^D//g' "$transcript" >&2 || true
      cat "$driver_stderr" >&2
      return 1
    fi
  else
    local command_text
    printf -v command_text '%q ' "$@"
    if ! script -q -e -c "$command_text" "$transcript" \
      </dev/null >/dev/null 2>"$driver_stderr"; then
      sed 's/\^D//g' "$transcript" >&2 || true
      cat "$driver_stderr" >&2
      return 1
    fi
  fi
  test ! -s "$driver_stderr"
}

run_shell_probe() {
  local shell=$1
  local home=$2
  local probe=$3
  local transcript=$4
  local path_value=$5
  local shell_binary rc
  shell_binary=$(shell_path "$shell")
  rc=$(startup_path "$shell" "$home")

  local -a environment=(
    env
    -u HEADROOM_PROJECT
    -u ANTHROPIC_CUSTOM_HEADERS
    "HOME=$home"
    "ZDOTDIR=$home"
    "XDG_CONFIG_HOME=$home/.config"
    "SHELL=$shell_binary"
    "PATH=$path_value"
    "AGENTDECK_BIN=$binary"
    "REAL_AGENTDECK=$binary"
    "AGENTDECK_TRACE=${AGENTDECK_TRACE_FILE-}"
    "CODEX_CONFIG=$home/codex.toml"
    "CLAUDE_CONFIG=$home/claude.json"
    "FIXTURE=${FIXTURE_DIR-}"
    "PROBE=$probe"
    "PROBE_OK=${PROBE_OK_FILE-}"
    "REMOVE_OUTPUT=${REMOVE_OUTPUT_FILE-}"
    "SHELL_NAME=$shell"
    "DEACTIVATION=${DEACTIVATION_VALUE-}"
    "RESULT_FILE=${RESULT_FILE_PATH-}"
  )

  case "$shell" in
    bash)
      run_pty "$transcript" "${environment[@]}" \
        "$shell_binary" --noprofile --rcfile "$rc" -i -c 'source "$PROBE"'
      ;;
    zsh)
      run_pty "$transcript" "${environment[@]}" \
        "$shell_binary" -d -i -c 'source "$PROBE"'
      ;;
    fish)
      run_pty "$transcript" "${environment[@]}" \
        "$shell_binary" -i -c 'source "$PROBE"'
      ;;
  esac
}

run_interactive_provider_use() {
  local shell=$1
  local home=$2
  local transcript=$3
  local quiet=$4
  local shell_binary
  shell_binary=$(shell_path "$shell")
  local -a command=("$binary")
  if test "$quiet" = true; then
    command+=(--quiet)
  fi
  command+=(
    provider use helper
    --client codex
    --config-path "$home/codex.toml"
    --via
  )
  run_pty "$transcript" \
    env \
    -u HEADROOM_PROJECT \
    -u ANTHROPIC_CUSTOM_HEADERS \
    "HOME=$home" \
    "ZDOTDIR=$home" \
    "XDG_CONFIG_HOME=$home/.config" \
    "SHELL=$shell_binary" \
    "PATH=$fake_bin:$client_bin:$base_path" \
    "REAL_AGENTDECK=$binary" \
    "AGENTDECK_TRACE=${AGENTDECK_TRACE_FILE-}" \
    "${command[@]}"
}

assert_managed() {
  local shell=$1
  local home=$2
  grep -F '# >>> agentdeck shell integration >>>' \
    "$(startup_path "$shell" "$home")" >/dev/null
}

assert_shell_absent() {
  local shell=$1
  local home=$2
  case "$shell" in
    bash)
      test ! -e "$home/.bashrc"
      test ! -e "$home/.bash_profile"
      ;;
    zsh)
      test ! -e "$home/.zshrc"
      ;;
    fish)
      test ! -e "$home/.config/fish/config.fish"
      ;;
  esac
}

normalize_transcript() {
  local input=$1
  local output=$2
  LC_ALL=C tr -d '\010\r' <"$input" |
    sed 's/\^D//g' >"$output"
}

posix_route_probe="$temporary/route.posix"
fish_route_probe="$temporary/route.fish"
posix_remove_probe="$temporary/remove.posix"
fish_remove_probe="$temporary/remove.fish"
posix_absent_probe="$temporary/absent.posix"
fish_absent_probe="$temporary/absent.fish"
posix_clients_probe="$temporary/clients.posix"
fish_clients_probe="$temporary/clients.fish"
posix_setup_probe="$temporary/setup.posix"
fish_setup_probe="$temporary/setup.fish"
posix_functions_probe="$temporary/functions.posix"
fish_functions_probe="$temporary/functions.fish"
posix_no_functions_probe="$temporary/no-functions.posix"
fish_no_functions_probe="$temporary/no-functions.fish"

cat >"$posix_route_probe" <<'EOF'
set -eu
typeset -f codex >/dev/null
typeset -f claude >/dev/null
: >"$AGENTDECK_TRACE"
cd "$FIXTURE"
test "$(codex before)" = 'codex||before'
test "$(claude before)" = 'claude||before'
test ! -s "$AGENTDECK_TRACE"
"$AGENTDECK_BIN" provider use helper \
  --client codex --config-path "$CODEX_CONFIG" --via >/dev/null 2>&1
"$AGENTDECK_BIN" provider use helper \
  --client claude --config-path "$CLAUDE_CONFIG" --via >/dev/null 2>&1
test "$(codex after)" = 'codex|my%2Bproject|after'
test "$(claude after)" = 'claude|X-Headroom-Project: my%2Bproject|after'
test "$(wc -l <"$AGENTDECK_TRACE" | tr -d ' ')" = 2
grep -Fx 'shell-init --project-environment codex' "$AGENTDECK_TRACE" >/dev/null
grep -Fx 'shell-init --project-environment claude' "$AGENTDECK_TRACE" >/dev/null
: >"$PROBE_OK"
EOF

cat >"$fish_route_probe" <<'EOF'
functions -q codex; or exit 20
functions -q claude; or exit 21
printf '' >"$AGENTDECK_TRACE"
cd "$FIXTURE"; or exit 22
test (codex before) = 'codex||before'; or exit 23
test (claude before) = 'claude||before'; or exit 24
test ! -s "$AGENTDECK_TRACE"; or exit 25
"$AGENTDECK_BIN" provider use helper \
  --client codex --config-path "$CODEX_CONFIG" --via >/dev/null 2>&1; or exit 26
"$AGENTDECK_BIN" provider use helper \
  --client claude --config-path "$CLAUDE_CONFIG" --via >/dev/null 2>&1; or exit 27
test (codex after) = 'codex|my%2Bproject|after'; or exit 28
test (claude after) = 'claude|X-Headroom-Project: my%2Bproject|after'; or exit 29
test (count (cat "$AGENTDECK_TRACE")) -eq 2; or exit 30
grep -Fx 'shell-init --project-environment codex' "$AGENTDECK_TRACE" >/dev/null; or exit 31
grep -Fx 'shell-init --project-environment claude' "$AGENTDECK_TRACE" >/dev/null; or exit 32
printf '' >"$PROBE_OK"
EOF

cat >"$posix_remove_probe" <<'EOF'
set -eu
typeset -f codex >/dev/null
typeset -f claude >/dev/null
remove_output=$("$AGENTDECK_BIN" shell remove "$SHELL_NAME")
printf '%s\n' "$remove_output" >"$REMOVE_OUTPUT"
deactivation=$(printf '%s\n' "$remove_output" |
  sed -n "s/^Current session deactivation for $SHELL_NAME: //p")
test -n "$deactivation"
eval "$deactivation"
! typeset -f codex >/dev/null
! typeset -f claude >/dev/null
: >"$PROBE_OK"
EOF

cat >"$fish_remove_probe" <<'EOF'
functions -q codex; or exit 40
functions -q claude; or exit 41
set remove_output ("$AGENTDECK_BIN" shell remove "$SHELL_NAME")
printf '%s\n' $remove_output >"$REMOVE_OUTPUT"
set deactivation (printf '%s\n' $remove_output |
  sed -n "s/^Current session deactivation for $SHELL_NAME: //p")
test -n "$deactivation"; or exit 42
eval "$deactivation"; or exit 43
not functions -q codex; or exit 44
not functions -q claude; or exit 45
printf '' >"$PROBE_OK"
EOF

cat >"$posix_absent_probe" <<'EOF'
set -eu
! typeset -f codex >/dev/null
! typeset -f claude >/dev/null
eval "$DEACTIVATION"
! typeset -f codex >/dev/null
! typeset -f claude >/dev/null
test "$(codex final)" = 'codex||final'
test "$(claude final)" = 'claude||final'
: >"$PROBE_OK"
EOF

cat >"$fish_absent_probe" <<'EOF'
not functions -q codex; or exit 50
not functions -q claude; or exit 51
eval "$DEACTIVATION"; or exit 52
not functions -q codex; or exit 53
not functions -q claude; or exit 54
test (codex final) = 'codex||final'; or exit 55
test (claude final) = 'claude||final'; or exit 56
printf '' >"$PROBE_OK"
EOF

cat >"$posix_clients_probe" <<'EOF'
set -eu
test "$(codex unavailable)" = 'codex||unavailable'
test "$(claude unavailable)" = 'claude||unavailable'
: >"$PROBE_OK"
EOF

cat >"$fish_clients_probe" <<'EOF'
test (codex unavailable) = 'codex||unavailable'; or exit 60
test (claude unavailable) = 'claude||unavailable'; or exit 61
printf '' >"$PROBE_OK"
EOF

cat >"$posix_setup_probe" <<'EOF'
set -eu
"$AGENTDECK_BIN" shell setup >"$RESULT_FILE"
: >"$PROBE_OK"
EOF

cat >"$fish_setup_probe" <<'EOF'
"$AGENTDECK_BIN" shell setup >"$RESULT_FILE"; or exit 70
printf '' >"$PROBE_OK"
EOF

cat >"$posix_functions_probe" <<'EOF'
set -eu
typeset -f codex >/dev/null
typeset -f claude >/dev/null
: >"$PROBE_OK"
EOF

cat >"$fish_functions_probe" <<'EOF'
functions -q codex; or exit 100
functions -q claude; or exit 101
printf '' >"$PROBE_OK"
EOF

cat >"$posix_no_functions_probe" <<'EOF'
set -eu
! typeset -f codex >/dev/null
! typeset -f claude >/dev/null
: >"$PROBE_OK"
EOF

cat >"$fish_no_functions_probe" <<'EOF'
not functions -q codex; or exit 110
not functions -q claude; or exit 111
printf '' >"$PROBE_OK"
EOF

test_shell_lifecycle() {
  local shell=$1
  local home="$temporary/lifecycle-$shell/home"
  local fixture="$temporary/lifecycle-$shell/my+project"
  local rc original trace transcript probe_ok remove_output deactivation
  rc=$(startup_path "$shell" "$home")
  original="$temporary/lifecycle-$shell/original"
  trace="$temporary/lifecycle-$shell/agentdeck.trace"
  transcript="$temporary/lifecycle-$shell/route.transcript"
  probe_ok="$temporary/lifecycle-$shell/route.ok"
  remove_output="$temporary/lifecycle-$shell/remove.output"

  mkdir -p "$fixture"
  write_startup_fixture "$shell" "$home"
  cp "$rc" "$original"
  prepare_provider "$home" true
  AGENTDECK_TEST_SHELL=$(shell_path "$shell")
  run_agentdeck "$home" shell setup "$shell" >/dev/null
  assert_managed "$shell" "$home"

  AGENTDECK_TRACE_FILE=$trace
  FIXTURE_DIR=$fixture
  PROBE_OK_FILE=$probe_ok
  REMOVE_OUTPUT_FILE=
  DEACTIVATION_VALUE=
  RESULT_FILE_PATH=
  run_shell_probe "$shell" "$home" \
    "$(test "$shell" = fish && printf '%s' "$fish_route_probe" || printf '%s' "$posix_route_probe")" \
    "$transcript" "$fake_bin:$client_bin:$base_path"
  test -e "$probe_ok"

  probe_ok="$temporary/lifecycle-$shell/remove.ok"
  transcript="$temporary/lifecycle-$shell/remove.transcript"
  PROBE_OK_FILE=$probe_ok
  REMOVE_OUTPUT_FILE=$remove_output
  run_shell_probe "$shell" "$home" \
    "$(test "$shell" = fish && printf '%s' "$fish_remove_probe" || printf '%s' "$posix_remove_probe")" \
    "$transcript" "$fake_bin:$client_bin:$base_path"
  test -e "$probe_ok"
  deactivation=$(sed -n \
    "s/^Current session deactivation for $shell: //p" "$remove_output")
  test -n "$deactivation"
  cmp "$rc" "$original"

  probe_ok="$temporary/lifecycle-$shell/absent.ok"
  transcript="$temporary/lifecycle-$shell/absent.transcript"
  PROBE_OK_FILE=$probe_ok
  DEACTIVATION_VALUE=$deactivation
  run_shell_probe "$shell" "$home" \
    "$(test "$shell" = fish && printf '%s' "$fish_absent_probe" || printf '%s' "$posix_absent_probe")" \
    "$transcript" "$fake_bin:$client_bin:$base_path"
  test -e "$probe_ok"

  local unavailable_home="$temporary/unavailable-$shell/home"
  local unavailable_transcript="$temporary/unavailable-$shell/transcript"
  local unavailable_normalized="$temporary/unavailable-$shell/normalized"
  write_startup_fixture "$shell" "$unavailable_home"
  run_agentdeck "$unavailable_home" shell setup "$shell" >/dev/null
  PROBE_OK_FILE="$temporary/unavailable-$shell/ok"
  AGENTDECK_TRACE_FILE="$temporary/unavailable-$shell/unused.trace"
  DEACTIVATION_VALUE=
  run_shell_probe "$shell" "$unavailable_home" \
    "$(test "$shell" = fish && printf '%s' "$fish_clients_probe" || printf '%s' "$posix_clients_probe")" \
    "$unavailable_transcript" "$client_bin:$base_path"
  test -e "$PROBE_OK_FILE"
  normalize_transcript "$unavailable_transcript" "$unavailable_normalized"
  test ! -s "$unavailable_normalized"
}

test_default_multi_shell_setup() {
  local invoking=$1
  local second third home probe transcript
  home="$temporary/default-$invoking/home"
  case "$invoking" in
    bash)
      second=zsh
      third=fish
      ;;
    zsh)
      second=fish
      third=bash
      ;;
    fish)
      second=zsh
      third=bash
      ;;
  esac
  write_startup_fixture "$invoking" "$home"
  write_startup_fixture "$second" "$home"
  probe=$(test "$invoking" = fish && printf '%s' "$fish_setup_probe" || printf '%s' "$posix_setup_probe")
  transcript="$temporary/default-$invoking/setup.transcript"
  PROBE_OK_FILE="$temporary/default-$invoking/setup.ok"
  RESULT_FILE_PATH="$temporary/default-$invoking/setup.output"
  AGENTDECK_TRACE_FILE="$temporary/default-$invoking/trace"
  run_shell_probe "$invoking" "$home" "$probe" "$transcript" \
    "$fake_bin:$client_bin:$base_path"
  test -e "$PROBE_OK_FILE"
  assert_managed "$invoking" "$home"
  assert_managed "$second" "$home"
  assert_shell_absent "$third" "$home"
}

test_interactive_via() {
  local shell=$1
  local home="$temporary/via-$shell/home"
  local quiet_home="$temporary/quiet-via-$shell/home"
  local probe transcript rc
  prepare_provider "$home" false
  transcript="$temporary/via-$shell/switch.transcript"
  AGENTDECK_TRACE_FILE="$temporary/via-$shell/trace"
  run_interactive_provider_use "$shell" "$home" "$transcript" false
  assert_managed "$shell" "$home"
  grep -F 'advisory:' "$transcript" >/dev/null
  grep -F "$shell: configured " "$transcript" >/dev/null

  PROBE_OK_FILE="$temporary/via-$shell/functions.ok"
  transcript="$temporary/via-$shell/functions.transcript"
  probe=$(test "$shell" = fish && printf '%s' "$fish_functions_probe" || printf '%s' "$posix_functions_probe")
  run_shell_probe "$shell" "$home" "$probe" "$transcript" \
    "$fake_bin:$client_bin:$base_path"
  test -e "$PROBE_OK_FILE"

  prepare_provider "$quiet_home" false
  transcript="$temporary/quiet-via-$shell/switch.transcript"
  AGENTDECK_TRACE_FILE="$temporary/quiet-via-$shell/trace"
  run_interactive_provider_use "$shell" "$quiet_home" "$transcript" true
  assert_shell_absent "$shell" "$quiet_home"

  probe=$(test "$shell" = fish && printf '%s' "$fish_no_functions_probe" || printf '%s' "$posix_no_functions_probe")
  transcript="$temporary/quiet-via-$shell/functions.transcript"
  PROBE_OK_FILE="$temporary/quiet-via-$shell/functions.ok"
  run_shell_probe "$shell" "$quiet_home" "$probe" "$transcript" \
    "$fake_bin:$client_bin:$base_path"
  test -e "$PROBE_OK_FILE"
}

for shell in bash zsh fish; do
  shell_path "$shell" >/dev/null
  test_shell_lifecycle "$shell"
  test_default_multi_shell_setup "$shell"
  test_interactive_via "$shell"
done
