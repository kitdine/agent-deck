import copy
import hashlib
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest

import scan_performance_switches as p03
import scan_performance_serving_matrix as matrix

REPO = Path(__file__).resolve().parents[1]
spec = importlib.util.spec_from_file_location('serving', REPO / 'scripts/scan-performance-serving.py')
serving = importlib.util.module_from_spec(spec)
spec.loader.exec_module(serving)


class SwitchTests(unittest.TestCase):
    def sample(self):
        source = {'client': 'all', 'period': 'today', 'panel': 'quota'}
        target = {**source, 'period': '7d'}
        oracle = dict(target=target, state='available', labels=['Average', 'Peak'], values=['123', '456'])
        oracle_hash = hashlib.sha256(json.dumps(oracle, sort_keys=True, ensure_ascii=False, separators=(',', ':')).encode()).hexdigest()
        return dict(kind='period', source=source, target=target, action_source=source.copy(), observed=target.copy(),
            action_target='7d', publication_before='p1', publication_after='p1',
            publication_window={'timezone': 'America/New_York', 'start': '2026-10-19T04:00:00Z', 'end': '2026-10-20T04:00:00Z'},
            window={'timezone': 'America/New_York', 'start': '2026-10-13T04:00:00Z', 'end': '2026-10-20T04:00:00Z', 'period': '7d'},
            period_oracle_sha256='e'*64, observed_period_sha256='e'*64,
            control_frame=[25, 920, 120, 26], selected_control_frame=[25, 920, 120, 26],
            panel_oracle_target=target.copy(), panel_oracle_prepared_ns=90,
            panel_oracle_fields=oracle, panel_oracle_sha256=oracle_hash, panel_expected_state='available', panel_expected_numeric_count=2,
            panel_body_sha256='1'*64, previous_panel_body_sha256='2'*64,
            panel_numeric_matches=[True, True], body_geometry=dict(semantic_region='quota', excludes_hero=True, excludes_notices=True, excludes_rhythm=True),
            hero_field_matches=[True, True, True],
            issued_ns=100, action_ns=110, frame_ns=150, elapsed_ns=40,
            helper_launches_before=2, helper_launches_after=2, frame_sha256='a'*64, previous_frame_sha256='b'*64,
            visible=True, selected_control=True, scope_matches=True, numeric_frame=True, panel_frame=True,
            cohort_sha256='c'*64)

    def test_correct_frame_and_action(self):
        self.assertIsNone(p03.validate_sample(self.sample()))

    def test_stale_wrong_scope_control_panel_and_publication_rejected(self):
        mutations = [({'frame_sha256': 'b'*64}, 'stale_frame'),
            ({'observed': {'client': 'all', 'period': 'today', 'panel': 'quota'}}, 'wrong_or_premutated_selection'),
            ({'selected_control': False}, 'wrong_native_frame'), ({'numeric_frame': False}, 'wrong_native_frame'),
            ({'panel_frame': False}, 'wrong_native_frame'), ({'publication_after': 'p2'}, 'publication_changed')]
        for fields, reason in mutations:
            with self.subTest(fields=fields):
                sample = self.sample(); sample.update(fields)
                self.assertEqual(p03.validate_sample(sample), reason)

    def test_wrong_window_and_unbound_bitmap_rejected(self):
        sample = self.sample(); sample['window']['period'] = 'today'
        self.assertEqual(p03.validate_sample(sample), 'wrong_window')
        sample = self.sample(); sample['frame_sha256'] = 'unverified'
        self.assertEqual(p03.validate_sample(sample), 'missing_bitmap_identity')

    def test_today_header_cannot_masquerade_as_selected_seven_day_window(self):
        sample = self.sample(); sample['window']['start'] = sample['publication_window']['start']
        self.assertEqual(p03.validate_sample(sample), 'wrong_window')
        sample = self.sample(); sample['publication_window']['end'] = '2026-10-21T04:00:00Z'
        self.assertEqual(p03.validate_sample(sample), 'wrong_window')

    def test_calendar_window_preserves_dst_and_rejects_unknown_period(self):
        publication = dict(timezone='America/New_York', start='2026-11-01T04:00:00Z', end='2026-11-02T05:00:00Z')
        window = p03.selection_window(publication, '7d')
        self.assertEqual(window['start'], '2026-10-26T04:00:00Z')
        self.assertEqual(window['end'], '2026-11-02T05:00:00Z')
        with self.assertRaises(ValueError):
            p03.selection_window(publication, 'unsupported')

    def test_wrong_period_oracle_and_missing_rendered_geometry_rejected(self):
        sample = self.sample(); sample['observed_period_sha256'] = 'f'*64
        self.assertEqual(p03.validate_sample(sample), 'wrong_period_totals')
        sample = self.sample(); sample.pop('period_oracle_sha256'); sample.pop('observed_period_sha256')
        self.assertEqual(p03.validate_sample(sample), 'missing_period_oracle')
        for rect in ([], [0, 0, 0, 2], [0, 0, float('nan'), 2]):
            sample = self.sample(); sample['control_frame'] = rect
            self.assertEqual(p03.validate_sample(sample), 'missing_control_geometry')

    def test_new_hero_and_controls_cannot_hide_stale_or_missing_panel_body(self):
        sample = self.sample(); sample['panel_frame'] = False
        self.assertTrue(sample['numeric_frame'] and sample['selected_control'])
        self.assertNotEqual(sample['frame_sha256'], sample['previous_frame_sha256'])
        self.assertEqual(p03.validate_sample(sample), 'wrong_native_frame')

    def test_panel_oracle_must_precede_action_and_bind_requested_scope(self):
        for fields in ({'panel_oracle_prepared_ns': 111}, {'panel_oracle_target': self.sample()['source']},
                       {'panel_oracle_prepared_ns': None}):
            sample = self.sample(); sample.update(fields)
            self.assertEqual(p03.validate_sample(sample), 'unbound_panel_oracle')

    def test_title_only_available_panel_has_no_numeric_semantics(self):
        sample = self.sample(); sample['panel_expected_numeric_count'] = 0
        self.assertEqual(p03.validate_sample(sample), 'missing_panel_semantics')
        sample = self.sample(); sample['panel_expected_state'] = 'unavailable'; sample['panel_expected_numeric_count'] = 0
        sample['panel_oracle_fields'].update(state='unavailable', labels=['Unavailable'], values=[])
        sample['panel_numeric_matches'] = []
        sample['panel_oracle_sha256'] = hashlib.sha256(json.dumps(sample['panel_oracle_fields'], sort_keys=True, ensure_ascii=False, separators=(',', ':')).encode()).hexdigest()
        self.assertIsNone(p03.validate_sample(sample))
        sample.pop('panel_body_sha256')
        self.assertEqual(p03.validate_sample(sample), 'missing_panel_evidence')

    def test_frozen_panel_fields_are_bound_to_digest(self):
        sample = self.sample(); sample['panel_oracle_fields']['values'][0] = '999'
        self.assertEqual(p03.validate_sample(sample), 'panel_oracle_digest_mismatch')

    def test_panel_numbers_and_crop_must_pass_independently_of_panel_flag(self):
        sample = self.sample(); sample['panel_numeric_matches'][0] = False
        self.assertTrue(sample['panel_frame'])
        self.assertEqual(p03.validate_sample(sample), 'wrong_panel_numbers')
        sample = self.sample(); sample['body_geometry']['excludes_rhythm'] = False
        self.assertEqual(p03.validate_sample(sample), 'panel_crop_contaminated')

    def test_hero_tokens_must_be_visible_in_hero_crop(self):
        sample = self.sample(); sample['hero_field_matches'] = [True, False, True]
        self.assertTrue(sample['numeric_frame'])
        self.assertEqual(p03.validate_sample(sample), 'wrong_hero_crop')

    def test_attribution_quality_cannot_use_neighboring_provider_numbers(self):
        sample = self.sample()
        for key in ('source', 'target', 'action_source', 'observed', 'panel_oracle_target'):
            sample[key]['panel'] = 'attribution'
        sample['panel_oracle_fields']['target']['panel'] = 'attribution'
        sample['panel_oracle_sha256'] = hashlib.sha256(json.dumps(sample['panel_oracle_fields'], sort_keys=True, ensure_ascii=False, separators=(',', ':')).encode()).hexdigest()
        sample['body_geometry'].update(semantic_region='attribution', excludes_provider_quality=True)
        self.assertEqual(p03.validate_sample(sample), 'panel_crop_contaminated')
        sample['body_geometry'].update(semantic_region='attribution.quality', excludes_provider_quality=False)
        self.assertEqual(p03.validate_sample(sample), 'panel_crop_contaminated')
        sample['body_geometry']['excludes_provider_quality'] = True
        self.assertIsNone(p03.validate_sample(sample))

    def test_premutated_selection_and_premature_or_wrong_timer_rejected(self):
        sample = self.sample(); sample['action_source'] = sample['target'].copy()
        self.assertEqual(p03.validate_sample(sample), 'wrong_or_premutated_selection')
        for fields in ({'action_ns': 99}, {'frame_ns': 109}, {'elapsed_ns': 50}, {'action_ns': 0}):
            sample = self.sample(); sample.update(fields)
            self.assertIsNotNone(p03.validate_sample(sample))

    def test_delayed_real_action_cost_is_not_subtracted(self):
        sample = self.sample(); sample['frame_ns'] += 150_000_000; sample['elapsed_ns'] += 150_000_000
        self.assertIsNone(p03.validate_sample(sample))
        summary = p03.summarize([sample]*100)
        self.assertFalse(next(iter(summary['groups'].values()))['budget_pass'])

    def test_actual_helper_launch_and_unknown_action_rejected(self):
        sample = self.sample(); sample['helper_launches_after'] += 1
        self.assertEqual(p03.validate_sample(sample), 'helper_launched')
        sample = self.sample(); sample['action_target'] = 'unknown'
        self.assertEqual(p03.validate_sample(sample), 'wrong_action')
        sample = self.sample(); sample['target'] = sample['source'].copy()
        self.assertEqual(p03.validate_sample(sample), 'not_one_actual_transition')

    def test_counts_split_by_transition_history_publication_and_exact_cohort(self):
        a = self.sample(); b = copy.deepcopy(a); b['cohort_sha256'] = 'd'*64
        result = p03.summarize([a]*50+[b]*50)
        self.assertEqual(len(result['groups']), 2)
        self.assertTrue(all(group['p95_ns'] is None for group in result['groups'].values()))
        invalid = copy.deepcopy(a); invalid['numeric_frame'] = False
        result = p03.summarize([a]*99+[invalid])
        self.assertEqual(next(iter(result['groups'].values()))['count'], 99)
        self.assertIsNone(next(iter(result['groups'].values()))['p95_ns'])

    def test_export_does_not_change_existing_p01_p02_patches(self):
        with tempfile.TemporaryDirectory() as root:
            root = Path(root)
            paths = ['apps/macos/AgentDeckApp/MenuBarItemController.swift', 'apps/macos/AgentDeckApp/AgentDeckApp.swift',
                'apps/macos/AgentDeckApp/MenuBarSurfaceView.swift', 'apps/macos/AgentDeckShared/EmbeddedHelperRunner.swift', 'cmd/agentdeck/main.go',
                'apps/macos/AgentDeckApp/MenuBarPanelViews.swift']
            for name in paths:
                path = root/name; path.parent.mkdir(parents=True, exist_ok=True); path.write_bytes((REPO/name).read_bytes())
            serving.patch_probe(root, 0, False, 'manual', True, 120)
            matrix.patch_matrix_probe(root, serving.sha)
            controller = root/paths[0]; prior = controller.read_text()
            p03.patch_switch_probe(root, serving.sha)
            updated = controller.read_text()
            # Existing clocks/frame handlers remain literally unchanged.
            for start, end in [('func captureNativeBaseline(', 'private func e1RebuildActionJSON('),
                               ('func captureE1StartupFrame(', 'private func p03Selection(')]:
                fragment = prior[prior.index(start):prior.index(end)] if end in prior else prior[prior.index(start):prior.index('\tprivate enum BaselineFailure:')]
                self.assertIn(fragment.rstrip(), updated)
            self.assertIn('element.accessibilityPerformAction(.press)', updated)
            self.assertIn('try p03Press(control)', updated)
            self.assertNotIn('p03PanelWords', updated)
            self.assertNotIn('model.attributionPanel', updated)
            capture = updated[updated.index('func captureP03('):]
            self.assertLess(capture.index('let panelOracle = try p03PanelOracle(publication.data, target: target)'), capture.index('let issued ='))
            self.assertGreater(capture.index('let control = try p03Control(kind, value)'), capture.index('let previousBody = try p03ContentImage(previous.image)'))
            self.assertGreater(capture.index('let control = try p03Control(kind, value)'), capture.index('negativePreflight = true'))
            self.assertIn('p03ReplaceBody(frame.image', capture)
            self.assertLess(updated.index('NSApp.postEvent(events[1]'), updated.index('window.sendEvent(events[0]'))
            self.assertIn('override func hitTest(_ point: NSPoint) -> NSView? { nil }', updated)
            self.assertNotIn('accessibilityIdentifier("p03.', (root/paths[2]).read_text())
            self.assertLess(updated.index('let frame = try baselineFrame()', updated.index('func captureP03(')), updated.index('let recognized =', updated.index('func captureP03(')))
            surface = (root/paths[2]).read_text()
            self.assertLess(surface.index('P03Action.begin("period"'), surface.index('model.selectedPeriod = option.id'))
            self.assertEqual((root/paths[3]).read_text().count('P03HelperLaunches.record()'), 2)


if __name__ == '__main__':
    unittest.main()
