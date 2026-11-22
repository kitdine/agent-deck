import unittest

from go_race_packages import split_packages


class GoRacePackagesTest(unittest.TestCase):
    def test_partitions_all_packages_once_and_isolates_the_cli(self):
        cli = "example.test/agentdeck/cmd/agentdeck"
        libraries = [
            "example.test/agentdeck/internal/store",
            "example.test/agentdeck/cmd/agentdeck-perf",
            "example.test/agentdeck/internal/activity",
            "example.test/agentdeck/internal/usage",
            "example.test/agentdeck/internal/quota",
        ]
        packages = [*libraries, cli]
        shards = split_packages(packages)
        self.assertEqual(shards[0], [cli])
        self.assertCountEqual([item for shard in shards for item in shard], packages)
        self.assertLessEqual(abs(len(shards[1]) - len(shards[2])), 1)
        self.assertEqual(split_packages(list(reversed(packages))), shards)

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
