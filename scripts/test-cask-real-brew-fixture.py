#!/usr/bin/env python3
"""Fixture preparation regression; no brew, signing, or system registration."""

from pathlib import Path
import os
import shutil
import subprocess
import tempfile
import unittest
from unittest import mock

from cask_real_brew_fixture import prepare_fixture_app


class FixtureTests(unittest.TestCase):
    def test_completions_are_generated_from_replacement_helper_before_signing(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            fixture = root / 'Debug App.app'
            for path in ('Contents/Helpers', 'Contents/Resources'):
                (fixture / path).mkdir(parents=True)
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
            self.assertEqual(signed, [bundle])
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
            self.assertEqual(signed, [bundle])
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
