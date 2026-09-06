# ROMD DAT catalogs

Go companion publisher; independent of ROMD application runtime. The unsigned
publisher uses the standard library; signed distribution uses pinned go-tuf/v2.
Supports macOS and Linux. Mise owns Go and development-tool versions.

- Run `mise install`, then `mise run check` (race tests, vet, formatting,
  workflow validation, and builds). Use `mise exec -- gofmt` for Go files.
  Tests are offline and use synthetic DATs; `fixtures/python-snapshot` is a
  compatibility fixture, not a Python runtime dependency.
- Preserve exact DAT bytes and complete catalogs; no 1G1R filtering here.
- Never treat checksums or unsigned experimental metadata as authentication.
- Publish artifacts before references; retain working artifacts on acquisition
  failures. Keep publication lock files rather than unlinking between owners.
- Do not commit upstream DATs or enable public mirroring until redistribution
  conditions have been established. Do not bundle credentials.
- Report prototype coverage separately from production and ROMD acceptance.
- Tests include localhost HTTP fixtures. A sandbox denial binding a test port
  needs scoped escalation, not weakened tests. TUF fetcher max tries is 1;
  zero means unlimited attempts and can hang missing-metadata tests.
- Root private keys stay offline from CI. Never commit `.keys/` or print keys.
  Only online role keys go in `TUF_ONLINE_KEYS`; read `docs/deployment.md`.
- Do not spawn agents unless the user explicitly requests them.
