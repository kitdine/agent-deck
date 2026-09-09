# Snapshot performance contract fixture

`synthetic-snapshot.json` is the complete, producer-generated reference for the
fixed synthetic Codex and Claude corpus in
`snapshot_performance_contract_test.go`. The test freezes business time at
`2026-08-13T10:00:00Z` and uses UTC. It compares the complete serialized
snapshot, chunk ordering and digest, internal fields omitted from JSON, and the
relevant usage/session logical tables across cold, unchanged, and changed-input
cycles.

Regenerate the reference only for an approved contract change:

```sh
AGENTDECK_UPDATE_SNAPSHOT_PERFORMANCE_FIXTURE=1 \
  scripts/run-go-test.sh ./cmd/agentdeck \
  -run '^TestSnapshotPerformanceContractSyntheticCorpus$'
```

Representative source data stays outside the repository. Run the measurement
harness only against an isolated corpus copy and write its report to a private
path:

```sh
AGENTDECK_SNAPSHOT_PERFORMANCE_CORPUS=/private/path/to/corpus \
AGENTDECK_SNAPSHOT_PERFORMANCE_REPORT=/private/path/to/report.json \
AGENTDECK_SNAPSHOT_PERFORMANCE_SAMPLES=1 \
AGENTDECK_SNAPSHOT_PERFORMANCE_DEADLINE=5m \
  scripts/run-go-test.sh ./cmd/agentdeck \
  -run '^TestSnapshotPerformanceRepresentativeCorpus$'
```

Each measured sample launches the real `refresh-indexes` and streamed `snapshot`
CLI command paths in two fresh helper processes. The end-to-end wall and CPU
metrics include both command initializations, helper processes and stream decode;
logical-row verification runs afterward and is timed separately. One deadline
bounds the whole sample and terminates unresponsive helpers.

The report records every sample, corpus digest and scale, hardware/toolchain,
fixed time/timezone, process startup, wall and CPU time, peak RSS, domain stage
times, verification time, output and logical-row digests, deadline, cache/load
status, and explicit failure stage. Timeout, non-zero, partial and missing-result
samples are written before the test reports failure. A complete over-target
sample remains `within_target: false`.
