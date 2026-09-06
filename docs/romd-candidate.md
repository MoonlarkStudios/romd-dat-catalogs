# One PlayStation candidate into ROMD review

This increment exposes the existing acquisition/signing/client layers for an
operator-reviewed trial. It does not enable scheduled upstream publication.

```sh
mise run check
./bin/publisher --output psx-state redump-psx
./bin/distribution stage --state psx-state --trust trust --keys ONLINE_KEYS_FILE \
  --out psx-bundle --version NEXT_VERSION \
  --release-base https://github.com/OWNER/REPO/releases/download/IMMUTABLE_TAG
./bin/distribution candidate --root INDEPENDENT_PUBLIC_ROOT \
  --bundle psx-bundle --cache reader-cache --catalog redump/psx/discs \
  --name 'Sony - PlayStation' --out candidate
```

Use separate disposable signing keys and a matching independently supplied root
for local trials. Do not use production keys merely to test a private bundle.
No command above uploads assets. Public mirroring retains its separate reuse
qualification gate. Redump acquisition explicitly uses its HTTP endpoint, one
complete public PSX disc document, reviewed minimum counts (10,000 entries /
50,000 tracks), and the existing bounded parser and last-working-state behavior.
This is not BIOS, restricted-source, or all-Redump coverage.

For real HTTPS publication, use `--site HTTPS_URL` instead of `--bundle`.
`candidate` requires a persistent publisher/root-specific TUF cache; serialize
calls sharing it. The command owns a nonblocking OS lock for its entire lifetime,
so restarting its caller cannot overlap a still-running cache writer. ROMD also
serializes its own requests. Keep the lock inode between owners. Keep that cache across
updates; never reset it to bypass expiry or rollback failures. The root is never
fetched on first use. The complete signed index, not RSS, selects the exact
catalog ID and expected header. Unhealthy/missing catalogs, inconsistent signed
artifact references, changed bytes, and invalid documents fail without emitting
a usable candidate directory.

Success writes `candidate.dat` (exact extracted document bytes) and
`candidate.json` (identity, document hash/size, publication version, source URL)
into a new private directory. Existing output directories are never overwritten.
ROMD repeats semantic validation and compares against its active document before
approval. The command itself cannot activate a ROMD catalog.

The local bundle mode uses the same TUF updater and a file-backed transport.
Only staged metadata, targets, and release assets are resolved; it never reaches
the network. Bundle presence does not grant trust. The independently supplied
root, signature chain, expiry, cached versions, and document bindings still apply.

## Storage observation and expansion gates

The real PSX document observed on 2026-09-06 UTC was 12,756,749 bytes; deterministic
gzip level 9 produced 3,973,980 bytes (about 69% smaller). Repeated probes returned
SHA-256 `0d5cffb7feb15aa4297ccaf722c62d2b08f17eb859d6ab7890088d481bf8a18e`.
This measures one document, not actual change frequency. One demo addition was
explicitly synthetic and must not be counted as an observed upstream update.

The current release publisher still republishes referenced full artifacts and a
recovery archive. Keep that prototype limitation out of broad source deployment.
Before expanding coverage:

- Reuse compressed immutable artifacts across publications. Identical extracted
  hashes deduplicate content; changed documents still need full artifacts.
- Budget retained full history: current and previous plus a modest recent window.
- Keep lightweight hashes, dates, versions, and change summaries longer term.
- Delete expired artifacts separately, only after checking references from every
  retained index. Installed ROMD catalogs must not depend on publisher retention.

No retention engine, delta patches, registry migration, or remote scheduling
expansion is added by this slice. Existing Go checks and smoke passed, including
new candidate binding/tampering/root/CLI tests. See the ROMD trial documentation
for application activation evidence; library tests alone do not establish it.
