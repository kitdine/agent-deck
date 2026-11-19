#!/usr/bin/env python3
"""Partition the Darwin Go package list for race jobs."""

import argparse
import sys


def split_packages(packages: list[str]) -> tuple[list[str], list[str], list[str]]:
    if len(packages) != len(set(packages)):
        raise ValueError("the package list contains duplicates")

    cli = [package for package in packages if package.endswith("/cmd/agentdeck")]
    if len(cli) != 1:
        raise ValueError("expected exactly one cmd/agentdeck package")

    rest = sorted(package for package in packages if package != cli[0])
    if len(rest) < 2:
        raise ValueError("the library shards would be empty")

    return cli, rest[::2], rest[1::2]


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("shard", type=int, choices=range(3))
    args = parser.parse_args()
    try:
        shards = split_packages(sys.stdin.read().split())
    except ValueError as error:
        parser.error(str(error))
    print(" ".join(shards[args.shard]))


if __name__ == "__main__":
    main()
