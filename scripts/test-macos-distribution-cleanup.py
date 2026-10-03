#!/usr/bin/env python3
"""Exercise the distribution script's actual EXIT lifecycle without native writes."""
import os
from pathlib import Path
import subprocess
import tempfile
import unittest


SCRIPT = Path(__file__).with_name("test-macos-distribution.sh")


class DistributionCleanupTests(unittest.TestCase):
    def run_cleanup(self, exit_code=0, unregister_failure=False, detach_failure=False,
                    not_found=False, still_registered=False, dump_failure=False,
                    path_name="fixtures with spaces"):
        with tempfile.TemporaryDirectory(prefix="agentdeck-cleanup-test-") as case:
            case = Path(case).resolve()
            temporary = case / path_name
            widget = temporary / "AgentDeck.app/Contents/PlugIns/AgentDeckWidget.appex"
            mounted = temporary / "mount/AgentDeck.app"
            extracted = temporary / "extracted/AgentDeck.app"
            for bundle in (widget, mounted, extracted):
                bundle.mkdir(parents=True)
            tap = case / "owned-tap"
            tap.mkdir()
            unrelated = case / "unrelated.app"
            unrelated.mkdir()
            log = case / "calls"
            bins = case / "bin"
            bins.mkdir()
            stubs = {
                "mktemp": '#!/bin/bash\nprintf "%s\\n" "$TEST_TEMPORARY"\n',
                "lsregister": '''#!/bin/bash
set -eu
if [[ $1 == -dump ]]; then
  if [[ ${TEST_STILL_REGISTERED:-0} == 1 ]]; then
    printf 'path:                       %s (0x123)\\n' "$TEST_WIDGET"
  fi
  exit "${TEST_DUMP_FAILURE:-0}"
fi
[ "$1" = -u ]
[ -d "$2" ] || exit 98
printf 'unregister\\0%s\\0' "$2" >> "$TEST_LOG"
if [[ ${TEST_UNREGISTER_FAILURE:-0} == 1 && $2 == *.appex ]]; then exit 9; fi
if [[ ${TEST_NOT_FOUND:-0} == 1 ]]; then
  printf 'failed to scan %s: -10814\\n' "$2" >&2
  exit 1
fi
''',
                "hdiutil": '''#!/bin/bash
set -eu
[ "$1" = detach ]
printf 'detach\\0%s\\0' "${@: -1}" >> "$TEST_LOG"
exit "${TEST_DETACH_FAILURE:-0}"
''',
            }
            for name, source in stubs.items():
                stub = bins / name
                stub.write_text(source)
                stub.chmod(0o755)
            # Run the real prologue and EXIT trap, then simulate completed work.
            # No copy of the cleanup implementation lives in this test.
            prologue = SCRIPT.read_text().split("\ncask_template=", 1)[0]
            driver = case / "driver.sh"
            driver.write_text(prologue + '''
fixture_root=$TEST_TAP
stub_mount="$temporary/mount"
stub_dmg_attached=1
exit "$TEST_EXIT"
''')
            env = dict(os.environ, PATH=f"{bins}:{os.environ['PATH']}",
                       AGENTDECK_LSREGISTER=str(bins / "lsregister"),
                       TEST_TEMPORARY=str(temporary), TEST_TAP=str(tap),
                       TEST_LOG=str(log), TEST_EXIT=str(exit_code),
                       TEST_UNREGISTER_FAILURE=str(int(unregister_failure)),
                       TEST_DETACH_FAILURE=str(int(detach_failure)),
                       TEST_NOT_FOUND=str(int(not_found)),
                       TEST_STILL_REGISTERED=str(int(still_registered)),
                       TEST_DUMP_FAILURE=str(int(dump_failure)), TEST_WIDGET=str(widget))
            result = subprocess.run(["bash", str(driver)], env=env, capture_output=True)
            entries = log.read_bytes().split(b"\0")[:-1] if log.exists() else []
            calls = list(zip(entries[::2], entries[1::2]))
            return {"code": result.returncode, "calls": calls,
                    "temporary_exists": temporary.exists(), "tap_exists": tap.exists(),
                    "unrelated_exists": unrelated.exists(), "stderr": result.stderr,
                    "widget": os.fsencode(widget), "host": os.fsencode(widget.parents[2]),
                    "mounted": os.fsencode(mounted), "extracted": os.fsencode(extracted)}

    def assert_unregister_order(self, result):
        calls = result["calls"]
        for name in ("widget", "host", "mounted", "extracted"):
            self.assertIn((b"unregister", result[name]), calls)
        self.assertLess(calls.index((b"unregister", result["widget"])),
                        calls.index((b"unregister", result["host"])))
        detach = next(i for i, call in enumerate(calls) if call[0] == b"detach")
        self.assertLess(calls.index((b"unregister", result["mounted"])), detach)
        self.assertTrue(result["unrelated_exists"])

    def test_success_unregisters_before_detach_and_removal(self):
        result = self.run_cleanup()
        self.assert_unregister_order(result)
        self.assertEqual(result["code"], 0, result["stderr"])
        self.assertFalse(result["temporary_exists"])
        self.assertFalse(result["tap_exists"])

    def test_failure_preserves_exit_and_still_cleans_owned_fixtures(self):
        result = self.run_cleanup(exit_code=37)
        self.assert_unregister_order(result)
        self.assertEqual(result["code"], 37, result["stderr"])
        self.assertFalse(result["temporary_exists"])
        self.assertFalse(result["tap_exists"])

    def test_unregister_failure_attempts_remaining_paths_and_keeps_fixtures(self):
        result = self.run_cleanup(unregister_failure=True)
        self.assert_unregister_order(result)
        self.assertNotEqual(result["code"], 0)
        self.assertTrue(result["temporary_exists"])
        self.assertFalse(result["tap_exists"])
        self.assertIn(b"unregister", result["stderr"])

    def test_detach_failure_keeps_mount_and_fails_successful_run(self):
        result = self.run_cleanup(detach_failure=True)
        self.assert_unregister_order(result)
        self.assertNotEqual(result["code"], 0)
        self.assertTrue(result["temporary_exists"])
        self.assertFalse(result["tap_exists"])
        self.assertIn(b"detach", result["stderr"])

    def test_not_found_succeeds_only_when_exact_path_is_absent(self):
        result = self.run_cleanup(not_found=True)
        self.assert_unregister_order(result)
        self.assertEqual(result["code"], 0, result["stderr"])
        self.assertFalse(result["temporary_exists"])

    def test_not_found_with_remaining_registration_is_a_failure(self):
        result = self.run_cleanup(not_found=True, still_registered=True)
        self.assert_unregister_order(result)
        self.assertNotEqual(result["code"], 0)
        self.assertTrue(result["temporary_exists"])

    def test_failed_registry_read_cannot_prove_cleanup(self):
        result = self.run_cleanup(not_found=True, dump_failure=True)
        self.assert_unregister_order(result)
        self.assertNotEqual(result["code"], 0)
        self.assertTrue(result["temporary_exists"])

    def test_not_found_with_literal_backslash_registration_is_a_failure(self):
        result = self.run_cleanup(not_found=True, still_registered=True,
                                  path_name=r"fixtures\test")
        self.assert_unregister_order(result)
        self.assertNotEqual(result["code"], 0)
        self.assertTrue(result["temporary_exists"])


if __name__ == "__main__":
    unittest.main()
