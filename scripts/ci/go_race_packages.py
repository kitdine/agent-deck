#!/usr/bin/env python3
"""Partition the Darwin Go package list into balanced race shards."""

import argparse
import sys

SHARD_COUNT = 2

# Scheduling weights (approximate seconds under `go test -race -p 1` on macos-15,
# measured from Actions run 45183596099). Unknown packages default to weight 1.
PACKAGE_WEIGHTS: dict[str, int] = {
    "/cmd/agentdeck": 826,
    "/internal/backup": 716,
    "/internal/usage": 479,
    "/cmd/agentdeck-perf": 333,
    "/internal/quota": 245,
    "/internal/store": 220,
    "/internal/desktop": 169,
    "/internal/provider": 125,
    "/internal/scanruntime": 119,
    "/internal/telemetry": 61,
    "/internal/sessionorigin": 51,
    "/internal/doctor": 43,
    "/internal/extension": 23,
    "/internal/installation": 23,
    "/internal/session": 16,
    "/internal/platform": 14,
}


def package_weight(package: str) -> int:
    for suffix, weight in PACKAGE_WEIGHTS.items():
        if package.endswith(suffix):
            return weight
    return 1


def split_packages(
    packages: list[str], shard_count: int = SHARD_COUNT
) -> tuple[list[str], ...]:
    if shard_count < 1:
        raise ValueError("shard_count must be at least 1")
    if len(packages) != len(set(packages)):
        raise ValueError("the package list contains duplicates")

    cli = [package for package in packages if package.endswith("/cmd/agentdeck")]
    if len(cli) != 1:
        raise ValueError("expected exactly one cmd/agentdeck package")
    if len(packages) < shard_count:
        raise ValueError("not enough packages to populate every race shard")

    buckets: list[list[str]] = [[] for _ in range(shard_count)]
    totals = [0] * shard_count
    ordered = sorted(packages, key=lambda pkg: (-package_weight(pkg), pkg))
    for package in ordered:
        target = min(
            range(shard_count), key=lambda idx: (totals[idx], len(buckets[idx]), idx)
        )
        buckets[target].append(package)
        totals[target] += package_weight(package)

    return tuple(sorted(bucket) for bucket in buckets)


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("shard", type=int, choices=range(SHARD_COUNT))
    args = parser.parse_args()
    try:
        shards = split_packages(sys.stdin.read().split())
    except ValueError as error:
        parser.error(str(error))
    print(" ".join(shards[args.shard]))


if __name__ == "__main__":
    main()
