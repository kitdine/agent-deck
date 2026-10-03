#!/usr/bin/env python3
"""Scoped L0 document checks, independent of product verification and live CE.

Supports simple inline/reference links, fences, inline/indented code, headings,
HTML ids, and Lx/Lx-Ly source-line fragments. Complex nesting/escaping is reported
as unsupported. External links are never fetched. --all is a manual audit only.
"""
from __future__ import annotations

import argparse
import importlib.util
import os
import re
import subprocess
import unicodedata
from pathlib import Path
from urllib.parse import unquote, urlsplit

from review_record import mask_inline_code, visible_markdown

SPEC = importlib.util.spec_from_file_location('record_check', Path(__file__).with_name('check-review-records.py'))
RECORDS = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(RECORDS)


def git(*args: str) -> bytes:
    return subprocess.check_output(['git', *args])


def parse_changes(raw: bytes) -> list[tuple[str, str | None, str | None]]:
    fields = raw.decode('utf-8').split('\0')
    if fields.pop() != '':
        raise ValueError('unterminated name-status diff')
    changes = []
    while fields:
        status, path = fields.pop(0), fields.pop(0)
        kind = status[0]
        if kind in {'R', 'C'} and re.fullmatch(r'[RC]\d+', status):
            changes.append((kind, path, fields.pop(0)))
        elif status == 'A':
            changes.append((kind, None, path))
        elif status == 'D':
            changes.append((kind, path, None))
        elif status in {'M', 'T'}:
            changes.append((kind, path, path))
        else:
            raise ValueError(f'unsupported diff status: {status}')
    return changes


def changes_for(base: str | None, head: str | None) -> list[tuple[str, str | None, str | None]]:
    if bool(base) != bool(head):
        raise ValueError('both base and head are required')
    refs = [base, head] if base else ['HEAD']
    changes = parse_changes(git('diff', '--name-status', '-z', '--find-renames',
                                '--find-copies', '--find-copies-harder', *refs, '--'))
    if not base:
        changes.extend(('A', None, name) for name in
                       git('ls-files', '--others', '--exclude-standard', '-z').decode().split('\0') if name)
    return changes


def changed_paths(base: str | None, head: str | None) -> list[str]:
    return sorted({name for _, old, new in changes_for(base, head)
                   for name in (old, new) if name and name.endswith('.md')})


def inventory_paths() -> list[str]:
    raw = git('ls-files', '-z', '--', '*.md', ':!vendor/**', ':!**/node_modules/**')
    raw += git('ls-files', '--others', '--exclude-standard', '-z', '--',
               '*.md', ':!vendor/**', ':!**/node_modules/**')
    return sorted({p for p in raw.decode('utf-8').split('\0') if p.endswith('.md') and Path(p).is_file()})


def anchors(text: str) -> set[str]:
    visible = visible_markdown(text)
    masked = visible_markdown(mask_inline_code(text))
    found = set(re.findall(r'(?:id|name)=[\'"]([^\'"]+)', masked))
    duplicates = {}
    for match in re.finditer(r'^ {0,3}#{1,6}\s+(.+?)\s*#*$', visible, re.M):
        if not masked[match.start():match.end()].lstrip().startswith('#'):
            continue
        heading = match[1]
        heading = re.sub(r'\[([^\]]+)\]\([^)]*\)', r'\1', heading)
        heading = heading.replace('`', '').replace('**', '').replace('__', '').lower()
        slug = ''.join(c for c in heading if c in {'-', '_', ' '} or
                       unicodedata.category(c)[0] in {'L', 'N', 'M'}).replace(' ', '-')
        count = duplicates.get(slug, 0)
        duplicates[slug] = count + 1
        found.add(slug + (f'-{count}' if count else ''))
    return found


def link_targets(text: str) -> tuple[list[str], list[str]]:
    clean = visible_markdown(mask_inline_code(text), strip_formatting=False)
    targets, unsupported = [], []
    # Four-space continuation inside a list is not reliably an indented code
    # block in this supported subset. Report it instead of silently dropping it.
    rows = text.splitlines()
    for index, row in enumerate(rows):
        if row.startswith(('    ', '\t')) and '](' in row and index:
            prior = rows[index - 1]
            if re.match(r'^ {0,3}(?:[-*+] |\d+[.)] )', prior):
                unsupported.append('indented list continuation: ' + row.strip())
    consumed = list(clean)
    patterns = [r'(?<!\\)!?\[[^\[\]\n\\]*\]\(([^()\n\\]*)\)',
                r'^ {0,3}\[[^\[\]\n\\]+\]:\s*(.+)$']
    for pattern in patterns:
        for match in re.finditer(pattern, clean, re.M):
            body = match[1].strip()
            target = re.fullmatch(r'(<[^<>]+>|[^\s<>]+)(?:\s+(?:"[^"\n]*"|\'[^\'\n]*\'))?', body)
            if target and '\\' not in body:
                targets.append(target[1].strip('<>'))
            else:
                unsupported.append(match[0])
            consumed[match.start():match.end()] = ' ' * (match.end() - match.start())
    residual = ''.join(consumed)
    for match in re.finditer(r'(?<!\\)!?\[[^\n]*\]\([^\n]*|^ {0,3}\[[^\n]+\]:[^\n]*', residual, re.M):
        unsupported.append(match[0])
    return targets, unsupported


def destination(path: Path, target: str, root: Path) -> Path:
    name = unquote(urlsplit(target).path)
    value = root / name.lstrip('/') if name.startswith('/') else path.parent / name if name else path
    return Path(os.path.abspath(os.path.normpath(value)))


def link_errors(path: Path, root: Path, targets: set[Path] | None = None) -> list[str]:
    root, path = root.resolve(), path.absolute()
    if not path.resolve().is_relative_to(root):
        return [f'{path}: document escapes repository']
    parsed, unsupported = link_targets(path.read_text(encoding='utf-8'))
    errors = [f'{path}: unsupported link syntax: {raw}' for raw in unsupported
              if targets is None or any(p.name in raw for p in targets)]
    for target in parsed:
        url = urlsplit(target)
        if url.scheme or url.netloc:
            continue
        dest = destination(path, target, root)
        if targets is not None and dest not in targets:
            continue
        if not dest.resolve().is_relative_to(root):
            errors.append(f'{path}: local link escapes repository: {target}')
        elif not dest.exists():
            errors.append(f'{path}: missing local link: {target}')
        elif url.fragment:
            fragment = unquote(url.fragment)
            line = re.fullmatch(r'L(\d+)(?:-L(\d+))?', fragment)
            if line:
                start, end = int(line[1]), int(line[2] or line[1])
                count = len(dest.read_text(encoding='utf-8').splitlines())
                if not 1 <= start <= end <= count:
                    errors.append(f'{path}: invalid line range: {target} (file has {count} lines)')
            elif dest.suffix == '.md' and fragment not in anchors(dest.read_text(encoding='utf-8')):
                errors.append(f'{path}: missing Markdown anchor: {target}')
    return errors


def inspect_paths(paths: list[str], root: Path, base: str | None,
                  inventory: bool = False, previous_paths: dict[str, str] | None = None) -> tuple[list[str], list[str]]:
    errors, historical = [], []
    for name in paths:
        findings = RECORDS.check([name], base, previous_paths)
        try:
            findings.extend(link_errors(Path(name), root))
        except (OSError, UnicodeError, ValueError) as error:
            findings.append(f'{name}: cannot inspect links: {error}')
        target = historical if inventory and name.startswith('docs/archive/') else errors
        target.extend(findings)
    return errors, historical


def affected_topics(changes: list[tuple[str, str | None, str | None]]) -> set[str]:
    return {Path(name).parts[2] for _, old, new in changes for name in (old, new)
            if name and name.startswith('docs/topics/') and len(Path(name).parts) > 3}


def topic_errors(changes: list[tuple[str, str | None, str | None]], root: Path) -> list[str]:
    errors = []
    for topic in sorted(affected_topics(changes)):
        live = root / 'docs/topics' / topic
        archived = root / 'docs/archive/topics' / topic
        if live.is_dir() and any(p.is_file() for p in live.rglob('*')):
            args = [topic]
        elif archived.is_dir():
            args = ['--archive', topic]
        else:
            # Whole deletion: no live matrix remains. Current incoming links
            # are checked against all old paths; never pass a missing selector.
            print(f'topic {topic} removed: no live task matrix; incoming links checked')
            continue
        result = subprocess.run(['bash', str(root / 'scripts/check-topic-docs.sh'), *args])
        if result.returncode:
            errors.append(f'{topic}: document-set check failed ({result.returncode})')
    return errors


def scoped_errors(changes: list[tuple[str, str | None, str | None]], root: Path, base: str) -> list[str]:
    root = root.resolve()
    candidates = sorted({new for _, _, new in changes if new and new.endswith('.md')})
    paths = [p for p in candidates if Path(p).is_file() and Path(p).resolve().is_relative_to(root)]
    unavailable = [f'{p}: changed document is missing, non-regular, or outside the repository'
                   for p in candidates if p not in paths]
    previous = {new: old for status, old, new in changes if status in {'R', 'C'} and old and new}
    errors, _ = inspect_paths(paths, root, base, previous_paths=previous)
    errors.extend(unavailable)
    targets = {root / name for _, old, new in changes for name in (old, new) if name}
    for name in inventory_paths():
        if name in paths:
            continue
        try:
            errors.extend(link_errors(Path(name), root, targets))
        except (OSError, UnicodeError, ValueError) as error:
            errors.append(f'{name}: cannot inspect incoming links: {error}')
    errors.extend(topic_errors(changes, root))
    return sorted(set(errors))


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--base')
    parser.add_argument('--head')
    parser.add_argument('--all', action='store_true', help='explicit manual inventory audit, not CI fallback')
    args = parser.parse_args()
    root = Path(__file__).resolve().parents[1]
    if Path.cwd().resolve() != root:
        parser.error('run from the repository root')
    if args.all and (args.base or args.head):
        parser.error('--all cannot be combined with a commit range')
    subprocess.run(['make', 'check-whitespace'], check=True)
    subprocess.run(['git', 'diff', '--check', *([args.base, args.head] if args.base else ['HEAD']), '--'], check=True)
    if args.all:
        errors, historical = inspect_paths(inventory_paths(), root, None, True)
        for finding in historical:
            print(f'Historical inventory finding: {finding}')
        audit = subprocess.run(['bash', 'scripts/check-topic-docs.sh'])
        if audit.returncode:
            errors.append('active document-set audit failed')
    else:
        errors = scoped_errors(changes_for(args.base, args.head), root, args.base or 'HEAD')
    for error in errors:
        print(error)
    if not errors:
        print('Document structural checks passed; no semantic PASS or CE gate is certified.')
    return bool(errors)


if __name__ == '__main__':
    raise SystemExit(main())
