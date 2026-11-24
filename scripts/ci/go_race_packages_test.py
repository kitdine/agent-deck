from pathlib import Path
import subprocess
import sys
import unittest

sys.path.insert(0, str(Path(__file__).resolve().parent))
from go_race_packages import package_weight, split_packages


class GoRacePackagesTest(unittest.TestCase):
    def test_partitions_all_packages_once_into_two_balanced_shards(self):
        packages = [
            "example.test/agentdeck/cmd/agentdeck",
            "example.test/agentdeck/cmd/agentdeck-perf",
            "example.test/agentdeck/internal/activity",
            "example.test/agentdeck/internal/backup",
            "example.test/agentdeck/internal/desktop",
            "example.test/agentdeck/internal/doctor",
            "example.test/agentdeck/internal/extension",
            "example.test/agentdeck/internal/installation",
            "example.test/agentdeck/internal/platform",
            "example.test/agentdeck/internal/provider",
            "example.test/agentdeck/internal/quota",
            "example.test/agentdeck/internal/scanruntime",
            "example.test/agentdeck/internal/session",
            "example.test/agentdeck/internal/sessionorigin",
            "example.test/agentdeck/internal/store",
            "example.test/agentdeck/internal/telemetry",
            "example.test/agentdeck/internal/usage",
            "example.test/agentdeck/internal/watch",
        ]
        shards = split_packages(packages)
        self.assertEqual(len(shards), 2)
        self.assertCountEqual([item for shard in shards for item in shard], packages)
        for shard in shards:
            self.assertEqual(shard, sorted(shard))
            self.assertTrue(shard)
        self.assertEqual(split_packages(list(reversed(packages))), shards)

        cli = "example.test/agentdeck/cmd/agentdeck"
        backup = "example.test/agentdeck/internal/backup"
        self.assertNotEqual(cli in shards[0], backup in shards[0])

        weights = [sum(package_weight(pkg) for pkg in shard) for shard in shards]
        total = sum(weights)
        self.assertLessEqual(abs(weights[0] - weights[1]) / total, 0.1)

    def test_refuses_missing_or_ambiguous_coverage(self):
        cli = "example.test/agentdeck/cmd/agentdeck"
        for packages in [
            ["example.test/agentdeck/internal/store"],
            [cli],
            [cli, cli, "example.test/agentdeck/internal/usage"],
        ]:
            with self.subTest(packages=packages):
                with self.assertRaises(ValueError):
                    split_packages(packages)

    def test_cli_outputs_both_shards_and_rejects_third_shard(self):
        script = Path(__file__).with_name("go_race_packages.py")
        packages = [
            "example.test/agentdeck/cmd/agentdeck",
            "example.test/agentdeck/internal/backup",
            "example.test/agentdeck/internal/store",
        ]
        combined = []
        for shard in ("0", "1"):
            result = subprocess.run(
                [sys.executable, str(script), shard],
                input="\n".join(packages) + "\n",
                text=True,
                capture_output=True,
                check=True,
            )
            combined.extend(result.stdout.split())
        self.assertCountEqual(combined, packages)
        rejected = subprocess.run(
            [sys.executable, str(script), "2"],
            input="\n".join(packages) + "\n",
            text=True,
            capture_output=True,
        )
        self.assertNotEqual(rejected.returncode, 0)


if __name__ == "__main__":
    unittest.main()
