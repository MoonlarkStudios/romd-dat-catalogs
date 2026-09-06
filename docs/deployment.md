# Signed catalog deployment

Status: synthetic publication is live at
https://moonlarkstudios.github.io/romd-dat-catalogs/. See the Release notes and
Actions runs for deployment evidence.
This environment publishes hand-authored synthetic DATs only. No upstream
mirroring permission or ROMD beta acceptance is implied.

## Development

Run `mise trust`, `mise install`, then `mise run check`. Go, actionlint, and
ShellCheck versions live in `.mise.toml`; CI uses the same configuration.
`mise run smoke` exercises the local unsigned publisher. `mise run build`
creates `bin/publisher` and `bin/distribution`. The TUF dependency and its
transitive libraries are pinned in `go.mod`/`go.sum`.

## Trust and layout

The implementation uses [go-tuf/v2](https://github.com/theupdateframework/go-tuf)
and its updater verification workflow. Clients start with a pinned public
`trust/1.root.json`, supplied independently of the download server. Never use
trust-on-first-use. Keep the client metadata cache for rollback detection.

Each role has a separate Ed25519 key with threshold one. The root private key
stays on the operator machine, outside CI. `online.json` contains only targets,
snapshot, and timestamp private keys. This initial single-operator trust setup
does not claim resilience to compromise of the offline root key. Recovery from
that compromise requires distributing a new trusted root out of band.

Root metadata expires after one year; targets after 30 days; snapshot after
seven days; timestamp after 48 hours. Expired metadata prevents new updates;
it does not remove already installed catalogs. The daily workflow refreshes
freshness metadata even when DAT bytes are unchanged. The RSS feed emits no
duplicate content-change event for unchanged documents.

Pages serves versioned TUF metadata, hash-prefixed `catalog.json` and `feed.xml`
targets, and a stable `/feed.xml` for readers. The plain RSS alias is a hint;
clients verify the TUF target before relying on its content. Signed catalog
indexes bind exact download URLs, sizes, and SHA-256 values for Release assets.
The unsigned `current.json` lives only inside the authenticated recovery archive;
it is not the public trust entry point.

Each Release contains the current/referenced DAT and metadata objects plus
`state.zip`. Only reachable publisher objects are included, so unreferenced
local history does not grow recovery archives indefinitely. Release state is
capped at 64 MiB expanded and compressed for this synthetic deployment. Referenced
objects are republished per Release; this is not the final incremental storage
design for complete upstream catalogs. Old Releases are retained. Pages switches
metadata sets as a deployment; an in-flight client holding a previous timestamp
may need to refresh after a deployment rather than assume old Pages paths persist.

## Initial setup

These initialization commands are for a new, independent deployment. The
MoonlarkStudios deployment already has a committed trust root; recover its
existing keys from private operational records instead of generating a replacement.
A repository transfer does not reset trust or metadata versions.

Initialize keys once in a private local directory; this command refuses to
overwrite an existing directory:

```sh
mise run build
./bin/distribution init --out .keys/synthetic-initial
mkdir -p trust
cp .keys/synthetic-initial/public/1.root.json trust/1.root.json
gh secret set TUF_ONLINE_KEYS < .keys/synthetic-initial/online.json
gh api --method POST repos/MoonlarkStudios/romd-dat-catalogs/pages -f build_type=workflow
```

Commit only the public root file. Keep `offline-root.json` and `online.json`
out of Git, logs, Pages artifacts, and Releases. Key files use mode 0600.
Back up the private directory securely before relying on this trust root.
GitHub secret values cannot be recovered by reading the secret back.

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

After the workflow and public root are merged, start the first deployment:

```sh
gh workflow run publish-synthetic.yml -f bootstrap=true
```

Bootstrap requires the public timestamp to return HTTP 404. Other errors fail
closed, and bootstrap cannot reset an existing publication. After the first
deployment and an ordinary restore/update run pass, enable the daily schedule:

```sh
gh workflow run publish-synthetic.yml
gh variable set SYNTHETIC_PUBLISH_ENABLED --body true
```

The workflow runs only on main and serializes scheduled/manual publication.
It uses standard Ubuntu runners, one-day Pages artifact retention, and no
Actions cache for DAT history. Code PRs never receive publishing secrets.
No production workflow is automatically enabled merely by merging code.

## Publication ordering and failures

1. Verify the previous Pages metadata using the pinned root and TUF updater.
2. Download and verify its recovery archive, then restore into a new directory.
3. Run the synthetic publisher and optional PSX acquisition; sign a private staging bundle using online keys.
4. Upload a new draft Release; publish it; download every asset and verify hashes.
5. Upload the Pages bundle and deploy it only after asset verification succeeds.
6. Verify the deployed signature chain and restore using a fresh client cache.

Every workflow attempt gets a distinct Release tag and monotonic metadata
version from the workflow run number and attempt. Never recreate or rename the
workflow to reset its run counter without planning a version migration. Staging
also rejects a version no greater than the authenticated prior publication.
No asset upload uses `--clobber`. A failed run can leave an orphan Release but
cannot advance Pages before asset checks. Retry with a new run/attempt.

If metadata expires during a prolonged outage, the regular restore deliberately
fails. Do not use bootstrap or disable expiry verification to recover. An
operator must recover the last independently verified publisher state from a
trusted local backup, refresh/rotate the root if required, and stage a higher
metadata version. A failed update is visible in Actions; installed ROMD catalogs remain usable.
Automated post-expiry recovery is deferred until observed failures justify it.

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

## Verification scope

`mise run check` includes localhost HTTP integration tests for signed target
download, wrong roots, modified targets/assets, expiry, rollback with cached
metadata, mixed snapshots, root rotation, state restoration, and staging failure
that leaves a simulated live publication intact. Tests use freshly generated
ephemeral keys. Workflow validation uses actionlint and ShellCheck.

These tests are not evidence of a completed GitHub Pages deployment, a full
upstream daily pack, unattended NAS acceptance, or ROMD application activation.

## Gated PlayStation publication

The existing `publish-synthetic.yml` workflow also supports the complete
`redump/psx/discs` catalog. Its filename, concurrency group, version counter,
and signing root deliberately remain unchanged. No second scheduler is needed.

`REDUMP_PSX_PUBLISH_ENABLED` defaults to absent/false. Do not set it until
current upstream redistribution conditions have been recorded. Merging the
workflow does not enable public Redump mirroring. Acquisition uses the explicit
HTTP Redump adapter, without redirect following or automatic retries, and
validates the expected PlayStation header and minimum catalog coverage.

After qualification, set the variable to `true` and manually dispatch the
existing workflow without bootstrap. The same opt-in enables its daily run.
Inspect the run summary for health, exact extracted-document hash, bytes, entry
counts, and last successful/check/change timestamps. An unchanged document
keeps its artifact identity and creates no new content-change RSS event.
Provider failures are signed as failed health while retaining the previous
artifact; they do not require the workflow to fail before it can report them.
A failure in restore, signing, upload, or deployment is an Actions failure.

Setting the PSX variable to `false` stops acquisition. On the next publication,
an existing PSX catalog is marked `publication_paused` with its last working
artifact retained; a never-enabled catalog is not added. Keep the existing
synthetic schedule enabled to continue refreshing signed metadata while paused,
or dispatch manually. Pausing does not revoke or delete previously public DATs.

This is a single-platform observation trial, not approval for broad coverage.
The current Release/recovery format still duplicates full artifacts and has a
64 MiB state limit. Before ongoing rollout, address artifact reuse and the
retention budget in issue #12. Record compressed size and actual change
frequency; unchanged probes are not evidence of a typical update interval.

Qualification status (2026-09-06): indexed official Redump overview text
supports public metadata reuse, but its current wiki page could not be fetched
(HTTP 404, HTTPS unavailable). Automation guidance and applicable distribution
conditions still need a durable source. No public PSX enable flag has been set.
Reference: http://wiki.redump.org/index.php?title=Redump.org
