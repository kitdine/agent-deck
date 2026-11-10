#!/usr/bin/env bash
set -euo pipefail

# Installs a locally built AgentDeck.app as the account's AgentDeck owner.
#
#   install-local-app.sh install <AgentDeck.app>
#   install-local-app.sh uninstall
#
# The App is copied beside its final location and published by its own helper
# with `install register`, inside the ownership lock; this script never moves
# the App into place, never writes the receipt, and never deletes the final App.
# Removal is only `install unregister --remove` with this installation's own
# cleanup manifest, run by a preserved private copy of the helper.
#
# Environment:
#   APPDIR      directory that receives AgentDeck.app (default /Applications)
#   BIN_LINK    command link to the App's helper, or "none" (default /usr/local/bin/agentdeck)
#   RECORD_DIR  private records: manifests, helper copies (default ~/.local/share/agentdeck/local-app)
#   LSREGISTER  LaunchServices registration tool, or "none"
#   TAKEOVER=1  replace another installation that is not running
#   FORCE=1     replace an existing App that this installer did not record

fail() {
  printf '%s\n' "$*" >&2
  exit 1
}

appdir=${APPDIR:-/Applications}
bin_link=${BIN_LINK:-/usr/local/bin/agentdeck}
record_dir=${RECORD_DIR:-$HOME/.local/share/agentdeck/local-app}
lsregister=${LSREGISTER:-/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister}
record="$record_dir/install-record"

staged=
helper_copy=
cleanup_manifest=
registered=0
installation_id=
target_helper=

on_exit() {
  local status=$?
  set +e
  if [ "$status" != 0 ] && [ "$registered" = 1 ]; then
    # Undo only this installation, conditionally; never restore an older App.
    if ! "$helper_copy" install unregister --channel local_app --executable "$target_helper" \
      --expected-installation "$installation_id" --cleanup-manifest "$cleanup_manifest" --remove >&2; then
      printf 'the new App could not be rolled back; its cleanup record is %s and helper %s\n' "$cleanup_manifest" "$helper_copy" >&2
      registered=2
    fi
  fi
  if [ "$status" != 0 ] && [ "$registered" != 2 ]; then
    [ -z "$helper_copy" ] || rm -f "$helper_copy"
    [ -z "$cleanup_manifest" ] || rm -f "$cleanup_manifest"
  fi
  # Private staging only; after a failed publication the staged App is back here.
  [ -z "$staged" ] || rm -rf "$staged"
}
trap on_exit EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

read_record() {
  [ -f "$record" ] && [ ! -L "$record" ] || return 1
  record_app=$(sed -n 's/^app=//p' "$record")
  record_executable=$(sed -n 's/^executable=//p' "$record")
  record_manifest=$(sed -n 's/^cleanup_manifest=//p' "$record")
  record_id=$(sed -n 's/^installation_id=//p' "$record")
  record_helper=$(sed -n 's/^cleanup_helper=//p' "$record")
  [ "$record_executable" = "$record_app/Contents/Helpers/agentdeck" ] || return 1
  [[ $record_id =~ ^[0-9a-f]{32}$ ]] || return 1
  case "$record_manifest:$record_helper" in
    "$record_dir/cleanup/"*":$record_dir/helpers/"*) ;;
    *) return 1 ;;
  esac
}

link_points_to() {
  [ -L "$1" ] && [ "$(readlink "$1")" = "$2" ]
}

register_launch_services() {
  [ "$lsregister" = none ] || [ ! -x "$lsregister" ] || "$lsregister" "$@" || :
}

install_app() {
  local source_app=$1 target new_record
  [ -d "$source_app" ] || fail "built AgentDeck.app not found: $source_app"
  [ -x "$source_app/Contents/Helpers/agentdeck" ] || fail "AgentDeck.app has no embedded helper: $source_app"
  mkdir -p "$appdir" "$record_dir/cleanup" "$record_dir/helpers"
  chmod 0700 "$record_dir"
  appdir=$(cd "$appdir" && pwd -P)
  record_dir=$(cd "$record_dir" && pwd -P)
  record="$record_dir/install-record"
  target="$appdir/AgentDeck.app"
  target_helper="$target/Contents/Helpers/agentdeck"
  if [ -e "$target" ] || [ -L "$target" ]; then
    if ! read_record || [ "$record_app" != "$target" ]; then
      [ "${FORCE:-0}" = 1 ] || fail "refusing to replace $target, which this installer did not record; rerun with FORCE=1"
    fi
  fi
  if [ "$bin_link" != none ] && { [ -e "$bin_link" ] || [ -L "$bin_link" ]; } && [ ! -L "$bin_link" ]; then
    fail "refusing to replace $bin_link, which is not a link; remove the other installation first or set BIN_LINK=none"
  fi

  # Same directory and filesystem as the final App, as publication requires.
  staged=$(mktemp -d "$appdir/.AgentDeck-staged.XXXXXX")
  ditto "$source_app" "$staged"
  cleanup_manifest=$(mktemp "$record_dir/cleanup/install-XXXXXXXX")
  rm -f "$cleanup_manifest"
  cleanup_manifest="$cleanup_manifest.json"
  helper_copy=$(mktemp "$record_dir/helpers/agentdeck-XXXXXXXX")
  cp -p "$staged/Contents/Helpers/agentdeck" "$helper_copy"
  chmod 0700 "$helper_copy"

  set -- install register --channel local_app --executable "$target_helper" \
    --source "$staged" --cleanup-manifest "$cleanup_manifest"
  if [ "${TAKEOVER:-0}" = 1 ]; then
    set -- "$@" --takeover
  fi
  if ! "$staged/Contents/Helpers/agentdeck" "$@" >/dev/null; then
    fail "AgentDeck did not register the App; nothing was replaced"
  fi
  registered=1
  installation_id=$(sed -n 's/^{"installation_id":"\([0-9a-f]\{32\}\)".*$/\1/p' "$cleanup_manifest")
  [[ $installation_id =~ ^[0-9a-f]{32}$ ]] || fail "registered App wrote an unreadable cleanup record"

  if [ "$bin_link" != none ]; then
    mkdir -p "$(dirname "$bin_link")"
    ln -sfn "$target_helper" "$bin_link"
  fi
  new_record=$(mktemp "$record_dir/.install-record.XXXXXX")
  {
    printf 'app=%s\n' "$target"
    printf 'executable=%s\n' "$target_helper"
    printf 'cleanup_manifest=%s\n' "$cleanup_manifest"
    printf 'installation_id=%s\n' "$installation_id"
    printf 'cleanup_helper=%s\n' "$helper_copy"
  } >"$new_record"
  local previous_manifest= previous_helper=
  if read_record; then
    previous_manifest=$record_manifest
    previous_helper=$record_helper
  fi
  mv -f "$new_record" "$record"
  registered=2
  [ -z "$previous_manifest" ] || rm -f "$previous_manifest"
  [ -z "$previous_helper" ] || rm -f "$previous_helper"
  register_launch_services -f "$target"
  printf 'installed %s\n' "$target"
}

uninstall_app() {
  local outcome
  read_record || fail "no valid local App installation record in $record_dir"
  [ -x "$record_helper" ] && [ -f "$record_manifest" ] ||
    fail "the cleanup helper or manifest for $record_app is missing; reinstall with make install-macos-app, then uninstall"
  if ! outcome=$("$record_helper" install unregister --channel local_app --executable "$record_executable" \
    --expected-installation "$record_id" --cleanup-manifest "$record_manifest" --remove); then
    fail "AgentDeck could not verify $record_app for removal; nothing was removed"
  fi
  case "$outcome" in
    "skipped superseded "*)
      printf 'skipped: a later installation now owns %s; it was left in place\n' "$record_app"
      ;;
    *)
      if [ "$bin_link" != none ] && link_points_to "$bin_link" "$record_executable"; then
        rm -f "$bin_link"
      fi
      register_launch_services -u "$record_app"
      printf 'uninstalled %s; preserved AgentDeck user state\n' "$record_app"
      ;;
  esac
  rm -f "$record" "$record_manifest" "$record_helper"
}

case "${1:-}" in
  install)
    [ $# = 2 ] || fail "usage: install-local-app.sh install <AgentDeck.app>"
    install_app "$2"
    ;;
  uninstall) uninstall_app ;;
  *) fail "usage: install-local-app.sh install <AgentDeck.app>|uninstall" ;;
esac
