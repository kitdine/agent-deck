# Desktop wire fixtures v1

These canonical response envelopes are shared by Go contract tests and Swift
decoding verification. They contain only synthetic, privacy-bounded values.
The macOS application foundation must reuse these files when it moves the
verified `Codable` and `Sendable` v1 types into `AgentDeckShared`.

Run the standalone Foundation verifier without creating an Xcode project:

```bash
swift desktop/fixtures/v1/verify.swift \
  desktop/fixtures/v1/snapshot-complete.json \
  desktop/fixtures/v1/snapshot-partial.json
```
