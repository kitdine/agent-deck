#!/usr/bin/env python3
"""Fixture preparation regression; no brew, signing, or system registration."""

from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

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

            def run(argv):
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
                    return subprocess.check_output(argv, text=True)

            prepare_fixture_app(fixture, bundle, helper, '1.2.3', run)
            self.assertEqual(signed, [bundle])
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

                def run(argv):
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


if __name__ == '__main__':
    unittest.main()
