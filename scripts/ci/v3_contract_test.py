from __future__ import annotations

import json
import os
import shutil
import subprocess
import tempfile
import unittest
from pathlib import Path
from unittest import mock

from docs_tools_test import DOCS, GitFixture, REVIEW, ROOT, SCOPE, workflow_run


class ScopeV3Test(GitFixture):
    def test_real_new_branch_uses_trusted_default_without_lightening_product(self):
        subprocess.run(['git', 'update-ref', 'refs/remotes/origin/main', self.base], check=True)
        head = self.state({'docs/specs/cli-design.md': '# Original\n', 'cmd/main.go': 'package main\n',
                           'docs/new.md': '# New document\n'}, self.base)
        event = {'before': '0' * 40, 'after': head, 'repository': {'default_branch': 'main'},
                 'pull_request': {'base': {'ref': 'evil', 'sha': 'f' * 40}}}
        result = SCOPE.resolve(event, 'push')
        self.assertEqual(result['doc_scope_known'], 'true')
        self.assertEqual(result['doc_baseline_ref'], 'refs/remotes/origin/main')
        self.assertEqual(result['doc_baseline_sha'], self.base)
        self.assertEqual(result['docs_only'], 'false')
        self.assertEqual(DOCS.changed_paths(result['doc_base'], result['doc_head']), ['docs/new.md'])
        (self.root / 'event.json').write_text(json.dumps(event))
        output = self.root / 'output'
        execution = subprocess.run(['bash', '-e', '-c', workflow_run('classify', 'Classify using the trusted base script')],
                                   env=dict(os.environ, BASE_SHA='0' * 40, HEAD_SHA=head, EVENT_NAME='push',
                                            GITHUB_EVENT_PATH=str(self.root / 'event.json'), GITHUB_OUTPUT=str(output)),
                                   capture_output=True)
        self.assertEqual(execution.returncode, 0, execution.stderr)
        values = dict(line.split('=', 1) for line in output.read_text().splitlines())
        self.assertEqual(values['doc_scope_known'], 'true')
        self.assertEqual(values['docs_only'], 'false')
        self.assertEqual(values['doc_base'], self.base)

    def test_missing_or_invalid_default_ref_is_unknown(self):
        for branch in ['missing', 'main\ninjected=true', '']:
            result = SCOPE.resolve({'before': '0' * 40, 'after': self.base,
                                    'repository': {'default_branch': branch}}, 'push')
            self.assertEqual(result['doc_scope_known'], 'false')
            self.assertIn('范围未知', result['doc_reason'])

    def test_multiple_merge_bases_are_unknown(self):
        original = SCOPE.git
        def ambiguous(*args):
            return self.base + '\n' + 'a' * 40 if args[:2] == ('merge-base', '--all') else original(*args)
        event = {'pull_request': {'base': {'sha': self.base}, 'head': {'sha': self.base}}}
        with mock.patch.object(SCOPE, 'git', side_effect=ambiguous):
            self.assertEqual(SCOPE.resolve(event, 'pull_request')['doc_scope_known'], 'false')

    def test_shallow_history_and_unrelated_history_are_unknown(self):
        original = SCOPE.git
        with mock.patch.object(SCOPE, 'git', side_effect=lambda *args: 'true' if args == ('rev-parse', '--is-shallow-repository') else original(*args)):
            self.assertEqual(SCOPE.resolve({'before': self.base, 'after': self.base}, 'push')['doc_scope_known'], 'false')
        unrelated = self.state({'docs/foreign.md': '# Foreign\n'})
        event = {'pull_request': {'base': {'sha': self.base}, 'head': {'sha': unrelated}}}
        self.assertEqual(SCOPE.resolve(event, 'pull_request')['doc_scope_known'], 'false')


class ParserV3Test(unittest.TestCase):
    def test_legacy_rereview_emoji_and_reopen_are_distinct_rounds(self):
        body = ('## Review — Round 1\n✅ 结论: FAIL\nVerdict: REOPEN\nCompletion gate: `FAILED`\n'
                '## 📋 Re-review — Round 2\n✅ **结论**：**PASS**\n- Completion gate: `VERIFIED`\n')
        self.assertEqual(len(REVIEW.review_sections(body)), 2)
        self.assertEqual(REVIEW.section_state(REVIEW.review_sections(body)[0]), ('FAIL', 'FAILED'))
        self.assertEqual(REVIEW.section_state(REVIEW.review_sections(body)[1]), ('PASS', 'VERIFIED'))
        self.assertEqual(REVIEW.validation_errors(body), [])

    def test_inline_quoted_declarations_and_code_blocks_are_not_live(self):
        body = ('## Round 1\nVerdict: PASS\nCompletion gate: `VERIFIED`\n'
                '`Verdict: FAIL` is another record.\n    Verdict: FAIL\n\tCompletion gate: FAILED\n'
                '```md\n## Round 99\nVerdict: FAIL\n```\n')
        self.assertEqual(REVIEW.section_state(REVIEW.review_sections(body)[-1]), ('PASS', 'VERIFIED'))
        self.assertEqual(REVIEW.validation_errors(body), [])

    def test_multiline_inline_quote_does_not_add_round_or_declarations(self):
        body = ('## Round 1\nVerdict: PASS\nCompletion gate: `VERIFIED`\n'
                'An inline quote ``example\n## Round 99\nVerdict: FAIL\nCompletion gate: FAILED\nend``\n')
        self.assertEqual(len(REVIEW.review_sections(body)), 1)
        self.assertEqual(REVIEW.section_state(REVIEW.review_sections(body)[0]), ('PASS', 'VERIFIED'))
        self.assertEqual(REVIEW.validation_errors(body), [])

    def test_historical_missing_gate_survives_unrelated_link_edit_without_certification(self):
        old = '# Index\n[link](old.md)\n## Round 1\nVerdict: PASS\n[source](old.md)\n'
        changed = old.replace('(old.md)', '(new.md)')
        self.assertEqual(REVIEW.validation_errors(changed, old), [])
        self.assertTrue(REVIEW.validation_notes(changed, old))
        self.assertEqual(REVIEW.section_state(REVIEW.review_sections(changed)[-1]), ('PASS', None))
        self.assertTrue(REVIEW.validation_errors(changed + 'New review facts.\n', old))

    def test_real_legacy_fix_records_get_latest_rereview_without_rewriting(self):
        for slug in ['claude-backup-api-key-redaction', 'claude-no-route-quality', 'claude-startup-route-live',
                     'hook-transcript-admission-edges', 'ownerless-findings-same-line', 'preassembly-quota-migration',
                     'staple-offline-first-launch']:
            path = ROOT / 'docs/fixes' / (slug + '.md')
            with self.subTest(slug=slug):
                self.assertEqual(REVIEW.latest_review_state(path), ('PASS', 'VERIFIED'))
                self.assertEqual(REVIEW.validation_errors(path.read_text(), path.read_text()), [])

    def test_real_double_gate_still_fails_when_round_changed(self):
        fixture = Path(__file__).parent / 'fixtures/pr40-round2-double-gate.txt'
        old = fixture.read_text()
        self.assertTrue(REVIEW.validation_errors(old))
        self.assertTrue(REVIEW.validation_errors(old + '\nNew review facts.', old))


class LinksV3Test(unittest.TestCase):
    def test_supported_links_ignore_all_code_forms_and_accept_root_and_line_ranges(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            target = root / 'target.md'
            target.write_text('# `Code` Heading\ntext\nlast\n')
            source = root / 'source.md'
            source.write_text('[root](/target.md#code-heading)\n[line](target.md#L1)\n[range](target.md#L1-L3)\n'
                              '`[example](missing.md)`\n``[example](missing.md) with ` tick``\n'
                              '    [indent](missing.md)\n\t[indent](missing.md)\n'
                              '~~~md\n[fence](missing.md)\n~~~\n[ref]: <target.md> "Title"\n')
            self.assertEqual(DOCS.link_errors(source, root), [])

    def test_invalid_line_ranges_and_missing_heading_are_different_errors(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            (root / 'target.md').write_text('# Target\ntext\n')
            source = root / 'source.md'
            source.write_text('[zero](target.md#L0)\n[reversed](target.md#L2-L1)\n'
                              '[beyond](target.md#L1-L3)\n[heading](target.md#absent)\n')
            errors = DOCS.link_errors(source, root)
            self.assertEqual(sum('invalid line range' in e for e in errors), 3)
            self.assertEqual(sum('missing Markdown anchor' in e for e in errors), 1)

    def test_complex_nested_and_escaped_links_report_unsupported(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            source = root / 'source.md'
            source.write_text('[label [nested]](target.md)\n[label](foo(bar).md)\n[label](foo\\ bar.md)\n')
            errors = DOCS.link_errors(source, root)
            self.assertEqual(len(errors), 3)
            self.assertTrue(all('unsupported link syntax' in e for e in errors), errors)
            self.assertFalse(any('missing local link' in e for e in errors))

    def test_inline_html_id_and_multiline_code_do_not_create_anchors(self):
        text = '# Real\n`<a id="fake">`\n`` code\n# Fake heading\n``\n<a id="actual"></a>\n'
        self.assertEqual(DOCS.anchors(text), {'real', 'actual'})

    def test_nested_indented_link_is_unsupported_not_silently_dropped(self):
        targets, unsupported = DOCS.link_targets('- parent item\n    [child](missing.md)\n')
        self.assertFalse(targets)
        self.assertTrue(unsupported)


class DocumentChangesV3Test(GitFixture):
    def materialize(self, head):
        for path in self.root.iterdir():
            if path.name != '.git':
                shutil.rmtree(path) if path.is_dir() and not path.is_symlink() else path.unlink()
        subprocess.run(['git', 'read-tree', head], check=True)
        subprocess.run(['git', 'checkout-index', '--all', '--force'], check=True)
        script = self.root / 'scripts/check-topic-docs.sh'
        script.parent.mkdir(exist_ok=True)
        script.write_text((ROOT / 'scripts/check-topic-docs.sh').read_text())

    def tasks(self, required=True):
        return ('# Topic\n\n## Documents\n\n| Document | Draft | Review |\n| --- | --- | --- |\n'
                '| tasks.md | [x] | [ ] |\n| requirements.md | ' + ('[x] | [ ]' if required else 'n/a | n/a') +
                ' |\n\n## Tasks\n\nNo product implementation.\n')

    def test_name_status_parser_handles_every_requested_kind(self):
        changes = DOCS.parse_changes(b'A\0docs/a.md\0M\0docs/m.md\0R100\0docs/topics/old/a.md\0docs/topics/new/a.md\0'
                                     b'C100\0docs/c.md\0docs/copy.md\0D\0docs/deleted.md\0T\0docs/type.md\0')
        self.assertEqual([c[0] for c in changes], ['A', 'M', 'R', 'C', 'D', 'T'])
        self.assertEqual(DOCS.affected_topics(changes), {'old', 'new'})
        self.assertEqual(changes[4], ('D', 'docs/deleted.md', None))

    def test_real_cross_topic_rename_checks_both_sets_and_incoming_links(self):
        requirement = '# Requirement\n' + 'Contract text.\n' * 12
        files = {'docs/topics/old/tasks.md': self.tasks(), 'docs/topics/old/requirements.md': requirement,
                 'docs/topics/new/tasks.md': self.tasks(False), 'docs/README.md': '[old](topics/old/requirements.md)\n'}
        base = self.state(files)
        files.pop('docs/topics/old/requirements.md')
        files.update({'docs/topics/old/tasks.md': self.tasks(False), 'docs/topics/new/tasks.md': self.tasks(),
                      'docs/topics/new/requirements.md': requirement})
        head = self.state(files, base)
        self.materialize(head)
        changes = DOCS.changes_for(base, head)
        self.assertIn(('R', 'docs/topics/old/requirements.md', 'docs/topics/new/requirements.md'), changes)
        self.assertEqual(DOCS.affected_topics(changes), {'old', 'new'})
        errors = DOCS.scoped_errors(changes, self.root, base)
        self.assertTrue(any('missing local link' in e and 'README' in e for e in errors), errors)
        files['docs/README.md'] = '[new](topics/new/requirements.md)\n'
        fixed = self.state(files, base)
        self.materialize(fixed)
        self.assertEqual(DOCS.scoped_errors(DOCS.changes_for(base, fixed), self.root, base), [])

    def test_real_delete_checks_tasks_and_incoming_without_reading_deleted_file(self):
        files = {'docs/topics/demo/tasks.md': self.tasks(), 'docs/topics/demo/requirements.md': '# Requirement\n' + 'Text\n' * 12,
                 'docs/README.md': '[requirement](topics/demo/requirements.md)\n'}
        base = self.state(files)
        files.pop('docs/topics/demo/requirements.md')
        head = self.state(files, base)
        self.materialize(head)
        errors = DOCS.scoped_errors(DOCS.changes_for(base, head), self.root, base)
        self.assertTrue(any('missing local link' in e for e in errors))
        self.assertTrue(any('document-set check failed' in e for e in errors))
        self.assertFalse(any('cannot read record' in e for e in errors))
        files.update({'docs/topics/demo/tasks.md': self.tasks(False), 'docs/README.md': '# Index\n'})
        fixed = self.state(files, base)
        self.materialize(fixed)
        self.assertEqual(DOCS.scoped_errors(DOCS.changes_for(base, fixed), self.root, base), [])

    def test_whole_topic_retirement_validates_archived_tasks_and_links(self):
        tasks = self.tasks(False)
        base = self.state({'docs/topics/demo/tasks.md': tasks, 'docs/README.md': '[topic](topics/demo/tasks.md)\n'})
        head = self.state({'docs/archive/topics/demo/tasks.md': tasks,
                           'docs/README.md': '[retired](archive/topics/demo/tasks.md)\n'}, base)
        self.materialize(head)
        self.assertFalse((self.root / 'docs/topics/demo').exists())
        self.assertEqual(DOCS.scoped_errors(DOCS.changes_for(base, head), self.root, base), [])

    def test_whole_topic_delete_does_not_pass_missing_selector_but_checks_incoming(self):
        base = self.state({'docs/topics/demo/tasks.md': self.tasks(False),
                           'docs/README.md': '[topic](topics/demo/tasks.md)\n'})
        head = self.state({'docs/README.md': '[topic](topics/demo/tasks.md)\n'}, base)
        self.materialize(head)
        errors = DOCS.scoped_errors(DOCS.changes_for(base, head), self.root, base)
        self.assertTrue(any('missing local link' in e for e in errors))
        self.assertFalse(any('(2)' in e for e in errors))

    def test_heading_change_checks_unchanged_incoming_source(self):
        base = self.state({'docs/target.md': '# Original\n', 'docs/README.md': '[target](target.md#original)\n'})
        head = self.state({'docs/target.md': '# Changed\n', 'docs/README.md': '[target](target.md#original)\n'}, base)
        self.materialize(head)
        errors = DOCS.scoped_errors(DOCS.changes_for(base, head), self.root, base)
        self.assertTrue(any('missing Markdown anchor' in e for e in errors))

    def test_real_copy_is_read_and_preserves_source_in_attribution(self):
        base = self.state({'docs/source.md': '# Unique copy\n'})
        head = self.state({'docs/source.md': '# Unique copy\n', 'docs/copy.md': '# Unique copy\n'}, base)
        self.materialize(head)
        changes = DOCS.changes_for(base, head)
        self.assertIn(('C', 'docs/source.md', 'docs/copy.md'), changes)
        self.assertEqual(DOCS.scoped_errors(changes, self.root, base), [])

    def test_changed_markdown_type_with_broken_symlink_is_not_silently_skipped(self):
        base = self.state({'docs/type.md': '# Original\n'})
        head = self.state({'docs/type.md': 'absent.md'}, base, {'docs/type.md': '120000'})
        self.materialize(head)
        changes = DOCS.changes_for(base, head)
        self.assertIn(('T', 'docs/type.md', 'docs/type.md'), changes)
        errors = DOCS.scoped_errors(changes, self.root, base)
        self.assertTrue(any('changed document is missing' in e for e in errors))

    def test_renamed_historical_record_does_not_invent_gate(self):
        body = '## Round 1\nVerdict: PASS\n'
        base = self.state({'docs/fixes/old.md': body})
        head = self.state({'docs/fixes/new.md': body}, base)
        self.materialize(head)
        self.assertEqual(DOCS.scoped_errors(DOCS.changes_for(base, head), self.root, base), [])


if __name__ == '__main__':
    unittest.main()
