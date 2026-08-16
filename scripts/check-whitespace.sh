#!/usr/bin/env bash
set -uo pipefail

# Repository-wide whitespace hygiene. `git diff --check` only inspects a diff,
# so a file committed with trailing whitespace stays wrong forever and every
# later diff touching a neighbouring line reports it again. This gate scans
# content instead of a diff, which is what makes an existing violation visible.
#
# Vendored dependencies are excluded: their formatting belongs to upstream and
# `go mod vendor` would reintroduce any local fix. Binary files are skipped by
# `grep -I` and by perl's `-T` text heuristic.
#
# Files are batched through xargs rather than inspected one process at a time;
# per-file invocation costs seconds on this repository and this gate runs inside
# `verify`. `/dev/null` is appended to every grep batch so grep always sees more
# than one file and therefore always prefixes its output with the file name,
# including when xargs splits a batch down to a single path.

list="$(mktemp "${TMPDIR:-/tmp}/agentdeck-whitespace.XXXXXX")" || {
	printf '%s\n' 'whitespace scan failed: unable to create repository file list' >&2
	exit 2
}
report="$(mktemp "${TMPDIR:-/tmp}/agentdeck-whitespace-out.XXXXXX")" || {
	rm -f "$list"
	printf '%s\n' 'whitespace scan failed: unable to create report file' >&2
	exit 2
}
trap 'rm -f "$list" "$report"' EXIT

if ! git ls-files --cached -z -- ':!vendor' ':!vendor/**' >"$list" 2>/dev/null ||
	! git ls-files --others --exclude-standard -z -- ':!vendor' ':!vendor/**' >>"$list" 2>/dev/null; then
	printf '%s\n' 'whitespace scan failed: unable to enumerate repository files' >&2
	exit 2
fi

# Drop symlinks and anything that vanished between listing and inspection, so
# grep never reports a path the repository does not actually carry as content.
regular="$(mktemp "${TMPDIR:-/tmp}/agentdeck-whitespace-reg.XXXXXX")" || {
	printf '%s\n' 'whitespace scan failed: unable to filter repository files' >&2
	exit 2
}
trap 'rm -f "$list" "$report" "$regular"' EXIT
while IFS= read -r -d '' path; do
	if [ -f "$path" ] && [ ! -L "$path" ]; then
		printf '%s\0' "$path"
	fi
done <"$list" >"$regular"

failed=0

scan() {
	local pattern="$1" label="$2" status
	# The greedy leading `.*` anchors on the LAST `:<line>:` grep emitted, so a
	# path that itself contains a colon still reports the right line number.
	LC_ALL=C xargs -0 grep -I -n -e "$pattern" -- /dev/null <"$regular" |
		sed -e 's/^\(.*\):\([0-9][0-9]*\):.*$/\1:\2/' -e "s/^/${label}: /" >>"$report"
	status=${PIPESTATUS[0]}
	# xargs reports 123 when a grep batch exits non-zero, which includes the
	# ordinary "no match" case. Only a hard failure is an error here.
	case "$status" in
	0 | 123 | 1) ;;
	*) failed=1 ;;
	esac
}

scan '[ 	]$' 'trailing whitespace'
scan $'\r$' 'CRLF line ending'

LC_ALL=C xargs -0 perl -e '
	for my $f (@ARGV) {
		next unless -f $f && !-l $f && -s $f && -T $f;
		open my $h, "<", $f or next;
		binmode $h;
		seek $h, -1, 2 or next;
		read $h, my $last, 1;
		print "missing final newline: $f\n" if $last ne "\n";
	}
' <"$regular" >>"$report" || failed=1

if [ "$failed" -ne 0 ]; then
	printf '%s\n' 'whitespace scan failed: unable to inspect repository files' >&2
	exit 2
fi

if [ -s "$report" ]; then
	sort "$report"
	printf '%s\n' 'whitespace scan found violations; fix them in the source file' >&2
	exit 1
fi
