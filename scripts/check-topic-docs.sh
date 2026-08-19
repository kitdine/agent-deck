#!/usr/bin/env bash
set -uo pipefail

# Audits every topic's document set. The Documents matrix in tasks.md is the
# declaration, but a declaration can be wrong — ux/widget.md was required and
# missing for a day because nothing compared the matrix against the topic's own
# content. This gate does that comparison, so a missing document fails a check
# instead of waiting to be noticed.
#
# Three independent things must agree for every topic:
#
#   1. what the matrix declares,
#   2. what files exist on disk,
#   3. what the topic's own text implies is required.
#
# (3) is what makes this an audit rather than a checklist. A surface named in
# requirements.md or in a task's scope requires a ux/<surface>.md whether or not
# anyone remembered to add the row.
#
# ux/prototype/ is excluded from (2). A prototype is a rendered specimen cited by
# a ux/<surface>.md, not a topic document, and it brings its own README plus a
# dependency tree full of third-party README files. Requiring a matrix row for
# each of those would make the audit fail permanently and teach everyone to
# ignore it, which is the one outcome a gate must never produce.
#
# Exit 0 clean, 1 findings, 2 harness failure.

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
topics_dir="$root/docs/topics"

if [ ! -d "$topics_dir" ]; then
	printf '%s\n' 'topic audit failed: docs/topics is missing' >&2
	exit 2
fi

found=0
note() {
	printf '%s: %s\n' "$1" "$2"
	found=1
}

for topic_path in "$topics_dir"/*/; do
	topic="$(basename "$topic_path")"
	tasks="$topic_path/tasks.md"

	if [ ! -f "$tasks" ]; then
		note "$topic" "tasks.md is missing; every topic has one and it is the only status authority"
		continue
	fi

	# The Documents matrix: rows between the '## Documents' heading and the next
	# heading. Row 1 is the header, row 2 the separator; skip both.
	matrix="$(awk '/^## Documents/{f=1;next} /^## /{f=0} f' "$tasks" |
		grep -E '^\|' | sed -E 's/^\| *//; s/ *\|.*$//' |
		grep -vE '^(Document|-+)$' || true)"

	declared_na="$(awk '/^## Documents/{f=1;next} /^## /{f=0} f' "$tasks" |
		grep -E '^\|' | grep -E '\| *n/a *\|' | sed -E 's/^\| *//; s/ *\|.*$//' || true)"

	# 1 vs 2 — every declared row that is not n/a must exist, and vice versa.
	while IFS= read -r row; do
		[ -n "$row" ] || continue
		case "$row" in
		'`ux/`' | 'ux/') continue ;;
		esac
		clean="${row//\`/}"
		if printf '%s\n' "$declared_na" | grep -qxF "$row"; then
			[ -e "$topic_path/$clean" ] &&
				note "$topic" "$clean is declared n/a but exists on disk"
			continue
		fi
		[ -e "$topic_path/$clean" ] ||
			note "$topic" "$clean is declared in the Documents matrix but not written"
	done <<<"$matrix"

	while IFS= read -r file; do
		[ -n "$file" ] || continue
		rel="${file#"$topic_path"}"
		printf '%s\n' "$matrix" | sed 's/`//g' | grep -qxF "$rel" ||
			note "$topic" "$rel exists but no Documents matrix row declares it"
	done < <(find "$topic_path" -name '*.md' -not -path '*/reviews/*' \
		-not -path '*/prototype/*' | sort)

	# 3 — surfaces the topic's own text implies. A ux/<surface>.md is required
	# per user-visible surface; the matrix must at least mention each one.
	surfaces="$(grep -ohE 'ux/[a-z0-9-]+\.md' "$topic_path"/*.md "$topic_path"/ux/*.md 2>/dev/null |
		sort -u || true)"
	while IFS= read -r surface; do
		[ -n "$surface" ] || continue
		printf '%s\n' "$matrix" | sed 's/`//g' | grep -qxF "$surface" ||
			note "$topic" "$surface is referenced by this topic's own text but has no matrix row"
	done <<<"$surfaces"

	# A drafted row must not be an empty stub.
	while IFS= read -r row; do
		[ -n "$row" ] || continue
		clean="${row//\`/}"
		[ -f "$topic_path/$clean" ] || continue
		if [ "$(wc -l <"$topic_path/$clean")" -lt 10 ]; then
			note "$topic" "$clean is shorter than ten lines; an empty file never satisfies a row"
		fi
	done <<<"$matrix"
done

if [ "$found" -ne 0 ]; then
	printf '%s\n' 'topic document audit found gaps; fix the document or the matrix row' >&2
	exit 1
fi
