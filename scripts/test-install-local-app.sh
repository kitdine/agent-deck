#!/usr/bin/env bash
set -euo pipefail

# make install-macos-app in a temporary account, APPDIR, command link and
# record directory. The App's helper is an agentdeck_acceptance build, so the
# receipt and ownership lock are the temporary account's. The fixture App has
# no extensions and LaunchServices registration is disabled, so neither
# PlugInKit nor the LaunchServices database is touched.

root=$(cd "$(dirname "$0")/.." && pwd)
temporary=$(mktemp -d "${TMPDIR:-/tmp}/agentdeck-local-app.XXXXXX")
trap 'rm -rf "$temporary"' EXIT
temporary=$(cd "$temporary" && pwd -P)

export GOCACHE=${GOCACHE:-/private/tmp/agent-deck-go-build}
export GOMODCACHE=${GOMODCACHE:-/private/tmp/agent-deck-go-mod}
go_bin=${GO:-go}

account="$temporary/account"
export AGENTDECK_ACCEPTANCE_ACCOUNT_HOME="$account"
export HOME="$temporary/home"
export APPDIR="$temporary/Applications"
export BIN_LINK="$temporary/bin/agentdeck"
export RECORD_DIR="$temporary/records"
export LSREGISTER=none
mkdir -p "$account" "$HOME" "$HOME/.agentdeck"
printf 'preserve user state\n' >"$HOME/.agentdeck/preserved"
receipt="$account/Library/Application Support/AgentDeck/installation.json"
app="$APPDIR/AgentDeck.app"
helper="$app/Contents/Helpers/agentdeck"
installer="$root/scripts/install-local-app.sh"

helper_build="$temporary/helper/agentdeck"
mkdir -p "$(dirname "$helper_build")"
(cd "$root" && "$go_bin" build -mod=vendor -trimpath -tags agentdeck_acceptance -o "$helper_build" ./cmd/agentdeck)
"$go_bin" tool nm "$helper_build" | grep -F 'installation.acceptanceAccountHome' >/dev/null || {
  echo "refusing to install a helper that would use the real account" >&2
  exit 1
}

make_app() {
  local bundle=$1 version=$2
  mkdir -p "$bundle/Contents/Helpers" "$bundle/Contents/MacOS"
  cp "$helper_build" "$bundle/Contents/Helpers/agentdeck"
  printf '#!/bin/sh\necho AgentDeck %s\n' "$version" >"$bundle/Contents/MacOS/AgentDeck"
  chmod 0755 "$bundle/Contents/MacOS/AgentDeck"
  printf 'version %s\n' "$version" >"$bundle/Contents/version.txt"
}

receipt_field() {
  sed -n "s/.*\"$1\":\"\\([^\"]*\\)\".*/\\1/p" "$receipt"
}

expect_failure() {
  local message=$1
  shift
  if "$@" >"$temporary/failure.log" 2>&1; then
    echo "$message" >&2
    exit 1
  fi
}

make_app "$temporary/build-1/AgentDeck.app" 1
make_app "$temporary/build-2/AgentDeck.app" 2
make_app "$temporary/build-3/AgentDeck.app" 3

# --- install, upgrade ---
bash "$installer" install "$temporary/build-1/AgentDeck.app" >/dev/null
grep -F 'version 1' "$app/Contents/version.txt" >/dev/null
[ "$(receipt_field executable)" = "$helper" ]
[ "$(receipt_field channel)" = local_app ]
[ "$(readlink "$BIN_LINK")" = "$helper" ]
"$BIN_LINK" --format json doctor | grep -F '"reason":"local_app"' >/dev/null
first_id=$(receipt_field installation_id)
first_helper=$(sed -n 's/^cleanup_helper=//p' "$RECORD_DIR/install-record")
test -x "$first_helper"
test -z "$(find "$APPDIR" -maxdepth 1 -name '.AgentDeck-staged.*')"

bash "$installer" install "$temporary/build-2/AgentDeck.app" >/dev/null
grep -F 'version 2' "$app/Contents/version.txt" >/dev/null
second_id=$(receipt_field installation_id)
[ "$second_id" != "$first_id" ]
test ! -e "$first_helper"
test -z "$(find "$APPDIR" -maxdepth 1 -name '.AgentDeck-staged.*')"
[ "$(ls "$RECORD_DIR/cleanup" | wc -l | tr -d ' ')" = 1 ]

# --- a running owner blocks publication; nothing changes ---
lock="$account/Library/Application Support/AgentDeck/ownership.lock"
python3 -c 'import fcntl,sys,time; f=open(sys.argv[1],"a"); fcntl.flock(f,fcntl.LOCK_SH); open(sys.argv[2],"w").close(); time.sleep(12)' "$lock" "$temporary/locked" &
holder=$!
while [ ! -e "$temporary/locked" ]; do sleep 0.05; done
receipt_before=$(cat "$receipt")
expect_failure "install replaced a running owner's App" bash "$installer" install "$temporary/build-3/AgentDeck.app"
grep -F 'owner_running' "$temporary/failure.log" >/dev/null
grep -F 'version 2' "$app/Contents/version.txt" >/dev/null
[ "$(cat "$receipt")" = "$receipt_before" ]
[ "$(ls "$RECORD_DIR/cleanup" | wc -l | tr -d ' ')" = 1 ]
test -z "$(find "$APPDIR" -maxdepth 1 -name '.AgentDeck-staged.*')"
kill "$holder" 2>/dev/null || :
wait "$holder" 2>/dev/null || :

# --- another owner: refused without TAKEOVER; TAKEOVER replaces it ---
other="$temporary/other/agentdeck"
mkdir -p "$(dirname "$other")"
cp "$helper_build" "$temporary/other/.agentdeck.next"
"$temporary/other/.agentdeck.next" install register --channel make --executable "$other" \
  --source "$temporary/other/.agentdeck.next" --cleanup-manifest "$temporary/other/cleanup.json" --takeover >/dev/null
other_receipt=$(cat "$receipt")
expect_failure "install replaced another owner without TAKEOVER" bash "$installer" install "$temporary/build-3/AgentDeck.app"
grep -F 'owned_by_other' "$temporary/failure.log" >/dev/null
grep -F 'version 2' "$app/Contents/version.txt" >/dev/null
[ "$(cat "$receipt")" = "$other_receipt" ]

# The superseded App's uninstall leaves the new owner and the App alone.
bash "$installer" uninstall | grep -F 'skipped: a later installation now owns' >/dev/null
test -d "$app"
[ "$(cat "$receipt")" = "$other_receipt" ]
test ! -e "$RECORD_DIR/install-record"

FORCE=1 TAKEOVER=1 bash "$installer" install "$temporary/build-3/AgentDeck.app" >/dev/null
grep -F 'version 3' "$app/Contents/version.txt" >/dev/null
[ "$(receipt_field executable)" = "$helper" ]

# --- a broken helper inside the final App does not block removal ---
printf 'broken\n' >"$helper"
bash "$installer" uninstall >/dev/null
test ! -e "$app"
test ! -e "$receipt"
test ! -L "$BIN_LINK"
test -d "$APPDIR"
test -f "$other"
test -f "$HOME/.agentdeck/preserved"
test -z "$(ls -A "$RECORD_DIR/cleanup")"
test -z "$(ls -A "$RECORD_DIR/helpers")"
expect_failure "uninstall without a record succeeded" bash "$installer" uninstall

# --- missing private helper copy: removal is refused, never a raw delete ---
bash "$installer" install "$temporary/build-1/AgentDeck.app" >/dev/null
rm -f "$(sed -n 's/^cleanup_helper=//p' "$RECORD_DIR/install-record")"
expect_failure "uninstall removed the App without its cleanup helper" bash "$installer" uninstall
test -d "$app"
test -f "$receipt"

# --- an unrecorded App at the target needs FORCE ---
rm -rf "$RECORD_DIR"
expect_failure "install replaced an unrecorded App without FORCE" bash "$installer" install "$temporary/build-2/AgentDeck.app"
grep -F 'version 1' "$app/Contents/version.txt" >/dev/null

printf 'local App install: receipt registration, conditional cleanup and takeover verified in a temporary account.\n'
