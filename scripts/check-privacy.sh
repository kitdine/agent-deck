#!/usr/bin/env bash
set -uo pipefail

pattern='AKIA[0-9A-Z]{16}|-----BEGIN [A-Z ]*PRIVATE KEY-----|sk-[A-Za-z0-9_-]{20,}|ghp_[A-Za-z0-9]{36}'
files="$(mktemp "${TMPDIR:-/tmp}/agentdeck-privacy.XXXXXX")" || {
	printf '%s\n' 'privacy scan failed: unable to create repository file list' >&2
	exit 2
}
trap 'rm -f "$files"' EXIT

if ! git ls-files --cached --others --exclude-standard -z >"$files" 2>/dev/null; then
	printf '%s\n' 'privacy scan failed: unable to enumerate repository files' >&2
	exit 2
fi

found=0
failed=0
while IFS= read -r -d '' path; do
	grep -q -E -- "$pattern" "$path" 2>/dev/null
	status=$?
	case "$status" in
	0)
		printf 'privacy scan found prohibited content: %s\n' "$path"
		found=1
		;;
	1) ;;
	*) failed=1 ;;
	esac
done <"$files"

if [ "$failed" -ne 0 ]; then
	printf '%s\n' 'privacy scan failed: unable to inspect repository files' >&2
	exit 2
fi
if [ "$found" -ne 0 ]; then
	exit 1
fi
