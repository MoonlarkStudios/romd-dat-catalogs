# One PlayStation candidate into ROMD review

The candidate reader supports Git-backed signed format 2 and the existing
format-1 local demo bundles. Rebuild older readers before switching the public
index; the JSON contract consumed by ROMD is unchanged. Public acquisition
remains gated by upstream qualification.

The following legacy bundle commands are for private local demos only. Public
publishing uses the Git-backed workflow in [deployment.md](deployment.md).

```sh
mise run check
./bin/publisher --output psx-state --catalog redump/psx/discs
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

The public storage model is now complete, uncompressed DATs at stable platform
paths in a separate Git repository. Only extracted-document changes produce
commits. Signed indexes bind full commit URLs and document hashes; Pages carries
the index and latest-only RSS. No routine Release assets or recovery archives
are produced. Historical browsing and publisher rollback are not beta goals.
Git retains history without trimming; measure repository growth and real update
frequency before adding storage machinery. Never rewrite history while signed
indexes may reference those commits. ROMD retains its active document locally.

Existing local Release-style bundles remain supported for the private demo;
this does not make them the public storage model. No additional scheduling,
checkpointing, or recovery framework is introduced.
