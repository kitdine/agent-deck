from __future__ import annotations

import importlib.util
import json
import os
import re
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
    return re.split(r'\n  [a-z]+:\n', text.split(f'\n  {name}:\n', 1)[1], maxsplit=1)[0]


def workflow_step(job, name):
    return workflow_job(job).split(f'      - name: {name}\n', 1)[1].split('      - name:', 1)[0]


def workflow_run(job, name):
    step = workflow_step(job, name)
    if '        run: |\n' in step:
        body = step.split('        run: |\n', 1)[1]
        return '\n'.join(line[10:] for line in body.splitlines() if line.strip())
    return step.split('        run: ', 1)[1].splitlines()[0]


def expression(value, result='success', docs_only='false', cancelled=False):
    value = value.removeprefix('${{').removesuffix('}}').strip()
    value = value.replace('needs.classify.result', repr(result))
    value = value.replace('needs.classify.outputs.docs_only', repr(docs_only))
    value = value.replace('!cancelled()', repr(not cancelled))
    return eval(value.replace('&&', ' and ').replace('||', ' or '), {'__builtins__': {}})


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

    def test_ci_first_deployment_establishes_doc_range_without_base_classifier(self):
        head = self.state({'docs/README.md': '# Doc\n', 'cmd/main.go': 'changed code\n'}, self.base)
        output = self.root / 'output'
        (self.root / 'event.json').write_text(json.dumps({'before': self.base, 'after': head}))
        env = dict(os.environ, BASE_SHA=self.base, HEAD_SHA=head, EVENT_NAME='push',
                   GITHUB_OUTPUT=str(output), GITHUB_EVENT_PATH=str(self.root / 'event.json'))
        result = subprocess.run(['bash', '-e', '-c', workflow_run('classify', 'Classify using the trusted base script')],
                                env=env, capture_output=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        values = dict(line.split('=', 1) for line in output.read_text().splitlines())
        self.assertEqual(values['docs_only'], 'false')
        self.assertEqual(values['doc_base'], self.base)
        self.assertEqual(values['doc_head'], head)

    def test_ci_new_branch_without_trusted_default_reports_unknown(self):
        output = self.root / 'output'
        (self.root / 'event.json').write_text(json.dumps({'before': '0' * 40, 'after': self.base}))
        env = dict(os.environ, BASE_SHA='0' * 40, HEAD_SHA=self.base, EVENT_NAME='push',
                   GITHUB_OUTPUT=str(output), GITHUB_EVENT_PATH=str(self.root / 'event.json'))
        result = subprocess.run(['bash', '-e', '-c', workflow_run('classify', 'Classify using the trusted base script')],
                                env=env, capture_output=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn('docs_only=false', output.read_text())
        self.assertIn('doc_scope_known=false', output.read_text())
        self.assertIn('范围未知', result.stdout.decode())

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
        result = subprocess.run(['bash', 'scripts/check-topic-docs.sh', 'v0-6-5-contract'],
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
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            (root / 'scripts').mkdir()
            (root / 'scripts/check-docs.py').write_text('import sys\nprint(repr(sys.argv[1:]))\n')
            for base, head, expected in [('a', 'b', "['--base', 'a', '--head', 'b']"), ('', '', None)]:
                result = subprocess.run(['bash', '-e', '-c', workflow_run('documentation', 'Document L0, links, document set and review records')],
                                        cwd=root, env=dict(os.environ, DIFF_BASE=base, DIFF_HEAD=head, DOC_SCOPE_KNOWN='true' if base else 'false', DOC_REASON='范围未知'), capture_output=True, text=True)
                if expected:
                    self.assertEqual(result.returncode, 0, result.stderr)
                    self.assertIn(expected, result.stdout)
                else:
                    self.assertNotEqual(result.returncode, 0)
                    self.assertIn('范围未知', result.stderr)
                    self.assertNotIn('--all', result.stdout)

    def test_both_required_jobs_fail_when_documentation_fails(self):
        for job in ['verify', 'desktop']:
            for status in ['success', 'failure', 'cancelled', 'skipped']:
                result = subprocess.run(['bash', '-e', '-c', workflow_run(job, 'Require documentation checks')],
                                        env=dict(os.environ, DOCUMENTATION_RESULT=status))
                self.assertEqual(result.returncode == 0, status == 'success')

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


if __name__ == '__main__':
    unittest.main()
