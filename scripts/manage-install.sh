#!/usr/bin/env bash
set -euo pipefail

start_marker='# >>> agentdeck completion >>>'
end_marker='# <<< agentdeck completion <<<'

fail() {
  printf '%s\n' "$*" >&2
  exit 1
}

validate_path() {
	[ -n "$1" ] || fail "$2 path is empty"
	case "$1" in
		*$'\n'*|*$'\r'*) fail "$2 path contains a newline" ;;
	esac
}

hash_file() {
  shasum -a 256 "$1" | awk '{print $1}'
}

valid_hash() {
  [[ $1 =~ ^[0-9a-f]{64}$ ]]
}

read_binary_identity() {
  local binary=$1 label=$2 output=$3 raw="$3.raw" lines identity version commit branch build_time go_version
  "$binary" version >"$raw" 2>/dev/null || fail "$label does not provide a valid AgentDeck version command: $binary"
  [ "$(tail -c 1 "$raw" | od -An -tuC | tr -d '[:space:]')" = 10 ] || fail "$label returned an invalid AgentDeck build identity: $binary"
  lines=$(wc -l <"$raw" | tr -d ' ')
  if [ "$lines" = 5 ]; then
    version=$(sed -n '1s/^Release Version: //p' "$raw")
    commit=$(sed -n '2s/^Git Commit Hash: //p' "$raw")
    branch=$(sed -n '3s/^Git Branch: //p' "$raw")
    go_version=$(sed -n '4s/^Go Version: //p' "$raw")
    build_time=$(sed -n '5s/^UTC Build Time: //p' "$raw")
    [[ $version =~ ^[^[:space:]]+$ ]] && [[ $commit =~ ^[^[:space:]]+$ ]] && [[ $branch =~ ^[^[:space:]]+$ ]] &&
      [[ $go_version =~ ^go[^[:space:]]+$ ]] &&
      [[ $build_time =~ ^([^[:space:]]+|[0-9]{4}-[0-9]{2}-[0-9]{2}[[:space:]][0-9]{2}:[0-9]{2}:[0-9]{2})$ ]] || fail "$label returned an invalid AgentDeck build identity: $binary"
    cp "$raw" "$output"
    return
  fi
  [ "$lines" = 1 ] || fail "$label returned an invalid AgentDeck build identity: $binary"
  identity=$(sed -n '1p' "$raw")
  if [[ $identity =~ ^agentdeck[[:space:]][^[:space:]]+[[:space:]]commit=[^[:space:]]+[[:space:]]branch=[^[:space:]]+[[:space:]]build_time=([^[:space:]]+|[0-9]{4}-[0-9]{2}-[0-9]{2}[[:space:]][0-9]{2}:[0-9]{2}:[0-9]{2})[[:space:]]go_version=go[^[:space:]]+$ ]]; then
    branch=${identity#* branch=}
    branch=${branch%% *}
  elif [[ $identity =~ ^agentdeck[[:space:]][^[:space:]]+[[:space:]]commit=[^[:space:]]+[[:space:]]build_time=[^[:space:]]+[[:space:]]go_version=go[^[:space:]]+$ ]]; then
    branch=unknown
  else
    fail "$label returned an invalid AgentDeck build identity: $binary"
  fi
  version=${identity#agentdeck }
  version=${version%% *}
  commit=${identity#* commit=}
  commit=${commit%% *}
  build_time=${identity#* build_time=}
  build_time=${build_time% go_version=*}
  go_version=${identity##* go_version=}
  printf 'Release Version: %s\nGit Commit Hash: %s\nGit Branch: %s\nGo Version: %s\nUTC Build Time: %s\n' "$version" "$commit" "$branch" "$go_version" "$build_time" >"$output"
}

canonical_path() {
  local path=$1 directory base
  directory=$(dirname "$path")
  base=$(basename "$path")
  (cd "$directory" && printf '%s/%s\n' "$(pwd -P)" "$base")
}

file_owner() {
  stat -f '%u' "$1" 2>/dev/null || stat -c '%u' "$1"
}

file_mode() {
  stat -f '%Lp' "$1" 2>/dev/null || stat -c '%a' "$1"
}

atomic_copy() {
	local source=$1 target=$2 directory temporary
	directory=$(dirname "$target")
	temporary=$(mktemp "$directory/.agentdeck.replace.XXXXXX")
	cp -p "$source" "$temporary"
	mv -f "$temporary" "$target"
}

ensure_directory() {
  local path=$1 current=$1 parent index
  local -a missing=()
  while [ ! -e "$current" ]; do
    missing+=("$current")
    parent=$(dirname "$current")
    [ "$parent" != "$current" ] || break
    current=$parent
  done
  [ -d "$current" ] || fail "directory path is blocked by a non-directory: $current"
  for ((index=${#missing[@]} - 1; index >= 0; index--)); do
    if mkdir "${missing[$index]}" 2>/dev/null; then
      created_dirs+=("${missing[$index]}")
    elif [ ! -d "${missing[$index]}" ]; then
      fail "unable to create directory: ${missing[$index]}"
    fi
  done
}

cleanup_created_directories() {
  local index
  [ "${install_succeeded:-0}" != 1 ] || return 0
  for ((index=${#created_dirs[@]} - 1; index >= 0; index--)); do
    rmdir "${created_dirs[$index]}" 2>/dev/null || :
  done
}

resolve_rc_path() {
  local path=$1 target count=0 followed_symlink=0
  while [ -L "$path" ]; do
    followed_symlink=1
    count=$((count + 1))
    [ "$count" -le 40 ] || fail "shell rc symlink chain is too deep: $1"
    target=$(readlink "$path")
    case "$target" in
      /*) path=$target ;;
      *) path=$(dirname "$path")/$target ;;
    esac
  done
  if [ "$followed_symlink" = 1 ] && [ ! -f "$path" ]; then
    fail "shell rc symlink target is not an existing regular file: $path"
  fi
  if [ -e "$path" ] && [ ! -f "$path" ]; then
    fail "shell rc is not a regular file: $path"
  fi
  path=$(canonical_path "$path")
  if [ -e "$path" ] && [ "$(file_owner "$path")" != "$(id -u)" ]; then
    fail "shell rc is not owned by the current user: $path"
  fi
  printf '%s\n' "$path"
}

detect_shell() {
	local requested=${COMPLETION_SHELL:-auto} pid comm raw_name name next count=0
  detected_login=0
  case "$requested" in
    fish|zsh|bash|none)
      detected_shell=$requested
      return
      ;;
    auto|'') ;;
    *) fail "unsupported COMPLETION_SHELL: $requested" ;;
  esac
  pid=$PPID
  while [ "$pid" -gt 1 ] && [ "$count" -lt 16 ]; do
    count=$((count + 1))
		comm=$(ps -p "$pid" -o comm= 2>/dev/null | awk '{$1=$1; print}' || :)
		[ -n "$comm" ] || break
		raw_name=${comm##*/}
		name=${raw_name#-}
		case "$name" in
			fish|zsh|bash)
				detected_shell=$name
				if [ "$raw_name" = -bash ]; then
					detected_login=1
				fi
				return
        ;;
    esac
		next=$(ps -p "$pid" -o ppid= 2>/dev/null | awk '{$1=$1; print}' || :)
    [[ $next =~ ^[0-9]+$ ]] || break
    pid=$next
  done
  fail "unable to detect invoking shell; set COMPLETION_SHELL=fish, zsh, bash, or none"
}

default_rc_path() {
  case "$detected_shell" in
    fish) printf '%s/fish/config.fish\n' "${XDG_CONFIG_HOME:-$HOME/.config}" ;;
    zsh) printf '%s/.zshrc\n' "${ZDOTDIR:-$HOME}" ;;
    bash)
      if [ "$detected_login" = 1 ]; then
        printf '%s/.bash_profile\n' "$HOME"
      else
        printf '%s/.bashrc\n' "$HOME"
      fi
      ;;
    *) fail "no rc path for shell: $detected_shell" ;;
  esac
}

shell_quote() {
  local escaped
  case "$1" in
    *$'\n'*) fail "completion path contains a newline" ;;
  esac
  escaped=$(printf '%s' "$1" | sed "s/'/'\\\\''/g")
  printf "'%s'" "$escaped"
}

write_block() {
  local shell=$1 completion=$2 destination=$3 quoted
  quoted=$(shell_quote "$completion")
  {
    printf '%s\n' "$start_marker"
    if [ "$shell" = zsh ]; then
      printf '%s\n' 'autoload -Uz compinit'
      printf '%s\n' '(( $+functions[compdef] )) || compinit'
    fi
    printf 'source %s\n' "$quoted"
    printf '%s\n' "$end_marker"
  } >"$destination"
}

marker_count() {
  grep -cF "$2" "$1" 2>/dev/null || :
}

extract_block() {
  awk -v start="$start_marker" -v end="$end_marker" '
    $0 == start { inside=1 }
    inside { print }
    $0 == end && inside { exit }
  ' "$1" >"$2"
}

remove_block() {
  local rc=$1 destination=$2 separator_added=$3 final_byte size suffix_metadata
  suffix_metadata="$work/remove-block-suffix"
  final_byte=$(tail -c 1 "$rc" | od -An -tuC | tr -d '[:space:]')
  awk -v start="$start_marker" -v end="$end_marker" -v separator="$separator_added" -v metadata="$suffix_metadata" '
    $0 == start {
      found++
      if (separator == "1") {
        if (!have_pending) { invalid=1; next }
        if (pending != "") printf "%s", pending
      } else if (have_pending) {
        printf "%s\n", pending
      }
      have_pending=0
      inside=1
      next
    }
    $0 == end && inside { inside=0; after=1; next }
    inside { next }
    {
      if (after) suffix=1
      if (have_pending) printf "%s\n", pending
      pending=$0
      have_pending=1
    }
    END {
      if (inside || found != 1 || invalid) exit 1
      if (have_pending) printf "%s\n", pending
      print suffix + 0 >metadata
    }
  ' "$rc" >"$destination"
  if [ "$(cat "$suffix_metadata")" = 1 ] && [ "$final_byte" != 10 ]; then
    size=$(wc -c <"$destination" | tr -d ' ')
    if [ "$size" -gt 0 ]; then
      dd if="$destination" of="$work/remove-block-truncated" bs=1 count=$((size - 1)) 2>/dev/null
      mv "$work/remove-block-truncated" "$destination"
    fi
  fi
  chmod "$(file_mode "$rc")" "$destination"
}

validate_markers() {
  local rc=$1 expected_hash=$2 extracted=$3
  [ "$(marker_count "$rc" "$start_marker")" = 1 ] || fail "managed completion start marker is missing or duplicated: $rc"
  [ "$(marker_count "$rc" "$end_marker")" = 1 ] || fail "managed completion end marker is missing or duplicated: $rc"
  extract_block "$rc" "$extracted"
  [ "$(hash_file "$extracted")" = "$expected_hash" ] || fail "managed completion block was modified: $rc"
}

read_manifest() {
  local file=$1 lines
  lines=$(wc -l <"$file" | tr -d ' ')
  if [ "$lines" = 2 ]; then
    manifest_version=1
    binary_path=${manifest_line1#path=}
    binary_sha=${manifest_line2#sha256=}
    [ "$manifest_line1" != "$binary_path" ] || fail "invalid version 1 install path record"
    [ "$manifest_line2" != "$binary_sha" ] || fail "invalid version 1 install hash record"
    completion_shell=none
    completion_path=
    completion_sha=
    rc_path=
    block_sha=
    rc_separator_added=
    return
  fi
  [ "$lines" = 9 ] || fail "invalid AgentDeck install manifest: $file"
  manifest_version=${manifest_line1#manifest_version=}
  binary_path=${manifest_line2#binary_path=}
  binary_sha=${manifest_line3#binary_sha256=}
  completion_shell=${manifest_line4#completion_shell=}
  completion_path=${manifest_line5#completion_path=}
  completion_sha=${manifest_line6#completion_sha256=}
  rc_path=${manifest_line7#rc_path=}
  block_sha=${manifest_line8#block_sha256=}
  rc_separator_added=${manifest_line9#rc_separator_added=}
  [ "$manifest_line1" = "manifest_version=$manifest_version" ] || fail "invalid manifest version record"
  [ "$manifest_line2" = "binary_path=$binary_path" ] || fail "invalid binary path record"
  [ "$manifest_line3" = "binary_sha256=$binary_sha" ] || fail "invalid binary hash record"
  [ "$manifest_line4" = "completion_shell=$completion_shell" ] || fail "invalid completion shell record"
  [ "$manifest_line5" = "completion_path=$completion_path" ] || fail "invalid completion path record"
  [ "$manifest_line6" = "completion_sha256=$completion_sha" ] || fail "invalid completion hash record"
  [ "$manifest_line7" = "rc_path=$rc_path" ] || fail "invalid rc path record"
  [ "$manifest_line8" = "block_sha256=$block_sha" ] || fail "invalid block hash record"
  [ "$manifest_line9" = "rc_separator_added=$rc_separator_added" ] || fail "invalid rc separator record"
  [ "$manifest_version" = 2 ] || fail "unsupported install manifest version: $manifest_version"
}

load_manifest() {
	local file=$1
	[ -f "$file" ] && [ ! -L "$file" ] || fail "valid AgentDeck install manifest not found: $file"
	manifest_line1=$(sed -n '1p' "$file")
	manifest_line2=$(sed -n '2p' "$file")
	manifest_line3=$(sed -n '3p' "$file")
	manifest_line4=$(sed -n '4p' "$file")
	manifest_line5=$(sed -n '5p' "$file")
	manifest_line6=$(sed -n '6p' "$file")
	manifest_line7=$(sed -n '7p' "$file")
	manifest_line8=$(sed -n '8p' "$file")
	manifest_line9=$(sed -n '9p' "$file")
	read_manifest "$file"
}

validate_regular_hash() {
  local path=$1 expected=$2 label=$3
  valid_hash "$expected" || fail "invalid $label hash"
  [ -f "$path" ] && [ ! -L "$path" ] || fail "$label is not a regular file: $path"
  [ "$(hash_file "$path")" = "$expected" ] || fail "$label hash mismatch: $path"
}

validate_loaded_manifest() {
  local expected_binary=$1 scratch=$2 expected_completion marker_line
  [ "$binary_path" = "$expected_binary" ] || fail "install manifest binary path mismatch"
  validate_regular_hash "$binary_path" "$binary_sha" "installed binary"
  if [ "$manifest_version" = 1 ]; then
    return
  fi
  case "$completion_shell" in fish|zsh|bash|none) ;; *) fail "invalid completion shell in manifest" ;; esac
  if [ "$completion_shell" = none ]; then
    [ -z "$completion_path$completion_sha$rc_path$block_sha$rc_separator_added" ] || fail "invalid completion opt-out records"
    return
  fi
  expected_completion="$data_dir/completions/agentdeck.$completion_shell"
  [ "$completion_path" = "$expected_completion" ] || fail "install manifest completion path mismatch"
  validate_regular_hash "$completion_path" "$completion_sha" "installed completion"
  [ -f "$rc_path" ] && [ ! -L "$rc_path" ] || fail "managed shell rc is not a regular file: $rc_path"
  [ "$(file_owner "$rc_path")" = "$(id -u)" ] || fail "managed shell rc is not owned by the current user: $rc_path"
  valid_hash "$block_sha" || fail "invalid managed block hash"
  case "$rc_separator_added" in 0|1) ;; *) fail "invalid managed rc separator record" ;; esac
  validate_markers "$rc_path" "$block_sha" "$scratch"
  if [ "$rc_separator_added" = 1 ]; then
    marker_line=$(awk -v start="$start_marker" '$0 == start { print NR; exit }' "$rc_path")
    [[ $marker_line =~ ^[0-9]+$ ]] && [ "$marker_line" -gt 1 ] || fail "managed completion separator is missing: $rc_path"
  fi
}

prepare_rc_with_block() {
  local rc=$1 block=$2 destination=$3 old_hash=${4:-} old_separator=${5:-}
  if [ -e "$rc" ]; then
    cp -p "$rc" "$destination"
  else
    : >"$destination"
    chmod 0600 "$destination"
  fi
  if [ -n "$old_hash" ]; then
    validate_markers "$destination" "$old_hash" "$work/existing-block"
    remove_block "$destination" "$work/rc-without-block" "$old_separator"
    mv "$work/rc-without-block" "$destination"
  else
    [ "$(marker_count "$destination" "$start_marker")" = 0 ] || fail "managed completion start marker already exists: $rc"
    [ "$(marker_count "$destination" "$end_marker")" = 0 ] || fail "managed completion end marker already exists: $rc"
  fi
  new_rc_separator_added=0
  if [ -s "$destination" ] && [ "$(tail -c 1 "$destination" | od -An -tuC | tr -d '[:space:]')" != 10 ]; then
    printf '\n' >>"$destination"
    new_rc_separator_added=1
  fi
  cat "$block" >>"$destination"
}

rollback_install() {
	if [ "${commit_started:-0}" != 1 ]; then
		return 0
	fi
  set +e
	if [ "$old_binary_exists" = 1 ]; then atomic_copy "$work/old-binary" "$target"; else rm -f "$target"; fi
	if [ "$old_completion_exists" = 1 ]; then atomic_copy "$work/old-completion" "$new_completion"; elif [ -n "$new_completion" ]; then rm -f "$new_completion"; fi
	if [ "$old_rc_exists" = 1 ]; then atomic_copy "$work/old-rc" "$new_rc"; elif [ -n "$new_rc" ]; then rm -f "$new_rc"; fi
	if [ "$old_manifest_exists" = 1 ]; then atomic_copy "$work/old-manifest" "$manifest"; else rm -f "$manifest"; fi
}

install_agentdeck() {
	local source=$1 requested_rc old_block_hash=
	validate_path "$source" "built binary"
	validate_path "$bindir" "binary directory"
	validate_path "$datadir" "install data directory"
  [ -f "$source" ] || fail "built AgentDeck binary not found: $source"
  detect_shell
  ensure_directory "$bindir"
  ensure_directory "$datadir"
  bin_dir=$(cd "$bindir" && pwd -P)
  data_dir=$(cd "$datadir" && pwd -P)
  target="$bin_dir/agentdeck"
  manifest="$data_dir/install-manifest"
  old_binary_exists=0
  old_completion_exists=0
  old_rc_exists=0
  old_manifest_exists=0
  new_completion=
  new_rc=

  if [ -e "$target" ] || [ -L "$target" ] || [ -e "$manifest" ] || [ -L "$manifest" ]; then
    [ "${FORCE:-0}" = 1 ] || fail "refusing to overwrite existing AgentDeck installation; rerun with FORCE=1"
    load_manifest "$manifest"
    validate_loaded_manifest "$target" "$work/validated-block"
		read_binary_identity "$target" "installed binary" "$work/old-identity"
    cp -p "$target" "$work/old-binary"
    cp -p "$manifest" "$work/old-manifest"
    old_binary_exists=1
		old_manifest_exists=1
		if [ "$manifest_version" = 2 ]; then
			[ "$completion_shell" = "$detected_shell" ] || fail "changing completion shell requires uninstall before reinstall"
			if [ "$completion_shell" != none ]; then
				old_block_hash=$block_sha
			fi
    fi
  fi

  install -m 0755 "$source" "$work/new-binary"
  new_binary_sha=$(hash_file "$work/new-binary")
	read_binary_identity "$work/new-binary" "built binary" "$work/new-identity"
  if [ "$detected_shell" != none ]; then
    ensure_directory "$data_dir/completions"
    new_completion="$data_dir/completions/agentdeck.$detected_shell"
    if [ -e "$new_completion" ] || [ -L "$new_completion" ]; then
      if [ "$old_manifest_exists" != 1 ] || [ "$manifest_version" != 2 ] || [ "$completion_path" != "$new_completion" ]; then
        fail "refusing existing completion outside the validated manifest: $new_completion"
      fi
      cp -p "$new_completion" "$work/old-completion"
      old_completion_exists=1
    fi
    "$work/new-binary" completion "$detected_shell" >"$work/new-completion"
    chmod 0644 "$work/new-completion"
    new_completion_sha=$(hash_file "$work/new-completion")
		requested_rc=${COMPLETION_RC:-$(default_rc_path)}
		validate_path "$requested_rc" "shell rc"
    ensure_directory "$(dirname "$requested_rc")"
    new_rc=$(resolve_rc_path "$requested_rc")
    if [ "$old_manifest_exists" = 1 ] && [ "$manifest_version" = 2 ] && [ "$completion_shell" != none ]; then
      [ "$completion_shell" = "$detected_shell" ] || fail "changing completion shell requires uninstall before reinstall"
      [ "$rc_path" = "$new_rc" ] || fail "changing completion rc requires uninstall before reinstall"
    fi
    if [ -e "$new_rc" ]; then
      cp -p "$new_rc" "$work/old-rc"
      old_rc_exists=1
    fi
    write_block "$detected_shell" "$new_completion" "$work/new-block"
    new_block_sha=$(hash_file "$work/new-block")
    prepare_rc_with_block "$new_rc" "$work/new-block" "$work/new-rc" "$old_block_hash" "${rc_separator_added:-}"
  else
    new_completion_sha=
    new_block_sha=
    new_rc_separator_added=
  fi

  {
    printf 'manifest_version=2\n'
    printf 'binary_path=%s\n' "$target"
    printf 'binary_sha256=%s\n' "$new_binary_sha"
    printf 'completion_shell=%s\n' "$detected_shell"
    printf 'completion_path=%s\n' "$new_completion"
    printf 'completion_sha256=%s\n' "$new_completion_sha"
    printf 'rc_path=%s\n' "$new_rc"
    printf 'block_sha256=%s\n' "$new_block_sha"
    printf 'rc_separator_added=%s\n' "$new_rc_separator_added"
  } >"$work/new-manifest"
  chmod 0644 "$work/new-manifest"

	if [ "$old_binary_exists" = 1 ]; then
		printf 'upgrade from:\n'
		cat "$work/old-identity"
		printf 'SHA-256: %s\n' "$binary_sha"
		printf 'upgrade to:\n'
		cat "$work/new-identity"
		printf 'SHA-256: %s\n' "$new_binary_sha"
	fi

	commit_started=1
	atomic_copy "$work/new-binary" "$target"
	if [ "$detected_shell" != none ]; then
		atomic_copy "$work/new-completion" "$new_completion"
		atomic_copy "$work/new-rc" "$new_rc"
	fi
	atomic_copy "$work/new-manifest" "$manifest"
  commit_started=0
  install_succeeded=1
  printf 'installed %s' "$target"
  if [ "$detected_shell" != none ]; then
    printf ' with %s completion via %s' "$detected_shell" "$new_rc"
  fi
  printf '\n'
}

rollback_uninstall() {
	if [ "${uninstall_started:-0}" != 1 ]; then
		return 0
	fi
  set +e
	atomic_copy "$work/uninstall-binary" "$binary_path"
	if [ "$manifest_version" = 2 ] && [ "$completion_shell" != none ]; then
		atomic_copy "$work/uninstall-completion" "$completion_path"
		atomic_copy "$work/uninstall-rc" "$rc_path"
	fi
	atomic_copy "$work/uninstall-manifest" "$manifest"
}

uninstall_agentdeck() {
	validate_path "$bindir" "binary directory"
	validate_path "$datadir" "install data directory"
  [ -d "$bindir" ] || fail "install directory not found: $bindir"
  [ -d "$datadir" ] || fail "install data directory not found: $datadir"
  bin_dir=$(cd "$bindir" && pwd -P)
  data_dir=$(cd "$datadir" && pwd -P)
  target="$bin_dir/agentdeck"
  manifest="$data_dir/install-manifest"
  load_manifest "$manifest"
  validate_loaded_manifest "$target" "$work/uninstall-block"
  cp -p "$binary_path" "$work/uninstall-binary"
  cp -p "$manifest" "$work/uninstall-manifest"
  if [ "$manifest_version" = 2 ] && [ "$completion_shell" != none ]; then
    cp -p "$completion_path" "$work/uninstall-completion"
    cp -p "$rc_path" "$work/uninstall-rc"
    remove_block "$rc_path" "$work/uninstall-rc-new" "$rc_separator_added"
  fi
  uninstall_started=1
  rm "$binary_path"
  if [ "$manifest_version" = 2 ] && [ "$completion_shell" != none ]; then
    rm "$completion_path"
		atomic_copy "$work/uninstall-rc-new" "$rc_path"
  fi
  rm "$manifest"
  uninstall_started=0
  printf 'uninstalled %s; preserved AgentDeck user state and unrelated shell configuration\n' "$binary_path"
}

action=${1:-}
source_binary=${2:-}
prefix=${PREFIX:-$HOME/.local}
bindir=${BINDIR:-$prefix/bin}
datadir=${DATADIR:-$prefix/share/agentdeck}
work=$(mktemp -d "${TMPDIR:-/tmp}/agentdeck-install-manager.XXXXXX")
commit_started=0
uninstall_started=0
install_succeeded=0
created_dirs=()
trap 'set +e; rollback_install; rollback_uninstall; cleanup_created_directories; rm -rf "$work"' EXIT
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM

case "$action" in
  install) install_agentdeck "$source_binary" ;;
  uninstall) uninstall_agentdeck ;;
  detect-shell)
    detect_shell
    printf '%s\n' "$detected_shell"
    ;;
  *) fail "usage: manage-install.sh install <binary>|uninstall|detect-shell" ;;
esac
