#!/usr/bin/env python3
"""Read-only structural review-record validation; no Hook, Beads or CE calls."""
import argparse
import subprocess
from pathlib import Path
from review_record import validation_errors, validation_notes


def is_review_record(path: str) -> bool:
    parts = Path(path).parts
    return path.endswith('.md') and path.startswith('docs/') and (
        'reviews' in parts or 'fixes' in parts)


def check(paths: list[str], base: str | None = None, previous_paths: dict[str, str] | None = None) -> list[str]:
    errors = []
    for name in paths:
        if not is_review_record(name):
            continue
        previous = None
        if base:
            # Missing at base means a new record; other Git failures must fail
            # closed rather than granting historical exemptions.
            prior = (previous_paths or {}).get(name, name)
            try:
                entry = subprocess.run(['git', 'ls-tree', '-z', base, '--', prior],
                                       check=True, capture_output=True)
                if entry.stdout:
                    previous = subprocess.check_output(['git', 'show', f'{base}:{prior}']).decode('utf-8')
            except (OSError, UnicodeError, subprocess.SubprocessError) as error:
                errors.append(f'{name}: cannot establish prior record: {error}')
                continue
        try:
            text = Path(name).read_text(encoding='utf-8')
            errors.extend(f'{name}: {error}' for error in validation_errors(text, previous))
            for note in validation_notes(text, previous):
                print(f'{name}: {note}')
        except (OSError, UnicodeError) as error:
            errors.append(f'{name}: cannot read record: {error}')
    return errors


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--base', help='preserve unchanged historical rounds at this commit')
    parser.add_argument('paths', nargs='+')
    args = parser.parse_args()
    errors = check(args.paths, args.base)
    for error in errors:
        print(error)
    raise SystemExit(bool(errors))
