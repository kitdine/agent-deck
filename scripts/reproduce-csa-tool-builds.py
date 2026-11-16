#!/usr/bin/env python3
"""Reproduce the four Task 8 measurement tools without changing source files.

Run at the recorded HEAD with the Task 8 candidate applied, using Go 1.27.3.
Only the supplied output directory and Go caches are written. No measurements
or installed applications are started. A mismatch fails instead of certifying
an approximate source state. The JSON manifest is intended for CEv1 ingestion.
"""

import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess


BASE = "27bbe5f65483ff18e4e40ca85518b4a8cf42d3e4"
INPUTS = {
    "run.go": "3ad906cba356df6cb45af3a6e1589a9681ed4b64c81db7b2c0db697d428fc8ff",
    "compare.go": "3cf6f40250f016b50e3a87ee20d58c0937411fff8eed9e5df09f2bc95ff6d5cb",
    "stats.go": "46c803c48ff384cfc504e6e02956f025cbaf9041458ed0117c3e9fcba2c135b8",
}
EXPECTED = {
    "schema33": "021fa1b4a49f6f0a8694bb12b94cc9bcfca8a233304891ba8e6dcf3de8ee82cd",
    "layout": "02d2ed597e45a4dfb2bb95d2d4a37756ac2194f0425c531f5336f48be69768f4",
    "analysis": "b09e2ed1c058c7d92e60276249ad43de57ae76e333235d8906008b7abb00e42f",
    "sync": "593059bee9c7d042e23bfddfc5aa7b47a4b515edbdd1aa94b0a4208dce4c930f",
}


def sha(data):
    return hashlib.sha256(data).hexdigest()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--go", required=True, help="Go 1.27.3 executable")
    parser.add_argument("--out", required=True, help="new diagnostic directory")
    args = parser.parse_args()
    root = Path(__file__).resolve().parents[1]
    out = Path(args.out).resolve()
    out.mkdir(mode=0o700, parents=True, exist_ok=False)
    env = dict(os.environ, GOCACHE="/private/tmp/agent-deck-go-build",
               GOMODCACHE="/private/tmp/agent-deck-go-mod")

    def run(*command):
        return subprocess.check_output(command, cwd=root, env=env)

    assert run("git", "rev-parse", "HEAD").decode().strip() == BASE
    assert run(args.go, "version").decode().strip() == "go version go1.27.3 darwin/amd64"
    source = root / "cmd/agentdeck-perf"
    for name, expected in INPUTS.items():
        assert sha((source / name).read_bytes()) == expected, name
    current = (source / "run.go").read_text()
    presync = current
    diff = run("git", "diff", "--no-ext-diff", "--unified=3", BASE,
               "--", "cmd/agentdeck-perf/run.go").decode()
    reversed_hunks = []
    for hunk in re.split(r"(?m)^@@ ", diff)[1:]:
        header, body = hunk.split("\n", 1)
        old_line = int(re.match(r"-(\d+)", header)[1])
        if old_line not in (846, 1039, 1051):
            continue
        lines = body.splitlines()
        before = "\n".join(line[1:] for line in lines if line.startswith((" ", "-"))) + "\n"
        after = "\n".join(line[1:] for line in lines if line.startswith((" ", "+"))) + "\n"
        assert presync.count(after) == 1, header
        presync = presync.replace(after, before)
        reversed_hunks.append(old_line)
    assert reversed_hunks == [846, 1039, 1051]
    start = presync.index('\tif raw, err := os.ReadFile(filepath.Join(dir, "invalid.json"))',
                          presync.index("func checkUnitOutcome"))
    end = presync.index("\tfiles, err := readSample(dir)", start)
    prelayout = presync[:start] + presync[end:]
    old_compare = run("git", "show", f"{BASE}:cmd/agentdeck-perf/compare.go")
    variants = {
        "schema33": {"run.go": prelayout.encode(), "compare.go": old_compare},
        "layout": {"run.go": presync.encode(), "compare.go": old_compare},
        "analysis": {"run.go": presync.encode()},
        "sync": {},
    }
    raw = run(args.go, "list", "-mod=vendor", "-deps", "-json", "./cmd/agentdeck-perf").decode()
    decoder = json.JSONDecoder()
    packages = []
    while raw.strip():
        obj, end = decoder.raw_decode(raw.lstrip())
        packages.append(obj)
        raw = raw.lstrip()[end:]
    goroot = Path(run(args.go, "env", "GOROOT").decode().strip())
    file_fields = ("GoFiles", "CgoFiles", "CFiles", "CXXFiles", "MFiles", "HFiles",
                   "FFiles", "SFiles", "SwigFiles", "SwigCXXFiles", "SysoFiles", "EmbedFiles")
    selected = {}
    for package in packages:
        for field in file_fields:
            for name in package.get(field, []):
                path = Path(package["Dir"]) / name
                key = ("GOROOT/" + str(path.relative_to(goroot)) if path.is_relative_to(goroot)
                       else str(path.relative_to(root)))
                selected[key] = path
    for name in ("go.mod", "go.sum", "vendor/modules.txt"):
        selected[name] = root / name
    manifest = {"base_commit": BASE, "recipe_sha256": sha(Path(__file__).read_bytes()),
                "source_scope": "go list -mod=vendor -deps selected compilation and embed files, plus go.mod/go.sum/vendor/modules.txt; tests excluded",
                "digest_recipe": "SHA256(sorted UTF-8 path:sha256 lines joined by LF, no final LF)",
                "build_command": "go build -mod=vendor -trimpath [-overlay=<version.json>] -o <output> ./cmd/agentdeck-perf",
                "go_env": json.loads(run(args.go, "env", "-json", "GOOS", "GOARCH", "GOAMD64",
                                         "CGO_ENABLED", "GOFLAGS", "GOEXPERIMENT", "GOTOOLCHAIN",
                                         "CC", "CXX", "CGO_CFLAGS", "CGO_CPPFLAGS", "CGO_CXXFLAGS",
                                         "CGO_LDFLAGS")),
                "toolchain_binaries": {name: sha((goroot / name).read_bytes()) for name in
                                       ("bin/go", "pkg/tool/darwin_amd64/compile", "pkg/tool/darwin_amd64/link")},
                "versions": {}}
    for version, replacements in variants.items():
        mapping = {}
        for name, content in replacements.items():
            path = out / f"{version}-{name}"
            path.write_bytes(content)
            mapping[str(source / name)] = str(path)
        overlay = out / f"{version}.json"
        overlay.write_text(json.dumps({"Replace": mapping}))
        binary = out / version
        command = [args.go, "build", "-mod=vendor", "-trimpath"]
        if mapping:
            command.append(f"-overlay={overlay}")
        run(*command, "-o", str(binary), "./cmd/agentdeck-perf")
        actual = sha(binary.read_bytes())
        assert actual == EXPECTED[version], (version, actual, EXPECTED[version])
        files = {key: sha(Path(mapping.get(str(path), path)).read_bytes())
                 for key, path in sorted(selected.items())}
        digest = sha("\n".join(f"{key}:{value}" for key, value in files.items()).encode())
        manifest["versions"][version] = {"binary_sha256": actual, "exact_binary_match": True,
                                         "source_digest": digest, "source_files": files,
                                         "build_info": run(args.go, "version", "-m", str(binary)).decode().splitlines()[1:]}
        print(version, actual, digest, flush=True)
    (out / "manifest.json").write_text(json.dumps(manifest, indent=2) + "\n")


if __name__ == "__main__":
    main()
