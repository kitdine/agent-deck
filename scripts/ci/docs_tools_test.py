from __future__ import annotations

import functools
import importlib.util
import json
import math
import os
import re
import shlex
import shutil
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path
from unittest import mock

ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / 'scripts'))
sys.path.insert(0, str(Path(__file__).parent))
import classify_diff as DIFF
import review_record as REVIEW
import doc_scope as SCOPE


def load(name, filename):
    spec = importlib.util.spec_from_file_location(name, ROOT / 'scripts' / filename)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


DOCS = load('docs_check', 'check-docs.py')
RECORDS = load('records_check', 'check-review-records.py')


def workflow_job(name):
    text = (ROOT / '.github/workflows/ci.yml').read_text()
    return re.split(r'\n  [a-z][a-z0-9_-]*:\n', text.split(f'\n  {name}:\n', 1)[1], maxsplit=1)[0]


def workflow_step(job, name):
    return workflow_job(job).split(f'      - name: {name}\n', 1)[1].split('      - name:', 1)[0]


def workflow_run(job, name):
    return named_step(parsed_workflow()['jobs'][job], name)['run']


def workflow_on(filename='ci.yml'):
    text = (ROOT / '.github/workflows' / filename).read_text()
    return text.split('\non:\n', 1)[1].split('\npermissions:\n', 1)[0]


def workflow_event_block(filename, event):
    section = '\n' + workflow_on(filename) + '\n'
    parts = re.split(rf'\n  {re.escape(event)}:[^\n]*\n', section, maxsplit=1)
    if len(parts) < 2:
        return None
    return re.split(r'\n  [a-z_]+:[^\n]*\n', parts[1], maxsplit=1)[0]


def workflow_optional_list_items(block, key):
    if block is None:
        return None
    text = block.rstrip('\n') + '\n'
    inline = re.search(rf'^    {re.escape(key)}:\s*\[([^\]]*)\]\s*(?:#.*)?$', text, re.M)
    if inline:
        return [item.strip().strip("'\"") for item in inline.group(1).split(',') if item.strip()]
    match = re.search(rf'^    {re.escape(key)}:\s*(?:#.*)?\n((?:      - .+\n)+)', text, re.M)
    if match:
        return [line.strip().removeprefix('- ').strip().strip("'\"") for line in match.group(1).splitlines()]
    scalar = re.search(rf'^    {re.escape(key)}:\s*([^\[\n#]+?)\s*(?:#.*)?$', text, re.M)
    if scalar:
        return [scalar.group(1).strip().strip("'\"")]
    if re.search(rf'^    {re.escape(key)}:\s*(?:#.*)?$', text, re.M):
        return []
    return None


def workflow_list_items(block, key):
    return workflow_optional_list_items(block, key) or []


def workflow_mapping(block, key, indent):
    lines = block.splitlines()
    header = ' ' * indent + f'{key}:'
    start = None
    for idx, line in enumerate(lines):
        if line == header:
            start = idx + 1
            break
    if start is None:
        return {}
    entry_prefix = ' ' * (indent + 2)
    mapping = {}
    for line in lines[start:]:
        if not line.strip() or line.lstrip().startswith('#'):
            continue
        if not line.startswith(entry_prefix) or line.startswith(entry_prefix + ' '):
            break
        k, sep, value = line[len(entry_prefix):].partition(':')
        if not sep:
            break
        k = k.strip()
        if k in mapping:
            raise AssertionError(f'duplicate mapping key {k!r} under {key!r}')
        mapping[k] = value.strip().strip("'\"")
    return mapping


def _find_github_expression_end(text, start):
    in_string = False
    i = start
    length = len(text)
    while i < length:
        ch = text[i]
        if in_string:
            if ch == "'":
                if i + 1 < length and text[i + 1] == "'":
                    i += 2
                    continue
                in_string = False
            i += 1
            continue
        if ch == "'":
            in_string = True
            i += 1
            continue
        if text[i:i + 2] == '}}':
            return i
        i += 1
    raise ValueError(f'unclosed GitHub expression in {text!r}')


def _format_github_expression_value(result):
    if isinstance(result, bool):
        return str(result).lower()
    return '' if result is None else str(result)


def resolve_github_expression(value, context, *, runner_os=None):
    if not isinstance(value, str) or '${{' not in value:
        return value
    parts = []
    pos = 0
    while True:
        start = value.find('${{', pos)
        if start == -1:
            parts.append(value[pos:])
            break
        if start > pos:
            parts.append(value[pos:start])
        end = _find_github_expression_end(value, start + 3)
        inner = value[start + 3:end]
        result = evaluate_github_expression(
            inner,
            needs=context.get('needs'),
            contexts=context,
            runner_os=runner_os,
        )
        parts.append(_format_github_expression_value(result))
        pos = end + 2
    return ''.join(parts)


def github_path_pattern_regex(pattern):
    parts = []
    i = 0
    length = len(pattern)
    while i < length:
        if pattern[i:i + 3] == '**/':
            parts.append('(?:.*/)?')
            i += 3
        elif pattern[i:i + 2] == '**':
            parts.append('.*')
            i += 2
        elif pattern[i] == '*':
            parts.append('[^/]*')
            i += 1
        elif pattern[i] == '?':
            parts.append('[^/]')
            i += 1
        else:
            parts.append(re.escape(pattern[i]))
            i += 1
    return re.compile('^' + ''.join(parts) + '$')


def workflow_paths_match(patterns, path):
    matched = False
    for pattern in patterns:
        negated = pattern.startswith('!')
        raw = pattern[1:] if negated else pattern
        if github_path_pattern_regex(raw).match(path):
            matched = not negated
    return matched


def workflow_event_triggers(filename, event, branch, changed_paths):
    block = workflow_event_block(filename, event)
    if block is None:
        return False
    branches = workflow_optional_list_items(block, 'branches')
    branches_ignore = workflow_optional_list_items(block, 'branches-ignore')
    if branches is not None and not workflow_paths_match(branches, branch):
        return False
    if branches_ignore is not None and workflow_paths_match(branches_ignore, branch):
        return False
    paths = workflow_optional_list_items(block, 'paths')
    paths_ignore = workflow_optional_list_items(block, 'paths-ignore')
    if paths is not None and not any(workflow_paths_match(paths, p) for p in changed_paths):
        return False
    if paths_ignore is not None and all(workflow_paths_match(paths_ignore, p) for p in changed_paths):
        return False
    return True


_SUPPORTED_EXPR_FUNCTIONS = {
    'always': 0,
    'cancelled': 0,
    'success': 0,
    'failure': 0,
    'contains': 2,
    'startswith': 2,
    'endswith': 2,
}
_STATUS_CHECK_FUNCTIONS = frozenset({'always', 'cancelled', 'success', 'failure'})
_SUPPORTED_EXPR_CONTEXTS = frozenset({'needs', 'github', 'matrix', 'steps', 'env', 'inputs'})

_EXPR_TOKEN_RE = re.compile(
    r"\s*(?:"
    r"(?P<STRING>'(?:''|[^'])*')"
    r"|(?P<OP>&&|\|\||==|!=|!)"
    r"|(?P<LPAREN>\()"
    r"|(?P<RPAREN>\))"
    r"|(?P<COMMA>,)"
    r"|(?P<NUMBER>(?:0[xX][0-9a-fA-F]+|[+-]?(?:[0-9]+(?:\.[0-9]+)?|\.[0-9]+)(?:[eE][+-]?[0-9]+)?))(?![A-Za-z0-9_.])"
    r"|(?P<DOT>\.)"
    r"|(?P<IDENT>[A-Za-z_][A-Za-z0-9_-]*)"
    r")",
    re.ASCII,
)
_PROP_SEG_RE = re.compile(r'\s*(?P<SEG>[A-Za-z_][A-Za-z0-9_-]*)', re.ASCII)
_COERCE_HEX_RE = re.compile(r'^0[xX][0-9a-fA-F]+$', re.ASCII)
_COERCE_DEC_RE = re.compile(r'^[+-]?(?:(?:[0-9]+(?:\.[0-9]*)?|\.[0-9]+)(?:[eE][+-]?[0-9]+)?|Infinity)$', re.ASCII)


@functools.lru_cache(maxsize=4)
def _parse_workflow_yaml(text):
    # Psych is the same standard-library YAML parser used by the repository's
    # workflow checks. Read scalar *text* so "on" and expressions are not
    # implicitly converted under YAML 1.1, normalize omitted mapping values
    # (plain untagged empty scalars) to {}, and reject duplicate mapping keys.
    reader = """
      def value(node)
        case node
        when Psych::Nodes::Scalar
          node.value
        when Psych::Nodes::Sequence
          node.children.map { |item| value(item) }
        when Psych::Nodes::Mapping
          node.children.each_slice(2).each_with_object({}) do |(key, item), result|
            name = key.value
            raise "duplicate YAML key #{name.inspect} at line #{key.start_line + 1}" if result.key?(name)
            result[name] = (item.is_a?(Psych::Nodes::Scalar) && item.plain && !item.quoted && item.tag.nil? && item.anchor.nil? && item.value.empty?) ? {} : value(item)
          end
        else
          raise "unsupported workflow node #{node.class}"
        end
      end
      puts JSON.generate(value(Psych.parse(STDIN.read).root))
    """
    result = subprocess.run(['ruby', '-rpsych', '-rjson', '-e', reader], input=text,
                            capture_output=True, text=True, check=True)
    return json.loads(result.stdout)


def parsed_workflow(filename='ci.yml'):
    return _parse_workflow_yaml((ROOT / '.github/workflows' / filename).read_text())


def runner_labels(job):
    value = job['runs-on']
    if isinstance(value, str):
        return [value]
    if isinstance(value, dict):
        unknown_keys = set(value) - {'group', 'labels'}
        if unknown_keys:
            raise ValueError(f'unsupported runs-on keys: {sorted(unknown_keys)!r}')
        if 'group' in value:
            group = value['group']
            if not isinstance(group, str) or not group.strip():
                raise ValueError(f'expected non-empty string for runs-on group, got {group!r}')
        if 'labels' in value:
            labels = value['labels']
            if isinstance(labels, str):
                return [labels]
            if isinstance(labels, list) and all(isinstance(item, str) for item in labels):
                return labels
            raise ValueError(f'expected string or list of strings for runs-on labels, got {labels!r}')
        if 'group' in value:
            return []
        raise ValueError(f'expected runs-on mapping with labels or group, got {value!r}')
    if isinstance(value, list):
        if not all(isinstance(item, str) for item in value):
            raise ValueError(f'expected list of strings for runs-on, got {value!r}')
        return value
    raise ValueError(f'expected string, list, or mapping for runs-on, got {value!r}')


def named_step(job, name):
    return next(step for step in job['steps'] if step.get('name') == name)


def _as_mapping(value):
    if value is None:
        return {}
    if not isinstance(value, dict):
        raise ValueError(f'expected mapping, got {value!r}')
    return value


_WINDOWS_RUNNER_LABELS = frozenset({
    'windows',
    'windows-latest',
    'windows-2025',
    'windows-2022',
    'windows-2019',
    'windows-11-arm',
    'windows-2025-vs2026',
    'windows-11-vs2026-arm',
})


def _explicit_runner_os(runner_os=None, contexts=None):
    if runner_os is not None:
        return str(runner_os)
    if isinstance(contexts, dict):
        runner_ctx = contexts.get('runner')
        if isinstance(runner_ctx, dict) and 'os' in runner_ctx:
            return str(runner_ctx.get('os', ''))
        if 'runner_os' in contexts:
            return str(contexts.get('runner_os', ''))
    return None


def _is_windows_runner_os(runner_os=None, contexts=None):
    explicit = _explicit_runner_os(runner_os, contexts)
    return explicit is not None and explicit.casefold() == 'windows'


def _job_runner_os(job, *, needs=None, contexts=None):
    explicit = _explicit_runner_os(contexts=contexts)
    if not isinstance(job, dict) or 'runs-on' not in job:
        if explicit is not None:
            return 'Windows' if explicit.casefold() == 'windows' else explicit
        return None
    expr_contexts = {
        key: val for key, val in (contexts or {}).items()
        if str(key).lower() not in ('runner', 'runner_os')
    }
    base_ctx = {'github': {}, 'matrix': {}, 'needs': needs or {}, **expr_contexts}
    labels = runner_labels(job)
    raw_runs_on = job['runs-on']
    if isinstance(raw_runs_on, dict) and 'group' in raw_runs_on:
        resolved_group = resolve_github_expression(raw_runs_on['group'], base_ctx)
        if not isinstance(resolved_group, str) or not resolved_group.strip():
            raise ValueError(f'expected non-empty runs-on group after evaluation, got {resolved_group!r}')
    resolved_labels = [resolve_github_expression(label, base_ctx) for label in labels]
    if explicit is not None:
        return 'Windows' if explicit.casefold() == 'windows' else explicit
    if any(label.casefold() in _WINDOWS_RUNNER_LABELS for label in resolved_labels):
        return 'Windows'
    return 'Linux'


def workflow_run_settings(job_name, step_name, *, needs=None, contexts=None, filename='ci.yml'):
    workflow = _as_mapping(parsed_workflow(filename))
    job = _as_mapping(_as_mapping(workflow.get('jobs')).get(job_name))
    step = _as_mapping(named_step(job, step_name))
    runner_os = _job_runner_os(job, needs=needs, contexts=contexts)
    windows_env = _is_windows_runner_os(runner_os, contexts)
    settings = {}
    env = {}
    for scope in (workflow, job):
        defaults = _as_mapping(scope.get('defaults'))
        settings.update(_as_mapping(defaults.get('run')))
    for scope in (workflow, job, step):
        context = {'github': {}, 'matrix': {}, 'needs': needs or {}, **(contexts or {}), 'env': dict(env)}
        for key, expr in _as_mapping(scope.get('env')).items():
            if isinstance(expr, (dict, list)):
                raise ValueError(f'expected scalar env value for {key!r}, got {expr!r}')
            resolved = resolve_github_expression(expr, context, runner_os=runner_os)
            if windows_env:
                existing_key = next((k for k in env if str(k).casefold() == str(key).casefold()), key)
                if existing_key != key and existing_key in env:
                    del env[existing_key]
            env[key] = resolved
    return {**settings, **step, 'shell': step.get('shell') or settings.get('shell'), 'env': env, 'runner_os': runner_os}


def resolve_effective_shell(job_name, step_name, filename='ci.yml'):
    return workflow_run_settings(job_name, step_name, filename=filename).get('shell')


def _script_extension_for_shell(shell_spec):
    command, _, _ = (shell_spec or '').strip().partition(' ')
    if Path(command).stem.lower() in ('pwsh', 'powershell'):
        return '.ps1'
    return '.sh'


def _shell_argv_for_script(shell_spec, script_path):
    path = str(script_path)
    if not shell_spec:
        # ScriptHandler selects bash if present but uses the sh argument format.
        shell_spec = 'bash -e {0}' if shutil.which('bash') else 'sh -e {0}'
    command, _, arguments = shell_spec.strip().partition(' ')
    executable = shutil.which(command)
    if executable is None:
        raise FileNotFoundError(f'workflow shell not found: {command}')
    if not arguments.strip():
        defaults = {'bash': ['--noprofile', '--norc', '-e', '-o', 'pipefail'],
                    'sh': ['-e'], 'python': [], 'pwsh': ['-command', ". '{0}'"]}
        if command.lower() in defaults:
            default_args = defaults[command.lower()]
            if any('{0}' in arg for arg in default_args):
                return [executable, *(arg.replace('{0}', path) for arg in default_args)]
            return [executable, *default_args, path]
    if '{0}' not in arguments:
        raise ValueError(f'expected a built-in Linux shell or a format containing {{0}}: {shell_spec!r}')
    return [executable, *(arg.replace('{0}', path) for arg in shlex.split(arguments))]


def run_workflow_script(settings, *, workspace=None, runner_env=None):
    # A job with no checkout starts in an empty workspace. The runner writes
    # the script outside it and supplies the workspace path to the process.
    with tempfile.TemporaryDirectory() as temp:
        if workspace is None:
            workspace = Path(temp) / 'workspace'
            workspace.mkdir()
        shell_spec = settings.get('shell')
        script_path = Path(temp) / f'step{_script_extension_for_shell(shell_spec)}'
        script_path.write_text(settings['run'])
        env = {'PATH': os.environ.get('PATH', os.defpath), 'TMPDIR': temp, **(runner_env or {}),
               **settings['env'], 'GITHUB_WORKSPACE': str(workspace), 'RUNNER_TEMP': temp}
        return subprocess.run(
            _shell_argv_for_script(shell_spec, script_path),
            cwd=workspace / settings.get('working-directory', ''), env=env,
            capture_output=True, text=True,
        )


def interfering_host_bash_env(directory):
    startup = Path(directory) / 'host-bash-env'
    startup.write_text('exit 48\n')
    return mock.patch.dict(os.environ, {'BASH_ENV': str(startup)})


def _parse_github_number(raw):
    lower = raw.lower()
    if lower.startswith('0x'):
        return int(raw, 16)
    if '.' in raw or 'e' in lower:
        return float(raw)
    return int(raw)


def _github_to_number(value):
    if value is None:
        return 0.0
    if isinstance(value, bool):
        return 1.0 if value else 0.0
    if isinstance(value, (int, float)):
        return float(value)
    if isinstance(value, str):
        stripped = value.strip()
        if stripped == '':
            return 0.0
        if _COERCE_HEX_RE.match(stripped):
            return float(int(stripped, 16))
        if _COERCE_DEC_RE.match(stripped):
            return float(stripped)
        return math.nan
    return math.nan


def _github_is_truthy(value):
    if value is None:
        return False
    if isinstance(value, bool):
        return value
    if isinstance(value, (int, float)):
        return not math.isnan(value) and value != 0
    if isinstance(value, str):
        return len(value) > 0
    return True


def _strip_github_expression_wrapper(value):
    expr = value.strip()
    if expr.startswith('${{'):
        end = _find_github_expression_end(expr, 3)
        if end + 2 != len(expr):
            raise ValueError(f'unexpected trailing content after GitHub expression: {expr[end + 2:]!r}')
        return expr[3:end].strip()
    return expr


def _normalize_github_expression(value):
    return re.sub(r'\s+', '', _strip_github_expression_wrapper(value))


def _tokenize_github_expression(expr):
    tokens = []
    pos = 0
    length = len(expr)
    while pos < length:
        if expr[pos].isspace():
            pos += 1
            continue
        if tokens and tokens[-1][0] == 'DOT':
            seg_match = _PROP_SEG_RE.match(expr, pos)
            if not seg_match:
                raise ValueError(f'expected property name after "." at position {pos}: {expr[pos:]!r}')
            tokens.append(('PROP', seg_match.group('SEG')))
            pos = seg_match.end()
            continue
        if expr[pos] == '.' and tokens and tokens[-1][0] in ('IDENT', 'PROP', 'RPAREN'):
            tokens.append(('DOT', '.'))
            pos += 1
            continue
        match = _EXPR_TOKEN_RE.match(expr, pos)
        if not match or match.end() == pos:
            raise ValueError(f'unexpected token in GitHub expression at position {pos}: {expr[pos:]!r}')
        kind = match.lastgroup
        raw = match.group(kind)
        if kind == 'STRING':
            tokens.append((kind, raw[1:-1].replace("''", "'")))
        elif kind == 'NUMBER':
            tokens.append((kind, _parse_github_number(raw)))
        else:
            tokens.append((kind, raw))
        pos = match.end()
    if not tokens:
        raise ValueError('empty GitHub expression')
    return tokens


class _GitHubExpressionParser:
    def __init__(self, tokens):
        self.tokens = tokens
        self.pos = 0

    def peek(self):
        return self.tokens[self.pos] if self.pos < len(self.tokens) else None

    def consume(self, expected_kind=None, expected_value=None):
        tok = self.peek()
        if tok is None:
            raise ValueError('unexpected end of GitHub expression')
        if expected_kind is not None and tok[0] != expected_kind:
            raise ValueError(f'expected {expected_kind}, got {tok!r}')
        if expected_value is not None and tok[1] != expected_value:
            raise ValueError(f'expected {expected_value!r}, got {tok!r}')
        self.pos += 1
        return tok

    def parse(self):
        node = self.parse_or()
        if self.peek() is not None:
            raise ValueError(f'unexpected trailing token in GitHub expression: {self.peek()!r}')
        return node

    def parse_or(self):
        node = self.parse_and()
        while self.peek() == ('OP', '||'):
            self.consume('OP', '||')
            node = ('or', node, self.parse_and())
        return node

    def parse_and(self):
        node = self.parse_eq()
        while self.peek() == ('OP', '&&'):
            self.consume('OP', '&&')
            node = ('and', node, self.parse_eq())
        return node

    def parse_eq(self):
        node = self.parse_unary()
        while self.peek() in (('OP', '=='), ('OP', '!=')):
            _, op = self.consume('OP')
            node = (op, node, self.parse_unary())
        return node

    def parse_unary(self):
        if self.peek() == ('OP', '!'):
            self.consume('OP', '!')
            return ('not', self.parse_unary())
        return self.parse_primary()

    def parse_primary(self):
        tok = self.peek()
        if tok is None:
            raise ValueError('unexpected end of GitHub expression')
        kind, val = tok
        if kind == 'LPAREN':
            self.consume('LPAREN')
            node = self.parse_or()
            self.consume('RPAREN')
            return node
        if kind in ('STRING', 'NUMBER'):
            self.consume(kind)
            return ('literal', val)
        if kind == 'IDENT':
            self.consume('IDENT')
            if self.peek() == ('LPAREN', '('):
                self.consume('LPAREN')
                args = []
                if self.peek() != ('RPAREN', ')'):
                    args.append(self.parse_or())
                    while self.peek() == ('COMMA', ','):
                        self.consume('COMMA')
                        args.append(self.parse_or())
                self.consume('RPAREN')
                return ('call', val, tuple(args))
            lower = val.lower()
            if lower in ('true', 'false', 'null') and self.peek() != ('DOT', '.'):
                literal_map = {'true': True, 'false': False, 'null': None}
                return ('literal', literal_map[lower])
            parts = [val]
            while self.peek() == ('DOT', '.'):
                self.consume('DOT')
                _, seg = self.consume('PROP')
                parts.append(seg)
            return ('context', tuple(parts))
        raise ValueError(f'unexpected token in GitHub expression: {tok!r}')


def _parse_github_expression(value):
    expr = _strip_github_expression_wrapper(value)
    return _GitHubExpressionParser(_tokenize_github_expression(expr)).parse()


def _iter_ast_nodes(node):
    yield node
    kind = node[0]
    if kind in ('or', 'and', '==', '!='):
        yield from _iter_ast_nodes(node[1])
        yield from _iter_ast_nodes(node[2])
    elif kind == 'not':
        yield from _iter_ast_nodes(node[1])
    elif kind == 'call':
        for arg in node[2]:
            yield from _iter_ast_nodes(arg)


def _has_status_check_function(value):
    ast = _parse_github_expression(value)
    return any(
        node[0] == 'call' and node[1].lower() in _STATUS_CHECK_FUNCTIONS
        for node in _iter_ast_nodes(ast)
    )


def _github_values_equal(left, right):
    if isinstance(left, str) and isinstance(right, str):
        return left.casefold() == right.casefold()
    if isinstance(left, bool) and isinstance(right, bool):
        return left is right
    if left is None and right is None:
        return True
    left_is_num = isinstance(left, (int, float)) and not isinstance(left, bool)
    right_is_num = isinstance(right, (int, float)) and not isinstance(right, bool)
    if left_is_num and right_is_num:
        if math.isnan(left) or math.isnan(right):
            return False
        return float(left) == float(right)
    if isinstance(left, (dict, list, tuple, set)) and isinstance(right, (dict, list, tuple, set)):
        return left is right
    left_num = _github_to_number(left)
    right_num = _github_to_number(right)
    if math.isnan(left_num) or math.isnan(right_num):
        return False
    return left_num == right_num


def _extract_ancestor_statuses(ancestor_results):
    if isinstance(ancestor_results, dict):
        statuses = []
        for entry in ancestor_results.values():
            if isinstance(entry, dict):
                statuses.append(str(entry.get('result', '')))
            else:
                statuses.append(str(entry))
        return statuses
    return [str(item) for item in ancestor_results]


def _resolve_context_path(parts, contexts, *, runner_os=None):
    current = contexts
    is_env_context = bool(parts) and str(parts[0]).casefold() == 'env'
    env_case_sensitive = is_env_context and not _is_windows_runner_os(runner_os, contexts)
    for idx, part in enumerate(parts):
        if not isinstance(current, dict):
            return ''
        matched_key = None
        if idx > 0 and env_case_sensitive:
            for key in current:
                if str(key) == part:
                    matched_key = key
                    break
        else:
            target = part.casefold()
            for key in current:
                if str(key).casefold() == target:
                    matched_key = key
                    break
        if matched_key is None:
            return ''
        current = current[matched_key]
    return '' if current is None else current


def evaluate_github_expression(value, *, needs=None, cancelled=False, ancestor_results=None, contexts=None, step_status=None, runner_os=None):
    ast = _parse_github_expression(value)
    needs_map = {} if needs is None else needs
    provided_contexts = {'needs': needs_map}
    if contexts is not None:
        for key, val in contexts.items():
            key_lower = str(key).lower()
            if key_lower not in _SUPPORTED_EXPR_CONTEXTS:
                raise ValueError(f'unsupported GitHub expression context: {key!r}')
            provided_contexts[key_lower] = val

    all_needs_success = all(
        isinstance(info, dict) and str(info.get('result', '')).casefold() == 'success'
        for info in needs_map.values()
    )
    any_direct_failure = any(
        isinstance(info, dict) and str(info.get('result', '')).casefold() == 'failure'
        for info in needs_map.values()
    )

    # Validate the entire AST up front so short-circuiting cannot hide unknown
    # functions, unknown/unavailable contexts, bad arity, or missing ancestor
    # conclusions required by failure().
    for node in _iter_ast_nodes(ast):
        kind = node[0]
        if kind == 'call':
            func_name, args = node[1], node[2]
            func_lower = func_name.lower()
            if func_lower not in _SUPPORTED_EXPR_FUNCTIONS:
                raise ValueError(f'unsupported GitHub expression function: {func_name!r}')
            expected_arity = _SUPPORTED_EXPR_FUNCTIONS[func_lower]
            if len(args) != expected_arity:
                raise ValueError(
                    f'GitHub expression function {func_name!r} expects {expected_arity} arguments, got {len(args)}'
                )
            if func_lower == 'failure' and step_status is None and not any_direct_failure and needs_map and ancestor_results is None:
                raise ValueError(
                    'failure() requires explicit ancestor_results when no direct need failed'
                )
        elif kind == 'context':
            root = node[1][0]
            root_lower = root.lower()
            if root_lower not in _SUPPORTED_EXPR_CONTEXTS:
                raise ValueError(f'unsupported GitHub expression context: {root!r}')
            if root_lower not in provided_contexts:
                raise ValueError(
                    f'GitHub expression context {root!r} is not available in this evaluation'
                )

    ancestor_statuses = None if ancestor_results is None else _extract_ancestor_statuses(ancestor_results)
    any_ancestor_failure = any_direct_failure or (
        ancestor_statuses is not None
        and any(status.casefold() == 'failure' for status in ancestor_statuses)
    )

    def eval_node(node):
        kind = node[0]
        if kind == 'literal':
            return node[1]
        if kind == 'context':
            return _resolve_context_path(node[1], provided_contexts, runner_os=runner_os)
        if kind == 'not':
            return not _github_is_truthy(eval_node(node[1]))
        if kind == 'and':
            left_val = eval_node(node[1])
            return eval_node(node[2]) if _github_is_truthy(left_val) else left_val
        if kind == 'or':
            left_val = eval_node(node[1])
            return left_val if _github_is_truthy(left_val) else eval_node(node[2])
        if kind == '==':
            return _github_values_equal(eval_node(node[1]), eval_node(node[2]))
        if kind == '!=':
            return not _github_values_equal(eval_node(node[1]), eval_node(node[2]))
        if kind == 'call':
            func_lower = node[1].lower()
            args = [eval_node(arg) for arg in node[2]]
            if step_status is not None and func_lower in ('success', 'failure', 'cancelled'):
                return step_status == func_lower
            if func_lower == 'always':
                return True
            if func_lower == 'cancelled':
                return bool(cancelled)
            if func_lower == 'success':
                return (not cancelled) and all_needs_success
            if func_lower == 'failure':
                if not any_direct_failure and needs_map and ancestor_results is None:
                    raise ValueError(
                        'failure() requires explicit ancestor_results when no direct need failed'
                    )
                return (not cancelled) and any_ancestor_failure
            if func_lower == 'contains':
                haystack, needle = args
                if isinstance(haystack, (list, tuple, set)):
                    return any(_github_values_equal(elem, needle) for elem in haystack)
                return str(needle).casefold() in str(haystack).casefold()
            if func_lower == 'startswith':
                return str(args[0]).casefold().startswith(str(args[1]).casefold())
            if func_lower == 'endswith':
                return str(args[0]).casefold().endswith(str(args[1]).casefold())
        raise ValueError(f'unsupported AST node: {node!r}')

    return eval_node(ast)


def evaluate_workflow_if(condition, needs, *, cancelled=False, ancestor_results=None, step_status=None, contexts=None, runner_os=None):
    condition = condition or 'success()'
    if not _has_status_check_function(condition):
        condition = f'success() && ({_strip_github_expression_wrapper(condition)})'
    return _github_is_truthy(evaluate_github_expression(
        condition, needs=needs, cancelled=cancelled,
        ancestor_results=ancestor_results, step_status=step_status, contexts=contexts,
        runner_os=runner_os,
    ))


def expression(
    value,
    result='success',
    docs_only='false',
    cancelled=False,
    go_checks_result='success',
    needs=None,
    ancestor_results=None,
    contexts=None,
):
    if needs is None:
        needs = {
            'classify': {'result': result, 'outputs': {'docs_only': docs_only}},
            'go_checks': {'result': go_checks_result},
        }
    return evaluate_github_expression(
        value,
        needs=needs,
        cancelled=cancelled,
        ancestor_results=ancestor_results,
        contexts=contexts,
    )


class GitFixture(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        # Synthetic objects and origin refs live only in this temporary repo;
        # no operations touch the user's repository refs or history.
        subprocess.run(['git', 'init', '-q', str(self.root)], check=True)
        self.cwd = Path.cwd()
        os.chdir(self.root)
        self.addCleanup(os.chdir, self.cwd)
        helper = self.root / 'scripts/ci/doc_scope.py'
        helper.parent.mkdir(parents=True)
        helper.write_text((ROOT / 'scripts/ci/doc_scope.py').read_text())
        self.base = self.state({'docs/specs/cli-design.md': '# Original\n',
                                'cmd/main.go': 'package main\n'})

    def state(self, files, parent=None, modes=None):
        DIFF.git('read-tree', '--empty')
        for name, text in files.items():
            blob = subprocess.check_output(['git', 'hash-object', '-w', '--stdin'], input=text.encode()).decode().strip()
            mode = (modes or {}).get(name, '100644')
            DIFF.git('update-index', '--add', '--cacheinfo', f'{mode},{blob},{name}')
        tree = DIFF.git('write-tree').decode().strip()
        env = dict(os.environ, GIT_AUTHOR_NAME='Fixture', GIT_AUTHOR_EMAIL='fixture@example.invalid',
                   GIT_COMMITTER_NAME='Fixture', GIT_COMMITTER_EMAIL='fixture@example.invalid')
        command = ['git', 'commit-tree', tree, '-m', 'Synthetic diff test']
        if parent:
            command += ['-p', parent]
        return subprocess.check_output(command, env=env).decode().strip()

class DiffClassificationTest(GitFixture):
    def result(self, changes, event='push', modes=None):
        files = {'docs/specs/cli-design.md': '# Original\n', 'cmd/main.go': 'package main\n'}
        files.update(changes)
        head = self.state(files, self.base, modes)
        return DIFF.classify(self.base, head, event)['docs_only']

    def test_documents_add_modify_and_pr(self):
        for event in ('push', 'pull_request'):
            self.assertEqual(self.result({'docs/specs/cli-design.md': '# Changed\n',
                                          '.agent-instructions/new.md': '# Rule\n'}, event), 'true')

    def test_code_workflow_scripts_deps_build_fixtures_generated_unknown_mixed(self):
        for name in ('cmd/main.go', '.github/workflows/ci.yml', 'scripts/test.md',
                     'go.mod', 'Makefile', 'docs/fixtures/input.md',
                     'docs/generated/catalog.md', 'docs/generatedinputs/catalog.md', 'docs/topics/demo/ux/prototype/README.md',
                     'unknown.md', 'tests/README.md'):
            with self.subTest(name=name):
                self.assertEqual(self.result({name: 'changed\n', 'docs/README.md': '# Doc\n'}), 'false')

    def test_rename_and_delete(self):
        for files in ({'docs/specs/new.md': '# Original\n', 'cmd/main.go': 'package main\n'},
                      {'cmd/main.go': 'package main\n'}):
            head = self.state(files, self.base)
            self.assertEqual(DIFF.classify(self.base, head, 'push')['docs_only'], 'false')

    def test_new_branch_missing_base_unknown_event_empty_and_nonancestor(self):
        head = self.state({'docs/README.md': '# Doc\n'}, self.base)
        for base, event in (('0' * 40, 'push'), ('f' * 40, 'push'), (self.base, 'unknown')):
            self.assertEqual(DIFF.classify(base, head, event)['docs_only'], 'false')
        self.assertEqual(DIFF.classify(self.base, self.base, 'push')['docs_only'], 'false')
        self.assertEqual(DIFF.classify(head, self.base, 'push')['docs_only'], 'false')

    def test_executable_and_symlink_markdown(self):
        for mode in ('100755', '120000'):
            self.assertEqual(self.result({'docs/new.md': 'target\n'}, modes={'docs/new.md': mode}), 'false')

    def test_diff_failure_falls_back_to_full(self):
        with mock.patch.object(DIFF, 'git', side_effect=OSError('unavailable')):
            self.assertEqual(DIFF.classify(self.base, self.base, 'push')['docs_only'], 'false')

    def test_event_entry_emits_fail_safe_for_invalid_event(self):
        event = self.root / 'event.json'
        output = self.root / 'output'
        event.write_text('{invalid json')
        result = subprocess.run([sys.executable, str(ROOT / 'scripts/ci/classify_diff.py'),
                                 '--event', str(event), '--event-name', 'push', '--output', str(output)],
                                capture_output=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn('docs_only=false', output.read_text())

    def run_classify_step(self, before, after):
        event = {'before': before, 'after': after}
        event_path = self.root / 'event.json'
        event_path.write_text(json.dumps(event))
        output = self.root / 'output'
        github = {'event_name': 'push', 'event': event, 'sha': after}
        settings = workflow_run_settings('classify', 'Classify using the trusted base script', contexts={'github': github})
        with interfering_host_bash_env(self.root):
            result = run_workflow_script(settings, workspace=self.root,
                                         runner_env={'GITHUB_OUTPUT': str(output), 'GITHUB_EVENT_PATH': str(event_path)})
        self.assertEqual(result.returncode, 0, result.stderr)
        return dict(line.split('=', 1) for line in output.read_text().splitlines()), result.stdout

    def test_ci_first_deployment_establishes_doc_range_without_base_classifier(self):
        head = self.state({'docs/README.md': '# Doc\n', 'cmd/main.go': 'changed code\n'}, self.base)
        values, _ = self.run_classify_step(self.base, head)
        self.assertEqual(values['docs_only'], 'false')
        self.assertEqual(values['doc_base'], self.base)
        self.assertEqual(values['doc_head'], head)

    def test_ci_new_branch_without_trusted_default_reports_unknown(self):
        values, stdout = self.run_classify_step('0' * 40, self.base)
        self.assertEqual(values['docs_only'], 'false')
        self.assertEqual(values['doc_scope_known'], 'false')
        self.assertIn('范围未知', stdout)

    def test_pr_uses_merge_base_not_base_tip(self):
        head = self.state({'docs/README.md': '# Doc\n', 'cmd/main.go': 'package main\n',
                           'docs/specs/cli-design.md': '# Original\n'}, self.base)
        base_tip = self.state({'cmd/main.go': 'new product code\n',
                               'docs/specs/cli-design.md': '# Original\n'}, self.base)
        result = DIFF.classify(base_tip, head, 'pull_request')
        self.assertEqual(result['base'], self.base)
        self.assertEqual(result['docs_only'], 'true')


class ReviewValidationTest(unittest.TestCase):
    def test_real_pr40_double_gate_red_single_gate_green(self):
        # Exact Round 2 from local committed history: b0c6402 and 787cf40.
        directory = Path(__file__).parent / 'fixtures'
        red = (directory / 'pr40-round2-double-gate.txt').read_text()
        green = (directory / 'pr40-round2-single-gate.txt').read_text()
        self.assertEqual(REVIEW.section_state(REVIEW.review_sections(red)[-1]), (None, None))
        self.assertTrue(REVIEW.validation_errors(red))
        self.assertEqual(REVIEW.section_state(REVIEW.review_sections(green)[-1]), ('PASS', 'VERIFIED'))
        self.assertEqual(REVIEW.validation_errors(green), [])

    def test_old_fail_then_pass_and_old_missing_gate(self):
        for old in ('FAIL', 'REOPEN'):
            self.assertEqual(REVIEW.validation_errors(f'## Round 1\nVerdict: {old}\n'
                '## Round 2\nVerdict: PASS\nCompletion gate: NOT_VERIFIED\n', f'## Round 1\nVerdict: {old}\n'), [])

    def test_conflicts_in_earlier_complete_round_still_fail(self):
        text = '## Round 1\nVerdict: FAIL\nVerdict: PASS\nCompletion gate: FAILED\n'
        self.assertTrue(REVIEW.validation_errors(text + '## Round 2\nVerdict: PASS\nCompletion gate: VERIFIED\n'))

    def test_unchanged_history_preserved_but_changed_round_checked(self):
        old = '## Round 1\nVerdict: FAIL\nCompletion gate: FAILED\nCompletion gate: VERIFIED\n'
        final = '## Round 2\nVerdict: PASS\nCompletion gate: VERIFIED\n'
        self.assertEqual(REVIEW.validation_errors(old + final, old), [])
        self.assertTrue(REVIEW.validation_errors(old.replace('Verdict: FAIL', 'Verdict: PASS') + final, old))
        self.assertEqual(REVIEW.validation_errors(old, old), [])
        self.assertTrue(REVIEW.validation_notes(old, old))  # no CE certification

    def test_incomplete_final_and_unknown_values_fail(self):
        for tail in ('Work pending.', 'Verdict: MAYBE\nCompletion gate: VERIFIED',
                     'Verdict: PASS', 'Verdict: PASS\nCompletion gate: GREEN'):
            self.assertTrue(REVIEW.validation_errors('## Round 1\nVerdict: PASS\nCompletion gate: VERIFIED\n## Round 2\n' + tail))

    def test_localized_nested_fences_and_nonverified_gate_are_valid(self):
        self.assertEqual(REVIEW.validation_errors('## 复评 — 第2轮\n## 📋 复评\n'
            '✅ **复评结论**：**PASS**\n### Summary\n证据门禁：BLOCKED\n'
            '```markdown\n## Round 9\nVerdict: FAIL\nCompletion gate: VERIFIED\n```\n'), [])

    def test_cli_does_not_load_hook_or_write(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            path = root / 'docs/topics/demo/reviews/tasks.md'
            path.parent.mkdir(parents=True)
            path.write_text('## Round 1\nVerdict: FAIL\nCompletion gate: FAILED\n')
            before = path.read_bytes()
            result = subprocess.run([sys.executable, str(ROOT / 'scripts/check-review-records.py'),
                                     'docs/topics/demo/reviews/tasks.md'], cwd=root, capture_output=True)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(path.read_bytes(), before)
            self.assertEqual(sorted(p.relative_to(root).as_posix() for p in root.rglob('*') if p.is_file()),
                             ['docs/topics/demo/reviews/tasks.md'])
        self.assertNotIn('beads-consistency', (ROOT / 'scripts/check-review-records.py').read_text())


class DocumentationChecksTest(unittest.TestCase):
    def test_links_missing_targets_and_anchors(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            target = root / 'target.md'
            target.write_text('# 中文 Contract\n## Same\n## Same\n')
            (root / 'with__underscores.md').write_text('# Heading\n')
            source = root / 'source.md'
            source.write_text('[ok](target.md#中文-contract)\n[duplicate](target.md#same-1)\n'
                              '[literal](with__underscores.md)\n[external](https://example.invalid)\n```md\n[example](absent.md)\n```\n')
            self.assertEqual(DOCS.link_errors(source, root), [])
            source.write_text('[bad](missing.md)\n[bad](target.md#absent)\n')
            self.assertEqual(len(DOCS.link_errors(source, root)), 2)

    def test_topic_audit_checks_complete_selected_set(self):
        result = subprocess.run(['bash', 'scripts/check-topic-docs.sh', '--archive', 'v0-6-5-contract'],
                                cwd=ROOT, capture_output=True)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)

    def test_topic_audit_rejects_invalid_selector(self):
        result = subprocess.run(['bash', 'scripts/check-topic-docs.sh', '../escape'], cwd=ROOT, capture_output=True)
        self.assertEqual(result.returncode, 2)


class CIWorkflowPathTest(unittest.TestCase):
    def test_docs_mixed_and_classifier_failure_all_run_documentation_and_tests(self):
        job = workflow_job('documentation')
        condition = re.search(r'^    if: (.+)$', job, re.M).group(1)
        for result, docs_only in [('success', 'true'), ('success', 'false'), ('failure', '')]:
            self.assertTrue(expression(condition, result, docs_only))
        self.assertFalse(expression(condition, cancelled=True))
        self.assertIn('run: make check-ci-docs-tools', job)
        self.assertNotIn('if:', workflow_step('documentation', 'Test CI classification and review-record tools'))

    def test_doc_scope_shell_passes_known_range_or_blocks_unknown(self):
        with tempfile.TemporaryDirectory() as temp, interfering_host_bash_env(temp):
            root = Path(temp)
            (root / 'scripts').mkdir()
            (root / 'scripts/check-docs.py').write_text('import sys\nprint(repr(sys.argv[1:]))\n')
            for base, head, expected in [('a', 'b', "['--base', 'a', '--head', 'b']"), ('', '', None)]:
                outputs = {'doc_base': base, 'doc_head': head, 'doc_scope_known': 'true' if base else 'false',
                           'doc_reason': '范围未知'}
                settings = workflow_run_settings('documentation', 'Document L0, links, document set and review records',
                                                 needs={'classify': {'outputs': outputs}})
                result = run_workflow_script(settings, workspace=root)
                if expected:
                    self.assertEqual(result.returncode, 0, result.stderr)
                    self.assertIn(expected, result.stdout)
                else:
                    self.assertNotEqual(result.returncode, 0)
                    self.assertIn('范围未知', result.stderr)
                    self.assertNotIn('--all', result.stdout)

    def test_both_required_jobs_fail_when_documentation_fails(self):
        for job, step in [('verify', 'Require applicable verification'), ('desktop', 'Require documentation checks')]:
            for status in ['success', 'failure', 'cancelled', 'skipped']:
                needs = {
                    'classify': {'result': 'success', 'outputs': {'docs_only': 'true'}},
                    'documentation': {'result': status},
                    'portable': {'result': 'skipped'},
                    'go_checks': {'result': 'skipped'},
                    'race': {'result': 'skipped', 'outputs': {}},
                }
                self.assertTrue(evaluate_workflow_if(parsed_workflow()['jobs'][job].get('if'), needs))
                result = run_workflow_script(workflow_run_settings(job, step, needs=needs))
                self.assertEqual(result.returncode == 0, status == 'success')

    def test_ci_triggers_push_only_on_integration_branches_and_keeps_pull_request(self):
        on_section = workflow_on('ci.yml')
        self.assertIn('  pull_request:', on_section)
        push_block = workflow_event_block('ci.yml', 'push')
        pr_block = workflow_event_block('ci.yml', 'pull_request')
        self.assertIsNotNone(push_block)
        self.assertIsNotNone(pr_block)
        push_branches = workflow_list_items(push_block, 'branches')
        self.assertEqual(push_branches, ['main', 'release/**'])
        self.assertNotIn('**', push_branches)
        self.assertIsNone(workflow_optional_list_items(push_block, 'branches-ignore'))
        self.assertIsNone(workflow_optional_list_items(pr_block, 'branches'))
        self.assertIsNone(workflow_optional_list_items(pr_block, 'branches-ignore'))
        for branch in ('main', 'release/v0.7.5'):
            self.assertTrue(workflow_event_triggers('ci.yml', 'push', branch, ['cmd/main.go']))
            self.assertTrue(workflow_event_triggers('ci.yml', 'pull_request', branch, ['cmd/main.go']))
        for branch in ('feature/demo', 'fix/ci-duplicate-runs-and-macos-concurrency'):
            self.assertFalse(workflow_event_triggers('ci.yml', 'push', branch, ['cmd/main.go']))

    def test_split_product_jobs_require_a_trusted_documentation_only_classification_to_skip(self):
        for job in ('portable', 'go_checks', 'race'):
            condition = re.search(r'^    if: (.+)$', workflow_job(job), re.M).group(1)
            with self.subTest(job=job):
                self.assertFalse(expression(condition, 'success', 'true'))
                self.assertTrue(expression(condition, 'success', 'false'))
                self.assertTrue(expression(condition, 'failure', 'true'))
                self.assertFalse(expression(condition, cancelled=True))

    def test_race_depends_on_go_checks_and_caps_parallelism_to_two_shards(self):
        job = workflow_job('race')
        self.assertIn('    needs: [classify, go_checks]\n', job)
        self.assertIn('      max-parallel: 2\n', job)
        self.assertIn('        shard: [0, 1]\n', job)
        self.assertNotIn('shard_2:', job)
        condition = re.search(r'^    if: (.+)$', job, re.M).group(1)
        for status in ('failure', 'skipped', 'cancelled'):
            with self.subTest(go_checks_result=status):
                self.assertFalse(expression(condition, 'success', 'false', go_checks_result=status))
                self.assertFalse(expression(condition, 'failure', 'true', go_checks_result=status))

    def _run_verify_step(self, needs):
        settings = workflow_run_settings('verify', 'Require applicable verification', needs=needs)
        return run_workflow_script(settings).returncode

    def _run_verify_via_workflow(self, needs, *, cancelled=False, ancestor_results=None):
        job = parsed_workflow()['jobs']['verify']
        if not evaluate_workflow_if(job.get('if'), needs, cancelled=cancelled, ancestor_results=ancestor_results):
            return False, 'skipped', None

        settings = workflow_run_settings('verify', 'Require applicable verification', needs=needs)
        # StepsRunner starts a new job at success. Failed upstream jobs remain
        # available through needs, but are not this first step's job status.
        step_status = 'cancelled' if cancelled else 'success'
        if not evaluate_workflow_if(settings.get('if'), needs, step_status=step_status, contexts={'env': settings['env']}, runner_os=settings.get('runner_os')):
            return True, 'success', None
        try:
            code = run_workflow_script(settings).returncode
        except OSError:
            # A step that cannot launch (e.g. a missing cwd) has no process exit.
            code = None
        continue_on_error = any(
            _github_is_truthy(evaluate_github_expression(value, needs=needs, cancelled=cancelled,
                                                         ancestor_results=ancestor_results, step_status=status))
            for value, status in ((job.get('continue-on-error'), None),
                                  (settings.get('continue-on-error'), step_status))
            if value is not None
        )
        conclusion = 'success' if (code == 0 or continue_on_error) else 'failure'
        return True, conclusion, code

    def test_race_shard_outputs_map_one_to_one_from_step_to_job_to_verify(self):
        race_job = workflow_job('race')
        shard_match = re.search(r'^        shard:\s*\[([^\]]+)\]\s*$', race_job, re.M)
        self.assertIsNotNone(shard_match)
        shards = [item.strip().strip("'\"") for item in shard_match.group(1).split(',') if item.strip()]
        self.assertEqual(shards, ['0', '1'])

        race_step = workflow_step('race', 'Race test package shard')
        step_id = re.search(r'^        id:\s*(\S+)\s*$', race_step, re.M).group(1)
        race_step_env = workflow_mapping(race_step, 'env', indent=8)
        self.assertEqual(race_step_env.get('SHARD'), '${{ matrix.shard }}')

        race_outputs = workflow_mapping(race_job, 'outputs', indent=4)
        self.assertEqual(
            race_outputs,
            {f'shard_{s}': f'${{{{ steps.{step_id}.outputs.shard_{s} }}}}' for s in shards},
        )

        verify_env = named_step(parsed_workflow()['jobs']['verify'], 'Require applicable verification')['env']
        expected_verify_env = {
            'CLASSIFY_RESULT': '${{ needs.classify.result }}',
            'DOCS_ONLY': '${{ needs.classify.outputs.docs_only }}',
            'DOCUMENTATION_RESULT': '${{ needs.documentation.result }}',
            'PORTABLE_RESULT': '${{ needs.portable.result }}',
            'GO_CHECKS_RESULT': '${{ needs.go_checks.result }}',
            'RACE_RESULT': '${{ needs.race.result }}',
            'RACE_SHARD_0': '${{ needs.race.outputs.shard_0 }}',
            'RACE_SHARD_1': '${{ needs.race.outputs.shard_1 }}',
        }
        self.assertEqual(verify_env, expected_verify_env)

        # Run the actual race step script with stubbed go/make to confirm step outputs per shard.
        step_outputs_by_shard = {}
        with tempfile.TemporaryDirectory() as temp, interfering_host_bash_env(temp):
            bin_dir = Path(temp) / 'bin'
            bin_dir.mkdir()
            go_stub = bin_dir / 'go'
            go_stub.write_text(
                '#!/usr/bin/env bash\n'
                'if [[ "$1" == "env" && "$2" == "GOCACHE" ]]; then\n'
                '  echo "/tmp/stub-gocache"\n'
                '  exit 0\n'
                'fi\n'
                'if [[ "$1" == "list" ]]; then\n'
                '  printf "%s\\n" example.test/agentdeck/cmd/agentdeck example.test/agentdeck/internal/backup example.test/agentdeck/internal/store\n'
                '  exit 0\n'
                'fi\n'
                'exit 1\n'
            )
            go_stub.chmod(0o755)
            make_stub = bin_dir / 'make'
            make_stub.write_text(
                '#!/usr/bin/env bash\n'
                '[[ "$1" == "test-race" ]] || exit 1\n'
                '[[ "$2" == GO_RACE_PACKAGES=* && -n "${2#GO_RACE_PACKAGES=}" ]] || exit 1\n'
                'exit 0\n'
            )
            make_stub.chmod(0o755)
            for shard in shards:
                out_file = Path(temp) / f'github_output_{shard}'
                out_file.write_text('')
                settings = workflow_run_settings('race', 'Race test package shard', contexts={'matrix': {'shard': shard}})
                runner_env = {'PATH': str(bin_dir) + os.pathsep + os.environ.get('PATH', os.defpath),
                              'GITHUB_OUTPUT': str(out_file)}
                completed = run_workflow_script(settings, workspace=ROOT, runner_env=runner_env)
                self.assertEqual(completed.returncode, 0, completed.stderr)
                parsed = dict(line.split('=', 1) for line in out_file.read_text().splitlines() if '=' in line)
                self.assertEqual(parsed, {f'shard_{shard}': 'success'})
                step_outputs_by_shard[shard] = parsed

        def evaluate_with_completed_shards(completed_shards):
            merged_step_outputs = {}
            for shard in completed_shards:
                merged_step_outputs.update(step_outputs_by_shard[shard])
            step_ctx = {'steps': {step_id: {'outputs': merged_step_outputs}}}
            evaluated_race_outputs = {
                key: resolve_github_expression(expr, step_ctx)
                for key, expr in race_outputs.items()
            }
            needs = {
                'classify': {'result': 'success', 'outputs': {'docs_only': 'false'}},
                'documentation': {'result': 'success'},
                'portable': {'result': 'success'},
                'go_checks': {'result': 'success'},
                'race': {'result': 'success', 'outputs': evaluated_race_outputs},
            }
            return self._run_verify_via_workflow(needs)

        will_run, conclusion, code = evaluate_with_completed_shards(['0', '1'])
        self.assertTrue(will_run)
        self.assertEqual(conclusion, 'success')
        self.assertEqual(code, 0)
        for subset in (['0'], ['1'], []):
            with self.subTest(completed_shards=subset):
                will_run, conclusion, code = evaluate_with_completed_shards(subset)
                self.assertTrue(will_run)
                self.assertEqual(conclusion, 'failure')
                self.assertNotEqual(code, 0)

    def test_verify_gate_structure_is_locked(self):
        lock_notice = '修改最终门禁结构、工作流顶层继承面、运行器、步骤或表达式需要同步更新本测试与评审'

        workflow = parsed_workflow()
        top_level_keys = workflow.keys()
        expected_top_level_keys = {'name', 'on', 'permissions', 'concurrency', 'jobs'}
        self.assertEqual(
            set(top_level_keys),
            expected_top_level_keys,
            f'ci.yml 顶层键集合 {set(top_level_keys)!r} 与认可白名单 {expected_top_level_keys!r} 不一致（拒绝顶层 defaults/env 等隐式继承面）；{lock_notice}',
        )

        verify_job = workflow['jobs']['verify']
        job_keys = verify_job.keys()
        expected_job_keys = {'needs', 'if', 'runs-on', 'steps'}
        self.assertEqual(
            set(job_keys),
            expected_job_keys,
            f'verify job 字段集合 {set(job_keys)!r} 与认可白名单 {expected_job_keys!r} 不一致（拒绝 continue-on-error/timeout-minutes/defaults 等未认可字段）；{lock_notice}',
        )

        runs_on_matches = runner_labels(verify_job)
        self.assertEqual(
            runs_on_matches,
            ['ubuntu-latest'],
            f"verify job runs-on {runs_on_matches!r} 与认可运行器 ['ubuntu-latest'] 不一致；{lock_notice}",
        )

        normalized_if = _normalize_github_expression(verify_job['if'])
        self.assertEqual(
            normalized_if,
            '!cancelled()',
            f"verify job 级 if 规范化后为 {normalized_if!r}，与认可门禁表达式 '!cancelled()' 不一致；{lock_notice}",
        )

        verify_needs = verify_job['needs']
        self.assertEqual(
            len(verify_needs),
            len(set(verify_needs)),
            f'verify job needs 存在重复项 {verify_needs!r}；{lock_notice}',
        )
        expected_needs = {'classify', 'documentation', 'portable', 'go_checks', 'race'}
        self.assertEqual(
            set(verify_needs),
            expected_needs,
            f'verify job needs 集合 {set(verify_needs)!r} 与认可集合 {expected_needs!r} 不一致；{lock_notice}',
        )

        steps = verify_job['steps']
        self.assertEqual(
            len(steps),
            1,
            f'verify job 必须且只能包含唯一最终门禁步骤，实际步骤数为 {len(steps)}；{lock_notice}',
        )
        only_step = steps[0]
        expected_step_keys = {'name', 'env', 'run'}
        self.assertEqual(
            set(only_step),
            expected_step_keys,
            f'verify 门禁步骤字段集合 {set(only_step)!r} 与认可白名单 {expected_step_keys!r} 不一致（拒绝 continue-on-error/if/shell/working-directory/timeout-minutes 等未认可字段）；{lock_notice}',
        )
        self.assertEqual(
            only_step.get('name'),
            'Require applicable verification',
            f'verify 唯一步骤名称 {only_step.get("name")!r} 与认可门禁步骤不一致；{lock_notice}',
        )
        self.assertTrue(
            workflow_run('verify', 'Require applicable verification').strip(),
            f'verify 唯一门禁步骤缺少 run 脚本；{lock_notice}',
        )

        verify_env = only_step['env']
        expected_verify_env = {
            'CLASSIFY_RESULT': '${{ needs.classify.result }}',
            'DOCS_ONLY': '${{ needs.classify.outputs.docs_only }}',
            'DOCUMENTATION_RESULT': '${{ needs.documentation.result }}',
            'PORTABLE_RESULT': '${{ needs.portable.result }}',
            'GO_CHECKS_RESULT': '${{ needs.go_checks.result }}',
            'RACE_RESULT': '${{ needs.race.result }}',
            'RACE_SHARD_0': '${{ needs.race.outputs.shard_0 }}',
            'RACE_SHARD_1': '${{ needs.race.outputs.shard_1 }}',
        }
        self.assertEqual(
            verify_env,
            expected_verify_env,
            f'verify 步骤 env 映射与认可映射不一致；{lock_notice}',
        )

    def test_verify_requires_every_product_job_on_mixed_or_unknown_scope(self):
        self.assertIn('    needs: [classify, documentation, portable, go_checks, race]\n', workflow_job('verify'))

        def base_needs():
            return {
                'classify': {'result': 'success', 'outputs': {'docs_only': 'false'}},
                'documentation': {'result': 'success'},
                'portable': {'result': 'success'},
                'go_checks': {'result': 'success'},
                'race': {'result': 'success', 'outputs': {'shard_0': 'success', 'shard_1': 'success'}},
            }

        will_run, conclusion, code = self._run_verify_via_workflow(base_needs())
        self.assertTrue(will_run)
        self.assertEqual(conclusion, 'success')
        self.assertEqual(code, 0)

        for upstream in ('classify', 'documentation', 'portable', 'go_checks', 'race'):
            for status in ('failure', 'skipped', 'cancelled'):
                with self.subTest(upstream=upstream, status=status):
                    needs = base_needs()
                    needs[upstream]['result'] = status
                    will_run, conclusion, code = self._run_verify_via_workflow(needs)
                    self.assertTrue(will_run)
                    self.assertEqual(conclusion, 'failure')
                    self.assertNotEqual(code, 0)

        # Fallback full product checks still run when classify fails/skips/cancels,
        # while verify must run and fail (never be skipped) without a successful classify job.
        for status in ('failure', 'skipped', 'cancelled'):
            for job in ('portable', 'go_checks', 'race'):
                condition = re.search(r'^    if: (.+)$', workflow_job(job), re.M).group(1)
                with self.subTest(fallback_job=job, classify_status=status):
                    self.assertTrue(expression(condition, status, 'true'))
            desktop_runner = re.search(r'^    runs-on: (.+)$', workflow_job('desktop'), re.M).group(1)
            self.assertEqual(expression(desktop_runner, status, 'true'), 'macos-26')

            needs = base_needs()
            needs['classify'] = {'result': status, 'outputs': {'docs_only': 'true'}}
            needs['portable']['result'] = 'skipped'
            needs['go_checks']['result'] = 'skipped'
            needs['race'] = {'result': 'skipped', 'outputs': {}}
            will_run, conclusion, code = self._run_verify_via_workflow(needs)
            self.assertTrue(will_run)
            self.assertEqual(conclusion, 'failure')
            self.assertNotEqual(code, 0)

    def test_verify_docs_only_fast_path_and_whole_run_cancellation(self):
        def docs_only_needs():
            return {
                'classify': {'result': 'success', 'outputs': {'docs_only': 'true'}},
                'documentation': {'result': 'success'},
                'portable': {'result': 'skipped'},
                'go_checks': {'result': 'skipped'},
                'race': {'result': 'skipped', 'outputs': {}},
            }

        # Valid docs-only skip passes verify.
        will_run, conclusion, code = self._run_verify_via_workflow(docs_only_needs())
        self.assertTrue(will_run)
        self.assertEqual(conclusion, 'success')
        self.assertEqual(code, 0)

        # Abnormal classify or documentation must cause verify to run and fail (never skip).
        for upstream in ('classify', 'documentation'):
            for status in ('failure', 'skipped', 'cancelled'):
                with self.subTest(docs_only_upstream=upstream, status=status):
                    needs = docs_only_needs()
                    needs[upstream]['result'] = status
                    will_run, conclusion, code = self._run_verify_via_workflow(needs)
                    self.assertTrue(will_run)
                    self.assertEqual(conclusion, 'failure')
                    self.assertNotEqual(code, 0)

        # Whole-run cancellation: verify's if (!cancelled()) evaluates to False, so
        # GitHub Actions skips verify (conclusion=skipped, workflow=cancelled, never green).
        all_cancelled = {
            'classify': {'result': 'cancelled', 'outputs': {'docs_only': ''}},
            'documentation': {'result': 'cancelled'},
            'portable': {'result': 'cancelled'},
            'go_checks': {'result': 'cancelled'},
            'race': {'result': 'cancelled', 'outputs': {}},
        }
        will_run, conclusion, code = self._run_verify_via_workflow(all_cancelled, cancelled=True)
        self.assertFalse(will_run)
        self.assertEqual(conclusion, 'skipped')
        self.assertIsNone(code)
        # Defense-in-depth: even if the step script runs directly with all-cancelled inputs,
        # it must exit non-zero.
        self.assertNotEqual(self._run_verify_step(all_cancelled), 0)

    def test_cask_real_brew_filters_main_push_paths_and_drops_stale_feature_branch(self):
        on_section = workflow_on('cask-real-brew.yml')
        self.assertIn('  workflow_dispatch:', on_section)
        push_block = workflow_event_block('cask-real-brew.yml', 'push')
        pr_block = workflow_event_block('cask-real-brew.yml', 'pull_request')
        self.assertIsNotNone(push_block)
        self.assertIsNotNone(pr_block)
        self.assertEqual(workflow_optional_list_items(push_block, 'branches'), ['main'])
        self.assertIsNone(workflow_optional_list_items(push_block, 'branches-ignore'))
        self.assertIsNone(workflow_optional_list_items(push_block, 'paths-ignore'))
        with self.subTest(event='pull_request', filter='branches'):
            self.assertIsNone(workflow_optional_list_items(pr_block, 'branches'))
        with self.subTest(event='pull_request', filter='branches-ignore'):
            self.assertIsNone(workflow_optional_list_items(pr_block, 'branches-ignore'))
        with self.subTest(event='pull_request', filter='paths-ignore'):
            self.assertIsNone(workflow_optional_list_items(pr_block, 'paths-ignore'))
        self.assertNotIn('feature/installed-state-ownership', on_section)
        push_paths = workflow_list_items(push_block, 'paths')
        pr_paths = workflow_list_items(pr_block, 'paths')
        self.assertTrue(push_paths)
        self.assertEqual(push_paths, pr_paths)

        trigger_samples = {
            'build_scripts': [
                'scripts/build-macos-app.sh',
                'scripts/check-macos-widget-intents.sh',
                'scripts/test-cask-real-brew.sh',
                'scripts/test-cask-real-brew-fixture.py',
                'scripts/cask_real_brew_fixture.py',
                'scripts/render-homebrew-cask.sh',
                'scripts/cask-local-driver.rb',
            ],
            'xcode_and_app': [
                'apps/macos/AgentDeck.xcodeproj/project.pbxproj',
                'apps/macos/AgentDeckApp/AgentDeckApp.swift',
                'apps/macos/AgentDeckWidget/AgentDeckWidget.swift',
                'apps/macos/AgentDeck.entitlements',
                'apps/macos/AgentDeckWidget.entitlements',
            ],
            'go_build_inputs': [
                'go.mod',
                'go.sum',
                'vendor/modules.txt',
                'vendor/example.com/dep/dep.go',
                'cmd/agentdeck/main.go',
                'internal/installation/receipt.go',
                'internal/platform/paths.go',
                'internal/provider/store.go',
                'internal/watch/watch.go',
            ],
            'packaging_and_workflow': [
                'packaging/homebrew/agentdeck-app.rb.tmpl',
                '.github/workflows/cask-real-brew.yml',
            ],
        }
        skip_samples = [
            'README.md',
            'docs/README.md',
            'docs/status.md',
            'docs/fixes/ci-duplicate-runs-and-macos-concurrency.md',
            '.agent-instructions/project-rules.md',
            'scripts/check-docs.py',
            'scripts/ci/classify_diff.py',
            '.github/workflows/ci.yml',
        ]
        # Jointly evaluate event, target branch (branches/branches-ignore), and changed paths.
        event_target_branches = (
            ('push', ('main',)),
            ('pull_request', ('main', 'release/v0.7.5')),
        )
        for event, branches in event_target_branches:
            for branch in branches:
                for category, paths in trigger_samples.items():
                    for sample in paths:
                        with self.subTest(event=event, branch=branch, category=category, sample=sample):
                            self.assertTrue(
                                workflow_event_triggers('cask-real-brew.yml', event, branch, [sample]),
                                f'{event} targeting {branch} must trigger on {sample}',
                            )
                for sample in skip_samples:
                    with self.subTest(event=event, branch=branch, skipped_sample=sample):
                        self.assertFalse(
                            workflow_event_triggers('cask-real-brew.yml', event, branch, [sample]),
                            f'{event} targeting {branch} must skip on {sample}',
                        )
                with self.subTest(event=event, branch=branch, mixed='trigger_with_doc'):
                    self.assertTrue(
                        workflow_event_triggers(
                            'cask-real-brew.yml', event, branch, ['docs/README.md', 'scripts/build-macos-app.sh']
                        )
                    )
                with self.subTest(event=event, branch=branch, mixed='docs_only'):
                    self.assertFalse(
                        workflow_event_triggers(
                            'cask-real-brew.yml', event, branch, ['docs/README.md', '.github/workflows/ci.yml']
                        )
                    )

        for non_main_branch in (
            'feature/installed-state-ownership',
            'fix/ci-duplicate-runs-and-macos-concurrency',
            'release/v0.7.5',
        ):
            with self.subTest(event='push', ignored_branch=non_main_branch):
                self.assertFalse(
                    workflow_event_triggers(
                        'cask-real-brew.yml', 'push', non_main_branch, ['scripts/build-macos-app.sh']
                    ),
                    f'push to {non_main_branch} must not trigger Cask real Homebrew',
                )

    def test_dynamic_runner_expression_keeps_product_macos(self):
        runner = re.search(r'^    runs-on: (.+)$', workflow_job('desktop'), re.M).group(1)
        self.assertEqual(expression(runner, 'success', 'true'), 'ubuntu-latest')
        self.assertEqual(expression(runner, 'success', 'false'), 'macos-26')
        self.assertEqual(expression(runner, 'failure', 'true'), 'macos-26')

    def test_inventory_reports_archived_findings_and_blocks_live_findings(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            old_cwd = Path.cwd()
            try:
                os.chdir(root)
                archive = 'docs/archive/reviews/old.md'
                live = 'docs/topics/demo/reviews/new.md'
                for name in [archive, live]:
                    Path(name).parent.mkdir(parents=True, exist_ok=True)
                    Path(name).write_text('## Round 1\nVerdict: PASS\n')
                errors, historical = DOCS.inspect_paths([archive], root, None, True)
                self.assertFalse(errors)
                self.assertTrue(historical)
                errors, historical = DOCS.inspect_paths([archive, live], root, None, True)
                self.assertTrue(errors)
                self.assertTrue(historical)
                errors, historical = DOCS.inspect_paths([archive], root, None, False)
                self.assertTrue(errors)  # changed archived record is not exempt
                self.assertFalse(historical)
            finally:
                os.chdir(old_cwd)


class VerifyRunnerBehaviorTest(unittest.TestCase):
    def test_pipeline_exit_depends_on_the_selected_shell(self):
        verify_shell = resolve_effective_shell('verify', 'Require applicable verification')
        for shell, expected in ((verify_shell, 0), ('bash', 1), ('sh', 0), ('bash {0}', 0),
                                ('bash -eo pipefail {0}', 1)):
            with self.subTest(shell=shell):
                settings = {'shell': shell, 'run': 'false | cat\n', 'env': {}}
                self.assertEqual(run_workflow_script(settings).returncode, expected)

    def test_omitted_shell_falls_back_to_sh_with_errexit(self):
        with tempfile.TemporaryDirectory() as temp:
            (Path(temp) / 'sh').symlink_to(shutil.which('sh'))
            with mock.patch.dict(os.environ, {'PATH': temp}):
                result = run_workflow_script({'run': 'false\nexit 0\n', 'env': {}})
                self.assertEqual(result.returncode, 1)
                with self.assertRaises(FileNotFoundError):
                    run_workflow_script({'shell': 'bash', 'run': 'exit 0\n', 'env': {}})

    def test_explicit_interpreter_and_custom_argument_template(self):
        with tempfile.TemporaryDirectory() as temp:
            (Path(temp) / 'python').symlink_to(sys.executable)
            with mock.patch.dict(os.environ, {'PATH': temp}):
                for shell in ('python', 'python -I {0}'):
                    with self.subTest(shell=shell):
                        result = run_workflow_script({'shell': shell, 'run': 'raise SystemExit(17)', 'env': {}})
                        self.assertEqual(result.returncode, 17, result.stderr)
                with self.assertRaisesRegex(ValueError, 'format containing'):
                    run_workflow_script({'shell': 'python -I', 'run': 'raise SystemExit(17)', 'env': {}})

    def test_pwsh_builtin_shell_uses_dot_source_command_and_ps1_extension(self):
        with tempfile.TemporaryDirectory() as temp:
            pwsh_stub = Path(temp) / 'pwsh'
            pwsh_stub.write_text(
                '#!/usr/bin/env python3\n'
                'import pathlib, re, sys\n'
                'assert sys.argv[1].lower() == "-command", sys.argv\n'
                'match = re.fullmatch(r"\\. \'(.+\\.ps1)\'", sys.argv[2])\n'
                'assert match is not None, sys.argv\n'
                'script = pathlib.Path(match.group(1))\n'
                'assert script.is_file(), script\n'
                'print(f"{script.suffix}:{script.read_text().strip()}")\n'
                'raise SystemExit(23)\n'
            )
            pwsh_stub.chmod(0o755)
            # Linux resolves executables case-sensitively, as the real runner
            # does; provide the upper-case name too so the case-insensitive
            # built-in argument lookup is exercised on every platform.
            upper_stub = Path(temp) / 'PWSH'
            if not upper_stub.exists():
                shutil.copy2(pwsh_stub, upper_stub)
            path_env = str(temp) + os.pathsep + os.environ.get('PATH', os.defpath)
            with mock.patch.dict(os.environ, {'PATH': path_env}):
                sample_script = Path(temp) / 'step.ps1'
                self.assertEqual(
                    _shell_argv_for_script('pwsh', sample_script),
                    [str(pwsh_stub), '-command', f". '{sample_script}'"],
                )
                for shell in ('pwsh', 'PWSH', "pwsh -command \". '{0}'\""):
                    with self.subTest(shell=shell):
                        result = run_workflow_script(
                            {'shell': shell, 'run': 'Write-Output "hello-pwsh"\n', 'env': {}},
                            runner_env={'PATH': path_env},
                        )
                        self.assertEqual(result.returncode, 23, result.stderr)
                        self.assertEqual(result.stdout.strip(), '.ps1:Write-Output "hello-pwsh"')
                with self.assertRaisesRegex(ValueError, 'format containing'):
                    run_workflow_script({'shell': 'pwsh -NoProfile', 'run': 'Write-Output "nope"\n', 'env': {}})

    def test_runner_labels_preserve_decoded_values_and_cardinality(self):
        cases = (
            ('ubuntu-latest # runner', ['ubuntu-latest']),
            ("'ubuntu-latest' # runner", ['ubuntu-latest']),
            ('"ubuntu-latest" # runner', ['ubuntu-latest']),
            ('[ubuntu-latest] # runner', ['ubuntu-latest']),
            (r'"ubuntu-\u006Catest"', ['ubuntu-latest']),
            (r'["ubuntu-\x6catest"]', ['ubuntu-latest']),
            ('\n  - ubuntu-latest', ['ubuntu-latest']),
            ('[]', []),
            ('[ubuntu-latest, ubuntu-latest]', ['ubuntu-latest', 'ubuntu-latest']),
            ('[ubuntu-latest, macos-26]', ['ubuntu-latest', 'macos-26']),
            ('"ubuntu-latest, macos-26"', ['ubuntu-latest, macos-26']),
        )
        for spelling, expected in cases:
            with self.subTest(spelling=spelling):
                self.assertEqual(runner_labels(_parse_workflow_yaml(f'runs-on: {spelling}\n')), expected)

    def test_step_uses_workspace_and_scoped_environment(self):
        step = {'name': 'gate', 'env': {'WINNER': 'step', 'RESULT': '${{ needs.build.result }}'},
                'run': 'set -eu\n'
                       'test . -ef "$GITHUB_WORKSPACE/.."\n'
                       'test ! -e "$GITHUB_WORKSPACE/.git"\n'
                       'test "$GLOBAL/$JOB/$WINNER/$RESULT" = global/job/step/failure\n'
                       'test -z "${LOCAL_ONLY+x}"\n'}
        job = {'env': {'JOB': 'job', 'WINNER': 'job'}, 'steps': [step],
               'defaults': {'run': {'working-directory': '..'}}}
        workflow = {'env': {'GLOBAL': 'global', 'WINNER': 'workflow'}, 'jobs': {'verify': job},
                    'defaults': {'run': {'shell': 'bash', 'working-directory': 'missing'}}}
        with mock.patch(__name__ + '.parsed_workflow', return_value=workflow), \
                mock.patch.dict(os.environ, {'LOCAL_ONLY': 'workstation'}):
            needs = {'build': {'result': 'failure'}}
            settings = workflow_run_settings('verify', 'gate', needs=needs)
            result = run_workflow_script(settings)
            self.assertEqual(result.returncode, 0, result.stderr)
            step.update({'working-directory': '.', 'shell': 'bash {0}',
                         'run': 'test . -ef "$GITHUB_WORKSPACE" || exit 7\nfalse\nexit 0\n'})
            result = run_workflow_script(workflow_run_settings('verify', 'gate', needs=needs))
            self.assertEqual(result.returncode, 0, result.stderr)

    def test_step_if_uses_the_current_job_status(self):
        needs = {'build': {'result': 'failure'}}
        for condition, status, expected in (
            (None, 'success', True),
            ("needs.build.result == 'failure'", 'success', True),
            ("needs.build.result == 'failure'", 'failure', False),
            ('success()', 'success', True),
            ('failure()', 'success', False),
            ('failure()', 'failure', True),
            ('cancelled()', 'cancelled', True),
            ("env.RESULT == 'failure'", 'success', True),
        ):
            with self.subTest(condition=condition, status=status):
                self.assertEqual(evaluate_workflow_if(condition, needs, step_status=status,
                                                      contexts={'env': {'RESULT': 'failure'}}), expected)


class GitHubExpressionEvaluatorTest(unittest.TestCase):
    def test_string_equality_and_inequality_are_case_insensitive(self):
        needs = {'classify': {'result': 'failure', 'outputs': {'docs_only': 'TRUE'}}}
        self.assertTrue(evaluate_github_expression("needs.classify.result == 'FAILURE'", needs=needs))
        self.assertFalse(evaluate_github_expression("needs.classify.result != 'FAILURE'", needs=needs))
        self.assertFalse(
            evaluate_github_expression("!cancelled() && needs.classify.result != 'FAILURE'", needs=needs)
        )
        self.assertTrue(evaluate_github_expression("needs.classify.outputs.docs_only == 'true'", needs=needs))
        self.assertTrue(evaluate_github_expression("contains('AgentDeck', 'deck')"))
        self.assertTrue(evaluate_github_expression("startsWith('Release/v0.7.5', 'release/')"))
        self.assertTrue(
            evaluate_github_expression(
                "startsWith(github.ref, 'refs/heads/')",
                contexts={'github': {'ref': 'refs/heads/main'}},
            )
        )

    def test_rejects_unknown_functions_contexts_and_syntax_even_under_short_circuit(self):
        invalid_expressions = [
            'true || undefinedFunction()',
            'false && undefinedFunction()',
            '!cancelled() && (true || undefinedFunction())',
            'true || missingContext.foo',
            'false && missingContext.foo',
            '!cancelled() && (true || missingContext.foo)',
            'true || env.MISSING_IN_EVAL',
            "true || contains('abc')",
            'true || always(1)',
            'true &&',
            'needs..result',
            '"double-quoted"',
            "__import__('os')",
        ]
        for expr in invalid_expressions:
            with self.subTest(expr=expr):
                with self.assertRaises(ValueError):
                    evaluate_github_expression(expr, needs={'classify': {'result': 'success'}})

    def test_failure_fails_closed_without_ancestor_conclusions_and_respects_ancestors(self):
        skipped_direct = {'race': {'result': 'skipped'}}
        success_direct = {'race': {'result': 'success'}}
        failed_direct = {'race': {'result': 'failure'}}

        with self.assertRaises(ValueError):
            evaluate_github_expression('failure()', needs=skipped_direct)
        with self.assertRaises(ValueError):
            evaluate_github_expression('failure()', needs=success_direct)
        with self.assertRaises(ValueError):
            evaluate_github_expression('true || failure()', needs=skipped_direct)

        self.assertTrue(
            evaluate_github_expression('failure()', needs=skipped_direct, ancestor_results={'go_checks': 'failure'})
        )
        self.assertFalse(
            evaluate_github_expression('failure()', needs=skipped_direct, ancestor_results={'go_checks': 'success'})
        )
        self.assertTrue(evaluate_github_expression('failure()', needs=failed_direct))
        self.assertFalse(evaluate_github_expression('failure()', needs=failed_direct, cancelled=True))
        self.assertFalse(evaluate_github_expression('failure()', needs={}))

    def test_mixed_type_coercion_and_nan_comparisons(self):
        cases = [
            ('null == false', True),
            ('null != false', False),
            ('null == 0', True),
            ("null == ''", True),
            ("null == '0'", True),
            ("null == '   '", True),
            ('null == true', False),
            ('null == 1', False),
            ("null == '1'", False),
            ('false == 0', True),
            ("false == ''", True),
            ("false == '0'", True),
            ('true == 1', True),
            ("true == '1'", True),
            ("true == ' 1.0 '", True),
            ("'1' == 1", True),
            ("'1' != 1", False),
            ("'1e2' == 100", True),
            ("'0xff' == 255", True),
            ("'-2' == -2", True),
            ("'-2.99e-2' == -2.99e-2", True),
            ("'true' == true", False),
            ("'true' != true", True),
            ("'abc' == 0", False),
            ("'abc' == false", False),
            ("'abc' == null", False),
            ("'abc' != false", True),
            ("'1_000' == 1000", False),
            ("'１２' == 12", False),
            ("'１２' != 12", True),
            ("'abc' == 'ABC'", True),
            ("'abc' != 'ABC'", False),
        ]
        for expr, expected in cases:
            with self.subTest(expr=expr):
                self.assertEqual(evaluate_github_expression(expr), expected)

        # NaN is never equal to any value (including itself) and is falsy.
        nan_ctx = {'env': {'NAN_VAL': float('nan')}}
        self.assertFalse(evaluate_github_expression('env.NAN_VAL == env.NAN_VAL', contexts=nan_ctx))
        self.assertTrue(evaluate_github_expression('env.NAN_VAL != env.NAN_VAL', contexts=nan_ctx))
        self.assertFalse(evaluate_github_expression('env.NAN_VAL == 0', contexts=nan_ctx))
        self.assertFalse(evaluate_github_expression("env.NAN_VAL == 'abc'", contexts=nan_ctx))
        self.assertTrue(evaluate_github_expression('!env.NAN_VAL', contexts=nan_ctx))

        # Regression for portable mutation: appending `&& '1' != 1` or `&& '１２' == 12` makes Actions skip portable.
        for suffix in ("&& '1' != 1", "&& '１２' == 12"):
            mutated_portable_if = (
                "!cancelled() && (needs.classify.result != 'success' || "
                f"needs.classify.outputs.docs_only != 'true') {suffix}"
            )
            with self.subTest(mutated_portable_if=mutated_portable_if):
                self.assertFalse(expression(mutated_portable_if, 'success', 'false'))
                self.assertFalse(expression(mutated_portable_if, 'failure', 'true'))

    def test_numeric_literals_negative_exponential_and_hex(self):
        parsed_values = [
            ('0', 0),
            ('711', 711),
            ('-2', -2),
            ('+42', 42),
            ('2.5', 2.5),
            ('-9.2', -9.2),
            ('0.5', 0.5),
            ('.5', 0.5),
            ('-.5', -0.5),
            ('1e2', 100.0),
            ('1E+2', 100.0),
            ('-2.99e-2', -0.0299),
            ('0xff', 255),
            ('0XFF', 255),
        ]
        for literal, expected in parsed_values:
            with self.subTest(literal=literal):
                actual = evaluate_github_expression(literal)
                self.assertEqual(actual, expected)

        for expr in (
            '-2 == -2',
            '-9.2 == -9.2',
            '1e2 == 100',
            '1E+2 == 100',
            '-2.99e-2 == -0.0299',
            '0xff == 255',
            '0XFF == 255',
            '.5 == 0.5',
            '+42 == 42',
        ):
            with self.subTest(comparison=expr):
                self.assertTrue(evaluate_github_expression(expr))

        for bad in (
            '0x',
            '0xgg',
            '-0xff',
            '+0xff',
            '-0xff == -255',
            'true || (-0xff == -255)',
            'false && (-0xff == -255)',
            'needs.0',
            'needs.-foo',
            'true || needs.0',
            'false && needs.0',
            '１２ == 12',
            '1 - 2',
            '1e',
            '1e+',
            '1.2.3',
            '123abc',
            '1e2xyz',
            '- 2',
        ):
            with self.subTest(bad_expr=bad):
                with self.assertRaises(ValueError):
                    evaluate_github_expression(bad, needs={'classify': {'result': 'success'}})

    def test_resolve_github_expression_interpolates_mixed_text_and_multiple_segments(self):
        context = {
            'github': {'workflow': 'CI', 'event_name': 'push', 'ref': 'refs/heads/main'},
            'matrix': {'shard': 1},
            'needs': {'classify': {'result': 'success', 'outputs': {'docs_only': 'false'}}},
            'env': {'A': 'alpha', 'B': 'beta', 'EMPTY': None},
        }
        with self.subTest(case='mixed_text'):
            self.assertEqual(
                resolve_github_expression('foo-${{ env.A }}-${{ env.B }}', context),
                'foo-alpha-beta',
            )
        with self.subTest(case='multi_segment_wrapped'):
            self.assertEqual(
                resolve_github_expression('${{ env.A }}-${{ env.B }}', context),
                'alpha-beta',
            )
        with self.subTest(case='concurrency_group_and_types'):
            self.assertEqual(
                resolve_github_expression(
                    parsed_workflow()['concurrency']['group'],
                    context,
                ),
                'ci-CI-push-refs/heads/main',
            )
            self.assertEqual(
                resolve_github_expression(
                    "shard=${{ matrix.shard }},docs=${{ needs.classify.outputs.docs_only == 'true' }},empty=${{ env.EMPTY }},brace=${{ 'a}}b' }}",
                    context,
                ),
                'shard=1,docs=false,empty=,brace=a}}b',
            )

        with self.subTest(case='workflow_run_settings_env'):
            workflow = {
                'env': {'WF': 'root'},
                'jobs': {
                    'verify': {
                        'runs-on': 'ubuntu-latest',
                        'env': {'JOB': '${{ env.WF }}-job'},
                        'steps': [
                            {
                                'name': 'gate',
                                'env': {'STEP': 'prefix-${{ env.JOB }}-${{ needs.build.result }}-suffix'},
                                'run': 'test "$STEP" = "prefix-root-job-failure-suffix"\n',
                            }
                        ],
                    }
                },
            }
            with mock.patch(__name__ + '.parsed_workflow', return_value=workflow):
                settings = workflow_run_settings('verify', 'gate', needs={'build': {'result': 'failure'}})
                self.assertEqual(settings['env']['JOB'], 'root-job')
                self.assertEqual(settings['env']['STEP'], 'prefix-root-job-failure-suffix')
                self.assertEqual(run_workflow_script(settings).returncode, 0)

        for bad_template in (
            'foo-${{ env.A }}-${{ unknownFunc() }}',
            '${{ env.A }}-${{ missingContext.val }}',
            'foo-${{ inputs.unavailable }}-bar',
            'foo-${{ env.A',
            'foo-${{ }}-bar',
        ):
            with self.subTest(bad_template=bad_template):
                with self.assertRaises(ValueError):
                    resolve_github_expression(bad_template, context)

    def test_empty_yaml_mappings_normalize_to_dicts_and_env_context_respects_runner_case(self):
        yaml_text = (
            'name: EmptyMappings\n'
            'on:\n'
            '  pull_request:\n'
            'env:\n'
            'defaults:\n'
            '  run:\n'
            'jobs:\n'
            '  linux_job:\n'
            '    runs-on: ubuntu-latest\n'
            '    defaults:\n'
            '    env:\n'
            '      MixedCase: LinuxValue\n'
            '      EMPTY_QUOTED: ""\n'
            '    steps:\n'
            '      - name: check empty step env\n'
            '        env:\n'
            '        run: test "$MixedCase" = LinuxValue\n'
            '      - name: check case sensitive env\n'
            '        env:\n'
            '          EXACT_HIT: ${{ env.MixedCase }}\n'
            '          WRONG_CASE_MISS: ${{ env.mixedcase }}\n'
            '          UPPER_CTX_HIT: ${{ ENV.MixedCase }}\n'
            '          NEEDS_CASEFOLD: ${{ needs.BUILD.RESULT }}\n'
            '        run: test "$EXACT_HIT/$WRONG_CASE_MISS/$UPPER_CTX_HIT/$NEEDS_CASEFOLD" = "LinuxValue//LinuxValue/success"\n'
            '  windows_job:\n'
            '    runs-on: windows-latest\n'
            '    env:\n'
            '      MixedCase: WindowsValue\n'
            '    steps:\n'
            '      - name: check case insensitive env on windows\n'
            '        env:\n'
            '          LOWER_HIT: ${{ env.mixedcase }}\n'
            '          UPPER_HIT: ${{ env.MIXEDCASE }}\n'
            '        run: exit 0\n'
        )
        with self.subTest(aspect='empty_yaml_mappings'):
            parsed = _parse_workflow_yaml(yaml_text)
            with mock.patch(__name__ + '.parsed_workflow', return_value=parsed):
                empty_step_settings = workflow_run_settings('linux_job', 'check empty step env')
                self.assertEqual(empty_step_settings['env']['MixedCase'], 'LinuxValue')
                self.assertEqual(run_workflow_script(empty_step_settings).returncode, 0)

                linux_settings = workflow_run_settings(
                    'linux_job',
                    'check case sensitive env',
                    needs={'build': {'result': 'success'}},
                )
                self.assertEqual(linux_settings['env']['EXACT_HIT'], 'LinuxValue')
                self.assertEqual(linux_settings['env']['WRONG_CASE_MISS'], '')
                self.assertEqual(linux_settings['env']['UPPER_CTX_HIT'], 'LinuxValue')
                self.assertEqual(linux_settings['env']['NEEDS_CASEFOLD'], 'success')
                self.assertEqual(run_workflow_script(linux_settings).returncode, 0)

                windows_settings = workflow_run_settings('windows_job', 'check case insensitive env on windows')
                self.assertEqual(windows_settings['env']['LOWER_HIT'], 'WindowsValue')
                self.assertEqual(windows_settings['env']['UPPER_HIT'], 'WindowsValue')

            self.assertEqual(parsed['on']['pull_request'], {})
            self.assertEqual(parsed['env'], {})
            self.assertEqual(parsed['defaults']['run'], {})
            self.assertEqual(parsed['jobs']['linux_job']['defaults'], {})
            self.assertEqual(parsed['jobs']['linux_job']['steps'][0]['env'], {})
            self.assertEqual(parsed['jobs']['linux_job']['env']['EMPTY_QUOTED'], '')

        with self.subTest(aspect='env_context_case'):
            self.assertEqual(
                evaluate_github_expression('env.FOO', contexts={'env': {'FOO': 'upper', 'foo': 'lower'}}),
                'upper',
            )
            self.assertEqual(
                evaluate_github_expression('env.foo', contexts={'env': {'FOO': 'upper', 'foo': 'lower'}}),
                'lower',
            )
            self.assertEqual(
                evaluate_github_expression('env.foo', contexts={'env': {'FOO': 'upper'}}),
                '',
            )
            self.assertEqual(
                evaluate_github_expression(
                    'env.foo',
                    contexts={'env': {'FOO': 'upper'}},
                    runner_os='Windows',
                ),
                'upper',
            )

    def test_p3_1_runner_context_still_rejected_in_expressions(self):
        with self.assertRaisesRegex(ValueError, "unsupported GitHub expression context: 'runner'"):
            evaluate_github_expression('runner.os', contexts={'runner': {'os': 'Windows'}})
        with self.assertRaisesRegex(ValueError, "unsupported GitHub expression context: 'runner'"):
            resolve_github_expression('${{ runner.os }}', {})

    def test_p3_2_same_layer_env_evaluation_uses_entry_snapshot(self):
        workflow_yaml = (
            "name: Same Layer Env\n"
            "on: [push]\n"
            "jobs:\n"
            "  j:\n"
            "    runs-on: ubuntu-latest\n"
            "    env:\n"
            "      A: x\n"
            "      B: ${{ env.A }}\n"
            "    steps:\n"
            "      - name: s\n"
            "        run: echo ok\n"
        )
        with tempfile.TemporaryDirectory() as tmpdir:
            tmp_root = Path(tmpdir)
            (tmp_root / '.github/workflows').mkdir(parents=True)
            (tmp_root / '.github/workflows/ci.yml').write_text(workflow_yaml)
            with mock.patch.object(sys.modules[__name__], 'ROOT', tmp_root):
                _parse_workflow_yaml.cache_clear()
                settings = workflow_run_settings('j', 's')
        self.assertEqual(settings['env']['A'], 'x')
        self.assertEqual(settings['env']['B'], '')

    def test_p3_3_empty_mapping_normalization_preserves_type_checks_and_sequences(self):
        parsed = _parse_workflow_yaml("items:\n  - \n  - x\nenv: \"\"\n")
        self.assertEqual(parsed['items'], ['', 'x'])
        self.assertEqual(parsed['env'], '')
        with self.assertRaisesRegex(ValueError, 'expected mapping'):
            _as_mapping(parsed['env'])

        for tagged in ('env: !!str\n', 'env: !!null\n', 'env: !custom\n', 'env: &empty\n'):
            with self.subTest(tagged=tagged.strip()):
                self.assertEqual(_parse_workflow_yaml(tagged)['env'], '')

        for empty_doc in ('', '# comment only\n'):
            with self.subTest(empty_doc=repr(empty_doc)):
                with self.assertRaises(subprocess.CalledProcessError):
                    _parse_workflow_yaml(empty_doc)
        self.assertEqual(_parse_workflow_yaml('---\n'), '')

        def workflow_with_env(env_spec):
            return (
                "name: EnvValidation\n"
                "on: [push]\n"
                "jobs:\n"
                "  j:\n"
                "    runs-on: ubuntu-latest\n"
                f"    {env_spec}\n"
                "    steps:\n"
                "      - name: s\n"
                "        run: exit 0\n"
            )

        bare_parsed = _parse_workflow_yaml(workflow_with_env('env:'))
        self.assertEqual(bare_parsed['jobs']['j']['env'], {})
        with mock.patch(__name__ + '.parsed_workflow', return_value=bare_parsed):
            self.assertEqual(workflow_run_settings('j', 's')['env'], {})

        for bad_env in (
            'env: ""',
            'env: !!str',
            'env: &empty',
            'env: {BAD: {}}',
            'env: {BAD: [x]}',
            'env: {BAD: {x: y}}',
        ):
            with self.subTest(bad_env=bad_env):
                bad_parsed = _parse_workflow_yaml(workflow_with_env(bad_env))
                with mock.patch(__name__ + '.parsed_workflow', return_value=bad_parsed):
                    with self.assertRaises(ValueError):
                        workflow_run_settings('j', 's')

    def test_p3_4_step_if_receives_runner_os_for_env_case_sensitivity(self):
        for label, expected in (
            ('windows-latest', (True, 'failure', 17)),
            ('ubuntu-latest', (True, 'success', None)),
        ):
            workflow_yaml = (
                "name: Step If Runner OS\n"
                "on: [push]\n"
                "jobs:\n"
                "  verify:\n"
                f"    runs-on: {label}\n"
                "    env:\n"
                "      FOO: present\n"
                "    steps:\n"
                "      - name: Require applicable verification\n"
                "        if: env.foo\n"
                "        run: exit 17\n"
            )
            with self.subTest(label=label):
                parsed = _parse_workflow_yaml(workflow_yaml)
                with mock.patch(__name__ + '.parsed_workflow', return_value=parsed):
                    result = CIWorkflowPathTest()._run_verify_via_workflow({})
                self.assertEqual(result, expected)

    def test_p3_5_custom_runner_label_with_windows_substring_is_not_windows(self):
        def run_settings_for(runs_on_yaml):
            workflow_yaml = (
                "name: Runner Label Detection\n"
                "on: [push]\n"
                "jobs:\n"
                "  j:\n"
                f"    runs-on: {runs_on_yaml}\n"
                "    env:\n"
                "      FOO: yes\n"
                "    steps:\n"
                "      - name: s\n"
                "        env:\n"
                "          HIT: ${{ env.foo }}\n"
                "        run: echo ok\n"
            )
            parsed = _parse_workflow_yaml(workflow_yaml)
            with mock.patch(__name__ + '.parsed_workflow', return_value=parsed):
                return workflow_run_settings('j', 's')

        for runs_on in (
            'windows',
            'windows-latest',
            'windows-2019',
            'windows-2022',
            'windows-2025',
            'windows-11-arm',
            'windows-2025-vs2026',
            'windows-11-vs2026-arm',
            '[self-hosted, Windows, ARM64]',
            '{group: pool, labels: windows-latest}',
            '{group: pool, labels: [self-hosted, Windows, ARM64]}',
        ):
            with self.subTest(windows_runs_on=runs_on):
                settings = run_settings_for(runs_on)
                self.assertEqual(settings['runner_os'], 'Windows')
                self.assertEqual(settings['env']['HIT'], 'yes')

        for runs_on in (
            '[self-hosted, ubuntu-latest, no-windows-tools]',
            'windows-tools',
            'windows-foo',
            'windows-2025-tools',
            'windows-11-tools-arm',
            '" windows-latest "',
            '{group: windows-latest}',
        ):
            with self.subTest(non_windows_runs_on=runs_on):
                settings = run_settings_for(runs_on)
                self.assertEqual(settings['runner_os'], 'Linux')
                self.assertEqual(settings['env']['HIT'], '')

        for bad_runs_on in (
            '{group: pool, labels: {}}',
            '\n      group: pool\n      labels:',
            '{group: pool, labels: [windows-latest, {}]}',
            '[windows-latest, {}]',
            '[windows-latest, [arm64]]',
            '[windows-latest, "${{ runner.os }}"]',
            '{group: pool, labels: [windows-latest, "${{ runner.os }}"]}',
            '{group: ""}',
            '{group: "  "}',
            '{group: "${{ runner.os }}"}',
            '{group: "${{ \'\' }}"}',
            '{group: pool, lables: windows-latest}',
            '{}',
        ):
            with self.subTest(bad_runs_on=bad_runs_on):
                with self.assertRaises(ValueError):
                    run_settings_for(bad_runs_on)

        for bad_runs_on in (
            '[windows-latest, "${{ runner.os }}"]',
            '[windows-latest, [arm64]]',
            '{group: "", labels: {}}',
            '{group: "${{ runner.os }}"}',
        ):
            job = _parse_workflow_yaml(f'runs-on: {bad_runs_on}\n')
            for explicit_ctx in ({'runner': {'os': 'Windows'}}, {'runner_os': 'Windows'}):
                with self.subTest(bad_runs_on=bad_runs_on, explicit_ctx=explicit_ctx):
                    with self.assertRaises(ValueError):
                        _job_runner_os(job, contexts=explicit_ctx)

        valid_job = _parse_workflow_yaml('runs-on: ubuntu-latest\n')
        for explicit_ctx in ({'runner': {'os': 'Windows'}}, {'runner_os': 'Windows'}):
            with self.subTest(valid_override=explicit_ctx):
                self.assertEqual(_job_runner_os(valid_job, contexts=explicit_ctx), 'Windows')

        for explicit_os, expected in (
            ('Windows', 'hit'),
            ('windows', 'hit'),
            (' Windows ', ''),
            ('Linux', ''),
            ('no-windows-tools', ''),
        ):
            with self.subTest(explicit_os=explicit_os):
                self.assertEqual(
                    evaluate_github_expression(
                        'env.foo',
                        contexts={'env': {'FOO': 'hit'}},
                        runner_os=explicit_os,
                    ),
                    expected,
                )


if __name__ == '__main__':
    unittest.main()
