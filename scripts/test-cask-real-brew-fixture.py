#!/usr/bin/env python3
"""Fixture preparation regression; no brew, signing, or system registration."""

from pathlib import Path
import os
import plistlib
import shutil
import subprocess
import tempfile
import unittest
from unittest import mock

from cask_real_brew_fixture import prepare_fixture_app, verify_fixture_widget_registration


def write_fixture_metadata(bundle, group='TESTTEAM.group.com.kitdine.agentdeck.fixture'):
    widget = bundle / 'Contents/PlugIns/AgentDeckWidget.appex'
    (widget / 'Contents').mkdir(parents=True)
    for target, identifier in ((bundle, 'com.kitdine.agentdeck.fixture'),
                               (widget, 'com.kitdine.agentdeck.fixture.widget')):
        (target / 'Contents/Info.plist').write_bytes(plistlib.dumps({
            'CFBundleIdentifier': identifier,
            'AgentDeckAppGroupIdentifier': group,
        }))


class FixtureTests(unittest.TestCase):
    def test_fixture_signing_resolves_entitlements_and_seals_inside_out(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            fixture = root / 'Debug.app'
            (fixture / 'Contents/Helpers').mkdir(parents=True)
            (fixture / 'Contents/Resources').mkdir()
            write_fixture_metadata(fixture)
            (fixture / 'Contents/Frameworks/AgentDeckShared.framework').mkdir(parents=True)
            helper = root / 'helper'
            helper.write_text('replacement helper')
            bundle = root / '1.2.3/AgentDeck.app'
            signatures = []
            verified = []

            def run(argv, *, env=None):
                if argv[0] == '/usr/bin/ditto':
                    shutil.copytree(argv[1], argv[2])
                elif argv[0] == '/usr/bin/codesign':
                    if '--sign' in argv:
                        entitlements = None
                        if '--entitlements' in argv:
                            entitlements = plistlib.loads(Path(argv[argv.index('--entitlements') + 1]).read_bytes())
                        signatures.append((argv[-1], entitlements, argv))
                    else:
                        verified.append(argv)
                else:
                    return 'generated completion\n'

            prepare_fixture_app(fixture, bundle, helper, '1.2.3', run)
            widget = bundle / 'Contents/PlugIns/AgentDeckWidget.appex'
            self.assertEqual([target for target, _, _ in signatures], [
                bundle / 'Contents/Helpers/agentdeck',
                bundle / 'Contents/Frameworks/AgentDeckShared.framework', widget, bundle,
            ])
            self.assertEqual(signatures[2][1], {
                'com.apple.security.app-sandbox': True,
                'com.apple.security.application-groups': ['TESTTEAM.group.com.kitdine.agentdeck.fixture'],
            })
            self.assertEqual(signatures[3][1], {
                'com.apple.security.application-groups': ['TESTTEAM.group.com.kitdine.agentdeck.fixture'],
            })
            for _, _, argv in signatures:
                self.assertNotIn('--deep', argv)
            self.assertTrue(any('--verify' in argv and '--deep' in argv and argv[-1] == bundle for argv in verified))

    def test_fixture_signing_refuses_unresolved_or_mismatched_groups(self):
        for group in ('', '$(AGENTDECK_APP_GROUP)', 'mismatch'):
            with self.subTest(group=group), tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary)
                fixture = root / 'Debug.app'
                (fixture / 'Contents/Helpers').mkdir(parents=True)
                (fixture / 'Contents/Resources').mkdir()
                write_fixture_metadata(fixture, group)
                if group == 'mismatch':
                    info = fixture / 'Contents/PlugIns/AgentDeckWidget.appex/Contents/Info.plist'
                    values = plistlib.loads(info.read_bytes())
                    values['AgentDeckAppGroupIdentifier'] = 'another.group'
                    info.write_bytes(plistlib.dumps(values))
                helper = root / 'helper'
                helper.write_text('replacement')
                signatures = []

                def run(argv, *, env=None):
                    if argv[0] == '/usr/bin/ditto':
                        shutil.copytree(argv[1], argv[2])
                    elif argv[0] == '/usr/bin/codesign':
                        signatures.append(argv)
                    else:
                        return 'completion\n'

                with self.assertRaisesRegex(ValueError, 'matching resolved App Group'):
                    prepare_fixture_app(fixture, root / '1.2.3/AgentDeck.app', helper, '1.2.3', run)
                self.assertEqual(signatures, [])

    def test_fixture_signing_never_seals_host_after_nested_failure(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            fixture = root / 'Debug.app'
            (fixture / 'Contents/Helpers').mkdir(parents=True)
            (fixture / 'Contents/Resources').mkdir()
            write_fixture_metadata(fixture)
            helper = root / 'helper'
            helper.write_text('replacement')
            bundle = root / '1.2.3/AgentDeck.app'
            widget = bundle / 'Contents/PlugIns/AgentDeckWidget.appex'
            signatures = []

            def run(argv, *, env=None):
                if argv[0] == '/usr/bin/ditto':
                    shutil.copytree(argv[1], argv[2])
                elif argv[0] == '/usr/bin/codesign':
                    signatures.append(argv[-1])
                    if argv[-1] == widget:
                        raise AssertionError('nested signing failure')
                else:
                    return 'completion\n'

            with self.assertRaisesRegex(AssertionError, 'nested signing failure'):
                prepare_fixture_app(fixture, bundle, helper, '1.2.3', run)
            self.assertEqual(signatures, [bundle / 'Contents/Helpers/agentdeck', widget])

    def test_widget_registration_success_still_requires_exact_checker(self):
        bundle = Path('/fixture/AgentDeck.app')
        checker = Path('/fixture/check-registration.sh')
        calls = []

        def run(argv):
            calls.append(argv)
            return ''

        verify_fixture_widget_registration(bundle, checker, run)
        self.assertEqual(calls, [
            ['/usr/bin/pluginkit', '-v', '-a', bundle / 'Contents/PlugIns/AgentDeckWidget.appex'],
            ['bash', checker, bundle],
        ])

    def test_widget_diagnostics_preserve_failure_without_retry_or_fallback(self):
        bundle = Path('/fixture/AgentDeck.app')
        for failed_step in ('add', 'check'):
            with self.subTest(failed_step=failed_step):
                calls = []

                def run(argv):
                    calls.append(argv)
                    if argv[0] == '/usr/bin/pluginkit':
                        if failed_step == 'add':
                            raise AssertionError('add rejected')
                        return 'add accepted'
                    if argv[0] == 'bash':
                        raise AssertionError('registration: no matches')
                    if argv[1] == '--verify':
                        raise AssertionError('signature diagnostic')
                    return 'fixture metadata'

                with self.assertRaises(AssertionError) as raised:
                    verify_fixture_widget_registration(bundle, Path('/fixture/check.sh'), run)
                self.assertIn('add rejected' if failed_step == 'add' else 'registration: no matches', str(raised.exception))
                self.assertIn('signature diagnostic', str(raised.exception))
                self.assertIn('fixture metadata', str(raised.exception))
                self.assertEqual(sum(argv[0] == '/usr/bin/pluginkit' for argv in calls), 1)
                self.assertEqual(sum(argv[0] in ('/usr/bin/codesign', '/usr/bin/plutil') for argv in calls), 6)

    def test_completions_are_generated_from_replacement_helper_before_signing(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            fixture = root / 'Debug App.app'
            for path in ('Contents/Helpers', 'Contents/Resources'):
                (fixture / path).mkdir(parents=True)
            write_fixture_metadata(fixture)
            (fixture / 'Contents/Helpers/agentdeck').write_text('original helper')
            helper = root / 'acceptance helper'
            helper.write_text('#!/bin/sh\nprintf "completion for %s\\n" "$2"\n')
            helper.chmod(0o755)
            bundle = root / '1.2.3/AgentDeck.app'
            signed = []

            def run(argv, *, env=None):
                if argv[0] == '/usr/bin/ditto':
                    shutil.copytree(argv[1], argv[2])
                elif argv[0] == '/usr/bin/codesign':
                    for shell in ('bash', 'zsh', 'fish'):
                        path = bundle / f'Contents/Resources/completions/agentdeck.{shell}'
                        self.assertEqual(path.read_text(), f'completion for {shell}\n')
                        self.assertEqual(path.stat().st_mode & 0o777, 0o644)
                    if '--sign' in argv:
                        signed.append(argv[-1])
                else:
                    self.assertEqual(Path(argv[0]), bundle / 'Contents/Helpers/agentdeck')
                    self.assertEqual(env['FIXTURE_MARKER'], 'preserved')
                    self.assertNotIn('HOMEBREW_AGENTDECK_REPAIR', env)
                    self.assertNotEqual(env['HOME'], str(root / 'outer-home'))
                    return subprocess.check_output(argv, env=env, text=True)

            base_env = dict(PATH=os.defpath, HOME=str(root / 'outer-home'),
                            FIXTURE_MARKER='preserved', HOMEBREW_AGENTDECK_REPAIR='1')
            prepare_fixture_app(fixture, bundle, helper, '1.2.3', run, env=base_env)
            self.assertEqual(signed, [bundle / 'Contents/Helpers/agentdeck',
                                      bundle / 'Contents/PlugIns/AgentDeckWidget.appex', bundle])
            self.assertEqual(base_env['HOME'], str(root / 'outer-home'))
            self.assertEqual(base_env['HOMEBREW_AGENTDECK_REPAIR'], '1')
            self.assertEqual((bundle / 'Contents/Resources/version').read_text(), '1.2.3\n')
            self.assertEqual((fixture / 'Contents/Helpers/agentdeck').read_text(), 'original helper')
            self.assertFalse((fixture / 'Contents/Resources/completions').exists())

    def test_empty_or_failed_completion_never_signs(self):
        for output in ('', '   \n', None):
            with self.subTest(output=output), tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary)
                fixture = root / 'Debug.app'
                (fixture / 'Contents/Helpers').mkdir(parents=True)
                (fixture / 'Contents/Resources').mkdir()
                write_fixture_metadata(fixture)
                helper = root / 'helper'
                helper.write_text('replacement')
                signed = []

                def run(argv, *, env=None):
                    if argv[0] == '/usr/bin/ditto':
                        shutil.copytree(argv[1], argv[2])
                    elif argv[0] == '/usr/bin/codesign':
                        signed.append(argv)
                    elif output is None:
                        raise subprocess.CalledProcessError(1, argv)
                    else:
                        return output

                with self.assertRaises((ValueError, subprocess.CalledProcessError)):
                    prepare_fixture_app(fixture, root / 'version/AgentDeck.app', helper, '1.2.3', run)
                self.assertEqual(signed, [])

    def test_real_helper_uses_private_completion_environment(self):
        repository = Path(__file__).resolve().parents[1]
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            fixture = root / 'Debug.app'
            (fixture / 'Contents/Helpers').mkdir(parents=True)
            (fixture / 'Contents/Resources').mkdir()
            write_fixture_metadata(fixture)
            (fixture / 'Contents/Helpers/agentdeck').write_text('original helper')
            helper = root / 'acceptance-helper'
            build_env = dict(os.environ)
            build_env.setdefault('GOCACHE', '/private/tmp/agent-deck-go-build')
            subprocess.run(['go', 'build', '-mod=vendor', '-trimpath', '-tags',
                            'agentdeck_acceptance', '-o', str(helper), './cmd/agentdeck'],
                           cwd=repository, env=build_env, check=True)
            home = root / 'outer-home'
            home.mkdir()
            base_env = {name: value for name, value in os.environ.items()
                        if not name.startswith(('AGENTDECK_ACCEPTANCE_', 'HOMEBREW_AGENTDECK_'))}
            base_env.update(HOME=str(home), HOMEBREW_AGENTDECK_REPAIR='1',
                            AGENTDECK_ACCEPTANCE_HOOK_DIR=str(root / 'inherited-hook'))
            refused = subprocess.run([str(helper), 'completion', 'bash'], env=base_env,
                                     text=True, capture_output=True)
            self.assertEqual(refused.returncode, 1)
            self.assertIn('reason: real_root_unresolved', refused.stderr)
            bundle = root / '1.2.3/AgentDeck.app'
            completion_environments = []
            signed = []

            def run(argv, *, env=None):
                if argv[0] == '/usr/bin/ditto':
                    shutil.copytree(argv[1], argv[2])
                elif argv[0] == '/usr/bin/codesign':
                    if '--sign' in argv:
                        signed.append(argv[-1])
                else:
                    output = subprocess.check_output(argv, env=env or base_env,
                                                     text=True, stderr=subprocess.STDOUT)
                    self.assertEqual(list(Path(env['HOME']).iterdir()), [])
                    self.assertEqual(list(Path(env['AGENTDECK_ACCEPTANCE_ACCOUNT_HOME']).iterdir()), [])
                    completion_environments.append(env)
                    return output

            with mock.patch.dict(os.environ, base_env, clear=True):
                prepare_fixture_app(fixture, bundle, helper, '1.2.3', run)
            self.assertEqual(signed, [bundle / 'Contents/Helpers/agentdeck',
                                      bundle / 'Contents/PlugIns/AgentDeckWidget.appex', bundle])
            self.assertEqual(len(completion_environments), 3)
            for env in completion_environments:
                self.assertNotEqual(env['HOME'], str(home))
                self.assertNotEqual(env['HOME'], env['AGENTDECK_ACCEPTANCE_ACCOUNT_HOME'])
                self.assertNotIn('AGENTDECK_ACCEPTANCE_HOOK_DIR', env)
                self.assertNotIn('HOMEBREW_AGENTDECK_REPAIR', env)
                self.assertFalse(Path(env['HOME']).exists())
                self.assertFalse(Path(env['AGENTDECK_ACCEPTANCE_ACCOUNT_HOME']).exists())
            for shell in ('bash', 'zsh', 'fish'):
                self.assertTrue((bundle / f'Contents/Resources/completions/agentdeck.{shell}').read_text().strip())
            self.assertEqual(list(home.iterdir()), [])
            self.assertFalse((root / 'inherited-hook').exists())
            self.assertEqual((fixture / 'Contents/Helpers/agentdeck').read_text(), 'original helper')


if __name__ == '__main__':
    unittest.main()
