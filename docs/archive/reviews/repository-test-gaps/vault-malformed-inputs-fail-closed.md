---
status: historical
retired: 2026-07-26
plan: repository-test-gaps
task: vault-malformed-inputs-fail-closed
---

# Review log — repository-test-gaps / vault-malformed-inputs-fail-closed

## Round 1 — 2026-07-23

- Reviewed state:
  `18fc330cc142dd7ccafb71d643b845c52feb692a17b0f5a7267b36d6fee62b50`
  (staged content manifest)
- Reviewer: `review_vault_malformed_r1` (`gpt-5.6-sol`, high)
- Scope: `internal/credentialvault/vault_test.go`; malformed payload and key
  framing, public sealing entrypoints, exact sentinel errors, byte preservation,
  and synthetic-data safety
- Findings:
  - [high] A one-byte ciphertext case did not protect empty ciphertext from
    being accepted as empty plaintext. Add a zero-length authenticated-metadata
    case with `ErrCiphertextInvalid` and unchanged key bytes.
  - [medium] Short, bad-magic, and bad-version key fixtures did not reject an
    oversized framed key. Add a trailing-byte case with exact preservation.
  - [medium] Only `Seal` protected empty references. Add the corresponding
    isolated `SealExisting` case before key creation.
- Evidence: focused malformed-payload and malformed-key tests PASS; complete
  `internal/credentialvault` package PASS; exact staged path and manifest
  verified; no production changes or non-synthetic secret values
- Verdict: REOPEN

## Round 2 — 2026-07-23

- Reviewed state:
  `b1390e77e93b6c54ce85d1aa6b0b03735f5110f608e9a7e8da0381e0be2ba4b4`
  (staged content manifest)
- Reviewer: `review_vault_malformed_r2` (`gpt-5.6-sol`, high)
- Scope: Round 1 closure plus the complete
  `internal/credentialvault/vault_test.go` candidate
- Findings: none
- Prior finding closure:
  - Zero-length ciphertext now requires `ErrCiphertextInvalid`, no returned
    value, and unchanged key bytes.
  - Oversized key framing now requires `ErrKeyVersionUnsupported` and exact
    key-file preservation.
  - `SealExisting` now rejects an empty reference before missing-key handling
    and creates no key.
- Evidence: focused malformed tests PASS including `-count=10`; complete
  `internal/credentialvault` package PASS; exact staged path and manifest
  verified; fixtures remain synthetic and secret-free
- Source task commit:
  `0921616ea9765bcea3d2b3d92bbf533268214e5b`
- Audit integrated commit:
  `cc0e276cfdcc41e738e1a27c844f692a610dfbe2`
- Signature and manifest: SSH signature GOOD; source and audit manifests both
  `b1390e77e93b6c54ce85d1aa6b0b03735f5110f608e9a7e8da0381e0be2ba4b4`
- Verdict: PASS
