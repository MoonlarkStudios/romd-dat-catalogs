# Signed catalog deployment

The companion repository contains Go tooling, workflows, fixtures, and platform
identities. A separate public data repository (proposed name
`MoonlarkStudios/romd-dat-data`) contains complete, uncompressed DAT documents at
stable paths, for example `redump/psx/discs.dat`. Git retains older versions.
There is no automated trimming or history rewriting.

Status: the currently deployed site at
https://moonlarkstudios.github.io/romd-dat-catalogs/ is the signed synthetic
prototype. The pending workflow uses Git-backed data; it has not been deployed.
No public upstream mirror is enabled. Establish redistribution conditions before
enabling PSX. This change does not qualify other Redump platforms or No-Intro.

## Development and verification

Run `mise trust`, `mise install`, then `mise run check` (race tests, vet,
formatting, actionlint/ShellCheck, and builds). Git must also be available;
new lifecycle tests exercise real local Git commits without network access.
Go and validation tool versions are pinned in `.mise.toml`. `mise run smoke`
exercises the unsigned local publisher. No Python runtime is needed.

## Data and trust contracts

Only changed extracted DAT bytes produce a data commit. ZIP repackaging, check
timestamps, signed metadata renewal, and failed acquisition produce no DAT
change. The workflow exports only current documents, compares the Git index,
and commits/pushes only if that index changed. Store DATs without Git LFS or
text conversion (`*.dat -text -filter` in the data repository's `.gitattributes`).
Git's own internal object compression is independent of the raw documents served
to clients.

Signed catalog format `romd-signed-catalog-2` binds every current document's
length and SHA-256 to a raw GitHub URL containing a full 40-character commit ID:
`https://raw.githubusercontent.com/OWNER/DATA_REPO/COMMIT/redump/psx/discs.dat`.
Branch URLs, short commits, query strings, and fragments are rejected. The data
commit is pushed and all referenced raw documents are downloaded/hash-verified
before Pages advances. Never force-push or rewrite data history: that could
break URLs in previously signed indexes. No historical browsing or publisher
rollback service is a beta requirement. ROMD owns its installed active document
locally and validates/reviews each update before activation.

Pages publishes the signed index and RSS using the existing pinned trust root
and [go-tuf/v2](https://github.com/theupdateframework/go-tuf). Clients verify the
root chain, signatures, expiry, cached versions, and exact document hash/size.
The root is supplied independently; never use trust-on-first-use. Preserve the
client cache for rollback detection. Old `ae48371` candidate readers understand
only format 1 and must be rebuilt from this Git-format implementation before
cutover. The `candidate.json` contract consumed by ROMD does not change. Older
readers fail closed; they do not activate an unverified update.

Root metadata expires after one year; targets after 30 days; snapshot after
seven days; timestamp after 48 hours. Daily runs renew signatures even when DAT
bytes are unchanged. Latest-only RSS keeps one content-change event per catalog
with a stable GUID across unchanged checks. RSS is a notification hint; clients
reconcile against the verified full index. The plain `/feed.xml` is not a trust
entry point. Historical feed browsing is not provided.

## Configure the Git-backed workflow

The existing `publish-synthetic.yml` filename, concurrency group, and run-number
version counter remain intact. Do not recreate it or bootstrap to reset trust.
The initial migration can read the existing signed format-1 recovery archive;
subsequent runs reconstruct disposable local state using only format-2 signed
metadata and latest DATs. Routine runs create no Releases or recovery archives.
Existing Releases are left intact for old references and the initial migration.
The legacy `stage --release-base` path remains only for existing local demo
bundles; it is not called by the publishing workflow.

Before merging/deploying the cutover:

1. Initialize the separate public data repository with a default branch and
   `.gitattributes` containing `*.dat -text -filter`. Do not add upstream DATs
   until redistribution is qualified. Synthetic fixtures can test deployment.
2. Set `DAT_DATA_REPOSITORY` to its `OWNER/REPO` name. Configure `DAT_DATA_TOKEN`
   as a fine-grained credential with Contents read/write scoped only to that
   data repository. The normal `GITHUB_TOKEN` cannot push to a different repo.
   No data token is needed by public ROMD readers. Never log the token.
3. Preserve existing `TUF_ONLINE_KEYS` and the committed public root chain.
   Rebuild deployed candidate readers for signed format 2.
4. Manually dispatch `publish-synthetic.yml` without bootstrap and verify its
   deployed signature chain, downloaded DAT hashes, and restored latest state.
5. Set `DAT_PUBLISH_ENABLED=true` for the daily schedule. The old
   `SYNTHETIC_PUBLISH_ENABLED` variable is no longer consulted. An unset data
   repository or disabled schedule means no daily metadata renewal; complete
   configuration before the existing 48-hour timestamp expires.
6. Only after qualification, set `REDUMP_PSX_PUBLISH_ENABLED=true` to acquire
   the complete PSX catalog over its explicitly supported HTTP endpoint.
   Without this flag, no real-source acquisition occurs.

Every run checks in the same Go/toolchain environment, restores authenticated
latest state, acquires/validates into staging, exports DATs, conditionally commits
and pushes data, signs the index/feed, verifies public commit URLs, and deploys
Pages. A final check verifies the deployed exact metadata version and restores
latest state. A competing data push fails normally; retry the workflow. Never
force-push or introduce remote checkpoints to resolve it.

The data checkout includes history so `git count-objects -vH` in the run summary
can expose stored-object growth. This is a measurement, not a storage budget or
a precise GitHub quota reading. Also inspect current document sizes and actual
change frequency before broader coverage. No cleanup engine, bounded history,
or incremental patch format is introduced. Issue #12 tracks growth measurement.

## Publication failure and pause

Provider failures are signed as failed health while keeping the last document
and its successful/change timestamps. Installed ROMD catalogs remain usable;
new update checks show a failure and offer retry. A paused existing PSX catalog
is reported as `publication_paused`, without deleting its DAT. A never-enabled
catalog is not invented. Keep ordinary metadata publication enabled while
pausing acquisition. Previously public DATs remain in Git history.

Failures during restoration, signing, data push, public hash verification, or
Pages deployment are visible Actions failures. A data commit made before a
later failure is harmless: retry exports the same bytes and reuses that commit.
Pages never advances before public documents are verified. Metadata versions
use `GITHUB_RUN_NUMBER * 1000 + GITHUB_RUN_ATTEMPT` and must exceed the restored
signed version. Retry with a new run/attempt, without resetting the counter.

If metadata expires during an outage, restoration deliberately fails closed.
Do not bootstrap or disable expiry checks. An operator must use a previously
verified local state, refresh/rotate the root if necessary, and stage a higher
version. Automatic outage recovery is deferred until demonstrated need.

### Private key custody

Back up private signing keys in an access-controlled secrets manager and verify
that restored values exactly match the original files before publication. Keep
account names, vaults, item identifiers, recovery access, and operator-specific
backup locations in private operational records, outside this repository.

Preserve the exact JSON representation of each versioned public root and its
root and online key sets. Record root versions, expiry dates, and public-root
SHA-256 fingerprints with the private backup. These are TUF Ed25519 keys, not
SSH keys. Retain previous versions during rotation.

The root private key is for offline rotation only and must never be available
to CI. Only `online.json` supplies the `TUF_ONLINE_KEYS` Actions secret. Transfer
secret values through stdin or restricted private files, never command-line
arguments, chat, logs, or committed templates. CI must not have access to the
root-key backup.


## Key rotation

Prepare a new key set locally using the old root private key:

```sh
./bin/distribution rotate --previous-root trust/1.root.json \
  --offline-key .keys/synthetic-initial/offline-root.json \
  --out .keys/synthetic-rotation-2
cp .keys/synthetic-rotation-2/public/2.root.json trust/2.root.json
```

The new root is signed by both the old and new root keys and replaces all online
role keys. Keep every versioned public root so an old client can traverse the
chain. Pause scheduled publishing, merge the new public root, update
`TUF_ONLINE_KEYS` from the new `online.json`, run and verify a manual publication,
then resume scheduling. Substitute the latest root/version for later rotations.
Never place the offline root key in CI. Old online keys are rejected by staging
after rotation; this is exercised by tests with an old pinned client root.


## Validation scope

Local tests exercise format-1 to format-2 migration with the same TUF cache,
real local Git commits, unchanged ZIP repackaging, failed acquisition retention,
changed documents, older immutable URLs after branch advancement, latest-only
restoration, and immutable URL restrictions. Existing signature, wrong-root,
expiry, rollback, tampering, and candidate-binding tests remain.
These are not evidence of live GitHub deployment, public Redump redistribution,
or a new ROMD activation/hardware test.

Qualification status (2026-09-06): indexed official Redump overview text supports
public metadata reuse, but a current copy could not be fetched. Applicable
redistribution conditions still need to be established before enabling mirroring.
Reference: http://wiki.redump.org/index.php?title=Redump.org
