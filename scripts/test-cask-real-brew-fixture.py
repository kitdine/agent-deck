#!/usr/bin/env python3
"""Fixture preparation regression; no brew, signing, or system registration."""

from pathlib import Path
import ast
import os
import plistlib
import shutil
import subprocess
import tempfile
import unittest
from unittest import mock

import cask_real_brew_fixture as fixture_module
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


class FixtureCommandTests(unittest.TestCase):
    def runner(self):
        source = Path(__file__).with_name('test-cask-real-brew.sh').read_text()
        driver = source.split("<<'PY'\n", 1)[1].rsplit('\nPY', 1)[0]
        function = next(node for node in ast.parse(driver).body
                        if isinstance(node, ast.FunctionDef) and node.name == 'run')
        namespace = dict(vars(fixture_module), subprocess=subprocess,
                         base_env={'FIXTURE_ENV': 'base'})
        exec(compile(ast.Module(body=[function], type_ignores=[]), '<actual CI run wrapper>', 'exec'), namespace)
        return namespace['run']

    def test_timeout_reports_partial_output_and_preserves_budget(self):
        for output in (b'==> Upgrading fixture\nwaiting at upgrade marker\n',
                       b'waiting at upgrade marker\xff\n',
                       '==> Upgrading fixture\nwaiting at upgrade marker\n'):
            with self.subTest(output=output):
                failure = subprocess.TimeoutExpired(['/fixture/brew', 'upgrade'], 180, output=output)
                with mock.patch('subprocess.run', side_effect=failure) as execute:
                    with self.assertRaises(Exception) as raised:
                        self.runner()([Path('/fixture/brew'), 'upgrade'])
                self.assertIn('waiting at upgrade marker', str(raised.exception))
                self.assertIn('180', str(raised.exception))
                self.assertIsInstance(raised.exception, AssertionError)
                self.assertIs(raised.exception.__cause__, failure)
                self.assertEqual(execute.call_args.kwargs['timeout'], 180)
                self.assertEqual(execute.call_args.kwargs['env'], {'FIXTURE_ENV': 'base'})
                self.assertEqual(execute.call_args.args[0], ['/fixture/brew', 'upgrade'])

    def test_empty_timeout_output_does_not_satisfy_expected_failure(self):
        failure = subprocess.TimeoutExpired(['/fixture/brew', 'upgrade'], 180)
        with mock.patch('subprocess.run', side_effect=failure):
            with self.assertRaises(Exception) as raised:
                self.runner()(['/fixture/brew', 'upgrade'], success=False)
        self.assertIn('no captured subprocess output', str(raised.exception))
        self.assertIs(raised.exception.__cause__, failure)

    def test_command_exit_contract_is_unchanged(self):
        for code, success in ((0, True), (1, False), (1, True), (0, False)):
            with self.subTest(code=code, success=success):
                result = subprocess.CompletedProcess(['/fixture/tool'], code, stdout='command detail\n')
                with mock.patch('subprocess.run', return_value=result) as execute:
                    call = lambda: self.runner()(['/fixture/tool'], env={'FIXTURE_ENV': 'explicit'},
                                                 success=success, timeout=17)
                    if (code == 0) == success:
                        self.assertEqual(call(), 'command detail\n')
                    else:
                        with self.assertRaisesRegex(AssertionError, 'command detail'):
                            call()
                self.assertEqual(execute.call_args.kwargs['timeout'], 17)
                self.assertEqual(execute.call_args.kwargs['env'], {'FIXTURE_ENV': 'explicit'})


class FixtureScenarioTests(unittest.TestCase):
    def test_local_cask_keeps_default_core_metadata_mode(self):
        source = Path(__file__).with_name('test-cask-real-brew.sh').read_text()
        driver = source.split("<<'PY'\n", 1)[1].rsplit('\nPY', 1)[0]
        scenario = next(node for node in ast.parse(driver).body
                        if isinstance(node, ast.ClassDef) and node.name == 'Scenario')
        for inherited in (None, '', '1'):
            with self.subTest(inherited=inherited), tempfile.TemporaryDirectory() as temporary:
                workspace = Path(temporary)
                repository = workspace / 'repository'
                (repository / 'bin').mkdir(parents=True)
                (repository / 'bin/brew').write_text('not an executable test dependency\n')
                base_env = {'HOME': '/unused-home', 'FIXTURE_SENTINEL': 'inherited'}
                if inherited is not None:
                    base_env['HOMEBREW_NO_INSTALL_FROM_API'] = inherited
                expected_base = dict(base_env)

                def run(argv, **_kwargs):
                    return str(workspace / 'metadata-mode/brew') if argv[-1] == '--prefix' else 'command output'

                execute = mock.Mock(side_effect=run)
                namespace = dict(Path=Path, shutil=shutil, workspace=workspace,
                                 brew_repository=repository, base_env=base_env,
                                 run=execute, print=mock.Mock())
                exec(compile(ast.Module(body=[scenario], type_ignores=[]), '<actual CI Scenario>', 'exec'), namespace)
                actual = namespace['Scenario']
                with mock.patch.object(actual, 'render') as render:
                    fixture = actual('metadata-mode')
                render.assert_called_once_with('1.2.3')
                self.assertNotIn('HOMEBREW_NO_INSTALL_FROM_API', fixture.env)
                self.assertEqual(base_env, expected_base)
                self.assertEqual(fixture.env['HOME'], str(fixture.home))
                self.assertEqual(fixture.env['HOMEBREW_CACHE'], str(fixture.directory / 'cache'))
                self.assertEqual(fixture.env['HOMEBREW_NO_AUTO_UPDATE'], '1')
                self.assertEqual(fixture.env['HOMEBREW_NO_INSTALL_CLEANUP'], '1')
                self.assertEqual(fixture.env['HOMEBREW_AGENTDECK_ACCEPTANCE_ACCOUNT_HOME'], str(fixture.account))
                self.assertEqual(fixture.env['FIXTURE_SENTINEL'], 'inherited')
                fixture.command('upgrade')
                self.assertEqual(execute.call_args.args[0], [
                    fixture.brew, 'upgrade', '--cask', f'--appdir={fixture.appdir}',
                    'agentdeck/test/agentdeck-app',
                ])
                self.assertNotIn('HOMEBREW_NO_INSTALL_FROM_API', execute.call_args.kwargs['env'])


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

    def test_widget_host_probe_never_replaces_original_failure(self):
        bundle = Path('/fixture/AgentDeck.app')
        checker = Path('/fixture/check.sh')
        registrar = '/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister'
        for probe in ('baseline-pass', 'pass', 'check-fails', 'register-fails', 'cleanup-fails'):
            with self.subTest(probe=probe):
                calls = []
                original = AssertionError('original registration: no matches')
                checks = 0

                def run(argv):
                    nonlocal checks
                    calls.append(argv)
                    if argv[0] == 'bash':
                        checks += 1
                        if checks == 1:
                            raise original
                        if checks == 2 and probe != 'baseline-pass':
                            raise AssertionError('no matches without host registration')
                        if probe == 'check-fails':
                            raise AssertionError('still no matches after host registration')
                    if argv[0] == registrar:
                        if '-f' in argv and probe == 'register-fails':
                            raise AssertionError('host registration refused')
                        if '-u' in argv and probe == 'cleanup-fails':
                            raise AssertionError('host probe cleanup refused')
                    return 'diagnostic output'

                with self.assertRaises(AssertionError) as raised:
                    verify_fixture_widget_registration(bundle, checker, run)
                self.assertIs(raised.exception.__cause__, original)
                self.assertIn('original registration: no matches', str(raised.exception))
                expected = [] if probe == 'baseline-pass' else [
                    [registrar, '-f', '-v', bundle], [registrar, '-u', bundle]]
                self.assertEqual([argv for argv in calls if argv[0] == registrar], expected)
                self.assertEqual(sum(argv[0] == '/usr/bin/pluginkit' for argv in calls), 1)
                self.assertEqual(checks, 2 if probe in ('baseline-pass', 'register-fails') else 3)
                if probe in ('pass', 'cleanup-fails'):
                    self.assertIn('Exact widget check after host registration: PASS', str(raised.exception))
                elif probe == 'check-fails':
                    self.assertIn('still no matches after host registration', str(raised.exception))
                elif probe == 'register-fails':
                    self.assertIn('host registration refused', str(raised.exception))
                else:
                    self.assertIn('Exact widget check without host registration: PASS', str(raised.exception))
                if probe == 'cleanup-fails':
                    self.assertIn('host probe cleanup refused', str(raised.exception))

    def test_widget_add_failure_does_not_probe_host_discovery(self):
        calls = []

        def run(argv):
            calls.append(argv)
            if argv[0] == '/usr/bin/pluginkit':
                raise AssertionError('add rejected')
            return 'fixture metadata'

        with self.assertRaisesRegex(AssertionError, 'add rejected'):
            verify_fixture_widget_registration(Path('/fixture/AgentDeck.app'), Path('/fixture/check.sh'), run)
        self.assertFalse(any('lsregister' in str(argv[0]) for argv in calls))

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
