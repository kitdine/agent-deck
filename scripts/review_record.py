"""Pure review parsing and the supported Markdown code masks; no live gates."""
from __future__ import annotations

import re
from pathlib import Path

PREFIX = r"^[ \t]*(?:[-*]\s+|✅\s*)?"
STYLE = r"(?:\*\*|__)?"
VALUE = STYLE + r"`?"
VERDICT = re.compile(
    PREFIX + STYLE + r"(?:Verdict|结论|裁决|评审结论|复评结论)" + STYLE +
    r"\s*[:：]\s*" + VALUE + r"(PASS|FAIL|REOPEN)\b", re.I | re.M)
COMPLETION_GATE = re.compile(
    PREFIX + STYLE + r"(?:Completion gate|完成门禁|证据门禁|验收门禁)" + STYLE +
    r"\s*[:：]\s*" + VALUE + r"(VERIFIED|NOT_VERIFIED|FAILED|BLOCKED|NOT_REQUIRED)\b", re.I | re.M)
REVIEW_ROUND = re.compile(
    r"^##[ \t]+(?:[^\w#\n]+[ \t]*)?"
    r"(?:(?:(?:Re-review|Review)[ \t]*[—–-][ \t]*)?Round[ \t]+\d+\b"
    r"|(?:评审|复评)[ \t]*[—–-]?[ \t]*第?\d+轮)", re.I | re.M)


def visible_markdown(text: str, strip_formatting: bool = True) -> str:
    """Exclude fenced and indented code, retaining line numbers and inline code.

    Declaration regexes support field/value emphasis explicitly. Removing all
    backticks would turn an inline-code quotation into a live declaration.
    strip_formatting remains accepted for callers of the former helper.
    """
    visible, fence = [], ''
    for raw in text.splitlines(keepends=True):
        line = raw.rstrip('\r\n')
        ending = raw[len(line):]
        marker = re.match(r'^ {0,3}(`{3,}|~{3,})(.*)$', line)
        if marker:
            run, tail = marker.groups()
            if not fence:
                fence = run
            elif run[0] == fence[0] and len(run) >= len(fence) and not tail.strip():
                fence = ''
            visible.append(' ' * len(line) + ending)
        elif fence or line.startswith(('    ', '\t')):
            visible.append(' ' * len(line) + ending)
        else:
            visible.append(raw)
    return ''.join(visible)


def mask_inline_code(text: str) -> str:
    """Mask matching backtick spans, retaining newlines and string offsets."""
    spans = re.compile(r'(?<!`)(`+)(?!`)([\s\S]*?)(?<!`)\1(?!`)')
    return spans.sub(lambda m: ''.join('\n' if c == '\n' else ' ' for c in m[0]), text)


def review_sections(text: str) -> list[str]:
    masked = visible_markdown(mask_inline_code(text))
    rounds = list(REVIEW_ROUND.finditer(masked))
    if not rounds:
        rounds = list(re.finditer(r'^##\s+📋', masked, re.M))
    if not rounds:
        return [text]
    starts = [r.start() for r in rounds] + [len(text)]
    return [text[start:end] for start, end in zip(starts, starts[1:])]


def declarations(section: str) -> tuple[set[str], set[str]]:
    masked = visible_markdown(mask_inline_code(section))
    visible = visible_markdown(section)
    def live_values(pattern: re.Pattern) -> set[str]:
        values = set()
        for match in pattern.finditer(visible):
            colon = re.search(r'[:：]', match[0]).start()
            # Preserve backticks around values, while excluding fields inside
            # inline code spans, including spans that cross line boundaries.
            if masked[match.start():match.start() + colon].strip():
                values.add(match[1].upper())
        return values
    verdicts = {'FAIL' if v == 'REOPEN' else v for v in live_values(VERDICT)}
    gates = live_values(COMPLETION_GATE)
    return verdicts, gates


def section_state(section: str) -> tuple[str | None, str | None]:
    verdicts, gates = declarations(section)
    if len(verdicts) != 1 or len(gates) > 1:
        return None, None
    return next(iter(verdicts)), next(iter(gates)) if gates else None


def latest_review_state(path: Path) -> tuple[str | None, str | None]:
    try:
        return section_state(review_sections(path.read_text(encoding='utf-8', errors='replace'))[-1])
    except OSError:
        return None, None


def mechanical_key(section: str) -> str:
    # Link-only maintenance does not convert a historical report to a new round.
    return re.sub(r'!?\[[^\]\n]*\]\([^\n)]*\)', '[link]', section).strip()


def validation_errors(text: str, previous_text: str | None = None) -> list[str]:
    """Enforce schema only on changed rounds; never certify semantic PASS/CE."""
    previous = {mechanical_key(s) for s in review_sections(previous_text)} if previous_text is not None else set()
    errors = []
    for index, section in enumerate(review_sections(text), 1):
        if mechanical_key(section) in previous:
            continue
        verdicts, gates = declarations(section)
        if len(verdicts) > 1 or len(gates) > 1:
            errors.append(f'round section {index}: conflicting verdict/gate declarations')
        if len(verdicts) != 1 or len(gates) != 1:
            errors.append(f'round section {index}: review state is incomplete or unparseable')
    return errors


def validation_notes(text: str, previous_text: str | None) -> list[str]:
    final = review_sections(text)[-1]
    if previous_text is not None and None in section_state(final):
        previous = {mechanical_key(s) for s in review_sections(previous_text)}
        if mechanical_key(final) in previous:
            return ['unchanged historical final state is not canonical; structural check does not certify its CE gate']
    return []
