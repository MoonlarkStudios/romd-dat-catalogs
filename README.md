# ROMD DAT catalogs

Companion publisher for ROMD's planned one-click No-Intro and Redump DAT
subscriptions. Complete upstream catalogs come first; future 1G1R filtering
belongs in ROMD's library layer.

**Current status: local, unsigned prototype using synthetic metadata.**
There are no upstream acquisition adapters, public DAT mirrors, signing keys,
automatic publication schedules, or ROMD integration yet. Checksums provide
integrity checks only. Do not enable production AutoApply against this output.

## Run

Python 3.11+ on macOS or Linux (POSIX file locking required):

```sh
python3 -m venv .venv
.venv/bin/pip install -r requirements.txt
.venv/bin/python -m unittest discover -s tests -v
.venv/bin/python publisher.py fixtures/manifest.json --output output
```

The example hostname is deliberately non-routable. For local serving tests,
the objects remain accessible under `output/`; HTTPS hosting is a later step.

The manifest is a trusted local registry of acquisition attempts. Each entry
contains a stable `catalogId`, exact expected DAT header name, public source
URL, and either a path to a DAT/single-DAT ZIP or a `failure` marker. Paths
are relative to the manifest. Never put credentials in URLs or manifests.
This prototype's limits are intentionally suitable for small fixtures, not
yet qualified for the entire upstream daily pack.

## What works

- Exact extracted-document hashes, preserving upstream bytes and notices.
- Bounded DAT/ZIP handling, safe XML parsing, expected-identity checks.
- Per-catalog failure state retaining the prior working artifact.
- Immutable content-addressed artifacts, RSS, and complete catalog snapshots.
- Stable feed event identities; an upstream restoration can reuse old content
  with a new event. Feed truncation does not limit index-based catch-up.
- Atomic replacement of `current.json` after its objects are written and
  verified, with a persistent OS lock serializing local publisher processes.
- Rejected-version exclusion in a small reference catch-up function.

`current.json` points to an exact snapshot by path, length, and SHA-256 and
explicitly declares `trust: unsigned-development`. The snapshot references
an immutable RSS file and every current catalog. RSS is a notification hint;
the complete index is the basis for reconciliation. No mutable standalone
`feed.xml` is produced in this prototype: follow the snapshot's feed reference.

The tests exercise local process interruption before pointer commit. They do
not prove host power-loss durability, remote object-store atomicity, signed
metadata, upstream authenticity, or ROMD activation safety. DAT checks here
are structural; full ROM/hash semantics and anomaly policies remain pending.

## Next acceptance gates

1. Define and implement authenticated metadata, trusted bootstrap, expiry,
   key rotation, compromise recovery, and rollback/freeze resistance using a
   reviewed signing design. The experimental format is not a stable contract.
2. Qualify acquisition and public redistribution conditions for each upstream.
   Preserve system/variant distinctions and explicit inclusion settings.
3. Add adapters, remote publication, a stable RSS URL, retention, and actual
   unattended change/outage observations. No GitHub API token required by readers.
4. Integrate with ROMD's durable jobs, CAS, exact-version diff, validated
   activation, curation preservation, history/rollback, and library convergence.

Only hand-authored synthetic DATs belong in current fixtures. No upstream
content is distributed by this repository. No-Intro and Redump remain the
authorities for their catalogs; this project is not affiliated with them.
