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
from cask_real_brew_fixture import prepare_fixture_app


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
                    return str(Path(argv[0]).parent.parent) if argv[-1] == '--prefix' else 'command output'

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
                # Homebrew reads the appdir back from its own install record on
                # uninstall and rejects the option there.
                fixture.command('uninstall')
                self.assertEqual(execute.call_args.args[0], [fixture.brew, 'uninstall', '--cask',
                                                             'agentdeck/test/agentdeck-app'])
                # The step may write below brew's HOME only, so the account is
                # HOME unless a case asks for the mismatch.
                self.assertEqual(fixture.account, fixture.home)
                with mock.patch.object(actual, 'render'):
                    mismatch = actual('metadata-mode-mismatch', account='account')
                self.assertEqual(mismatch.account, mismatch.directory / 'account')
                self.assertNotEqual(mismatch.account, mismatch.home)
                self.assertEqual(mismatch.env['HOMEBREW_AGENTDECK_ACCEPTANCE_ACCOUNT_HOME'], str(mismatch.account))


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



def home_mismatch_code():
    """The CI script's own HOME-mismatch judgement, not a copy of it."""
    source = Path(__file__).with_name('test-cask-real-brew.sh').read_text()
    driver = source.split("<<'PY'\n", 1)[1].rsplit('\nPY', 1)[0]
    wanted = {'control_cask', 'judge_sandbox_control', 'judge_home_mismatch'}
    body = [node for node in ast.parse(driver).body
            if (isinstance(node, ast.FunctionDef) and node.name in wanted) or
            (isinstance(node, ast.Assign) and getattr(node.targets[0], 'id', '') == 'CONTROL_MARKER')]
    assert {node.name for node in body if isinstance(node, ast.FunctionDef)} == wanted
    import shlex
    namespace = {'shlex': shlex}
    exec(compile(ast.Module(body=body, type_ignores=[]), '<actual CI HOME-mismatch case>', 'exec'), namespace)
    return namespace


class HomeMismatchJudgementTests(unittest.TestCase):
    support = Path('/fixture/account/Library/Application Support/AgentDeck')
    denial = ('Error: chmod /fixture/account/Library/Application Support/AgentDeck: operation not permitted\n')

    def judge(self, returncode, output, receipts=(), app=False, caskroom=False):
        return home_mismatch_code()['judge_home_mismatch'](returncode, output, self.support, list(receipts), app, caskroom)

    def test_helper_denied_at_the_account_is_the_expected_refusal(self):
        self.judge(1, '==> Moving App\n' + self.denial + 'Error: step failed\n')

    def test_failure_before_the_registration_step_is_not_the_refusal(self):
        # ISO-IR-R5-F1: nothing reached the helper, yet the install failed,
        # left nothing behind, and said "permission denied".
        for output in ('curl: could not create download cache: permission denied\n',
                       'Error: Operation not permitted @ dir_s_mkdir - /fixture/home/cache\n',
                       'Error: chmod /fixture/other/Library/Application Support/AgentDeck: operation not permitted\n',
                       'Error: download failed\n', ''):
            with self.subTest(output=output):
                with self.assertRaisesRegex(AssertionError, 'before the registration step'):
                    self.judge(1, output)

    def test_denial_text_and_account_path_must_be_on_one_line(self):
        output = 'note: /fixture/account/Library/Application Support/AgentDeck\ncurl: permission denied\n'
        with self.assertRaisesRegex(AssertionError, 'before the registration step'):
            self.judge(1, output)

    def test_success_or_leftovers_are_not_a_rollback(self):
        for returncode, app, caskroom in ((0, False, False), (1, True, False), (1, False, True)):
            with self.subTest(returncode=returncode, app=app, caskroom=caskroom):
                with self.assertRaisesRegex(AssertionError, 'did not fail and roll back'):
                    self.judge(returncode, self.denial, app=app, caskroom=caskroom)

    def test_any_receipt_reports_the_design_assumption(self):
        for returncode in (0, 1):
            with self.assertRaisesRegex(AssertionError, '^DESIGN: '):
                self.judge(returncode, self.denial, receipts=['/fixture/account/installation.json'])

    def control(self, returncode, output, declared_written=True, account_written=False):
        return home_mismatch_code()['judge_sandbox_control'](returncode, output, declared_written, account_written)

    def test_control_requires_the_step_to_have_run(self):
        self.control(0, '==> step\nSANDBOX_CONTROL_declared=allowed\nSANDBOX_CONTROL_account=denied\n')
        for returncode, output in ((1, 'curl: could not create download cache: permission denied\n'),
                                   (0, 'SANDBOX_CONTROL_account=denied\n'),
                                   (1, 'SANDBOX_CONTROL_declared=allowed\nSANDBOX_CONTROL_account=denied\n')):
            with self.subTest(returncode=returncode, output=output):
                with self.assertRaisesRegex(AssertionError, 'proves nothing'):
                    self.control(returncode, output)

    def test_control_denial_counts_only_beside_an_allowed_declared_write(self):
        output = 'SANDBOX_CONTROL_declared=denied\nSANDBOX_CONTROL_account=denied\n'
        with self.assertRaisesRegex(AssertionError, 'not evidence'):
            self.control(0, output)
        with self.assertRaisesRegex(AssertionError, 'not evidence'):
            self.control(0, 'SANDBOX_CONTROL_declared=allowed\nSANDBOX_CONTROL_account=denied\n', declared_written=False)

    def test_control_reports_the_design_assumption_when_the_account_is_writable(self):
        with self.assertRaisesRegex(AssertionError, '^DESIGN: '):
            self.control(0, 'SANDBOX_CONTROL_declared=allowed\nSANDBOX_CONTROL_account=allowed\n')
        # A step that says "denied" while its marker exists was not denied.
        with self.assertRaisesRegex(AssertionError, '^DESIGN: '):
            self.control(0, 'SANDBOX_CONTROL_declared=allowed\nSANDBOX_CONTROL_account=denied\n', account_written=True)

    def test_control_cask_replaces_only_the_step_script_and_probes_both_directories(self):
        code = home_mismatch_code()
        root = Path(__file__).resolve().parents[1]
        with tempfile.TemporaryDirectory() as temporary:
            base = Path(temporary).resolve()
            checksums = base / 'checksums.txt'
            checksums.write_text('c' * 64 + '  AgentDeck_v1.2.3_universal.dmg\n')
            rendered = base / 'agentdeck-app.rb'
            subprocess.run(['bash', root / 'scripts/render-homebrew-cask.sh',
                            root / 'packaging/homebrew/agentdeck-app.rb.tmpl', 'v1.2.3', checksums, rendered], check=True)
            declared, parent = base / 'home/declared dir', base / 'locked'
            parent.mkdir()
            account = parent / "account's dir"
            production = rendered.read_text()
            control = code['control_cask'](production, declared, account)
            rendered.write_text(control)
            subprocess.run(['ruby', '-c', rendered], check=True, stdout=subprocess.DEVNULL)

            def outside_script(text):
                lines = text.splitlines()
                start = next(i for i, line in enumerate(lines) if '"agentdeck-cask-register"' in line)
                end = next(i for i in range(start + 1, len(lines)) if lines[i].strip() == 'SH')
                return lines[:start + 1] + lines[end:]

            # The sandbox declarations, the arguments and every other stanza
            # are the production cask's own.
            self.assertEqual(outside_script(control), outside_script(production))
            self.assertNotIn('install register', control.split('"agentdeck-cask-register"', 1)[1].split('\n        SH\n', 1)[0])

            def step():
                argv = subprocess.run(['ruby', root / 'scripts/cask-local-driver.rb', rendered, base / 'Applications',
                                       base / 'prefix', 'postflight-argv'], check=True, stdout=subprocess.PIPE).stdout
                return subprocess.run([item.decode() for item in argv.split(b'\0')], text=True,
                                      stdout=subprocess.PIPE, stderr=subprocess.STDOUT)

            parent.chmod(0o500)
            try:
                denied = step()
            finally:
                parent.chmod(0o700)
            self.assertEqual((denied.returncode, denied.stdout),
                             (0, 'SANDBOX_CONTROL_declared=allowed\nSANDBOX_CONTROL_account=denied\n'))
            marker = code['CONTROL_MARKER']
            self.assertTrue((declared / marker).is_file())
            self.assertFalse(account.exists())
            code['judge_sandbox_control'](denied.returncode, denied.stdout, True, False)
            allowed = step()
            self.assertEqual((allowed.returncode, allowed.stdout),
                             (0, 'SANDBOX_CONTROL_declared=allowed\nSANDBOX_CONTROL_account=allowed\n'))
            self.assertTrue((account / marker).is_file())


if __name__ == '__main__':
    unittest.main()
