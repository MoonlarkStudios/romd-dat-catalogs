# Source state and explicit recovery

`redump.NewScheduler(stateDirectory)` adds persistent admission to the acquisition
library. It is not invoked by a CLI or workflow. Live upstream polling and public
mirroring remain disabled.

Call `Initialize(reviewedCatalogs)` once in a new directory, then reuse
`Run(ctx, reviewedCatalogs, newStagingDirectory)` across process restarts. An
existing directory cannot be initialized again. Missing/corrupt state is an
error, never an instruction to start over. Keep this directory on durable local
storage, for example an ignored `.source-state/redump` directory with its parent
created by the operator. It contains runtime state, not source credentials.

## Persisted contract

The versioned JSON state binds the acquisition origin and the complete reviewed
registry: catalog ID, source system, expected header, platform, representation,
policy version, and count floors. Catalog ordering is canonicalized, so reordering
an unchanged allowlist is harmless. Adding/removing entries or changing any
binding fails with `ErrRegistryChanged` before acquisition. A reviewed registry
migration remains a future operation; do not delete state to bypass this check.

The schedule records last admission, next admission, and whether the run is still
in flight. Normal polling has a 24-hour interval from admission. A later provider
Retry-After extends that deadline, including delays longer than a day. A new
scheduler process checks the saved deadline before creating acquisition staging
or accessing the network. A regressed local clock is rejected.

Admission is committed before the first request. Longer retry deadlines are
committed while the run is still in flight, before results are returned. After
acquisition completes, completion is committed before the caller receives its
publisher attempts. The caller still owns staging and the subsequent publication.

## Errors and recovery boundary

| Outcome | Meaning |
| --- | --- |
| `ErrNotDue` | Leave the working publication alone and wait until the persisted schedule permits another run. |
| `ErrBusy` | Another process owns the source lock; do not start another acquisition. |
| `ErrRegistryChanged` | Configuration differs from the persisted identity/policy bindings. |
| `ErrInterrupted` | A prior run admitted work without a committed completion; recovery must be resolved before further acquisition. |
| State read/validation/write error | Stop; do not create replacement state or publish partial results. |

An interrupted run is deliberately not automatically retried after its default
daily deadline: the process could have died after receiving a longer upstream
Retry-After but before recording it. Preserve state and staging for inspection.
Use the explicit recovery command below after resolving the previous run.
Registry migration remains unimplemented; do not reset state to bypass it.

Acquisition completion and publication are separate transactions. A crash after
completion but before publication does not guarantee replay of those candidates;
the old working publication remains the fallback. This is durable scheduling and
registry binding, not a transactional candidate outbox or full pipeline resume.

## Storage and verification

A persistent OS file lock serializes processes on macOS/Linux; lock files are
kept between owners. State uses a same-directory temporary file, file sync,
atomic rename, and directory sync. Directories use mode 0700 and files mode 0600.
The local state is trusted operator storage, not signed remote metadata.

Tests exercise restart cooldown, registry changes, daily admission, corrupt or
missing state, clock regression, a forced child-process exit, and contention
against a separate process. `mise run check` includes race detection. These tests
prove the tested process-level behavior, not NAS/filesystem power-loss durability.

## Explicit operator recovery

`mise run build` builds `bin/source-state`, with `init`, `inspect`, and `recover`
commands. It does not acquire or publish DATs. Initialize a reviewed catalog JSON
array once with `bin/source-state init --registry reviewed-catalogs.json`.
`bin/source-state inspect` returns the current digest, schedule, in-flight marker,
and recovery count. The default directory is `.source-state/redump`; override
it with `--state` consistently across commands.

For an interrupted run, first stop the old runner, inspect its staging and logs,
and establish the upstream retry window. Then use `recover` with all of:

- `--expected-digest` from a fresh inspection;
- `--not-before` as an RFC3339 date at or after both now and the saved deadline;
- `--confirm-stopped`, `--confirm-retry-window`, and `--discard-candidates`.

These flags record operator assertions; they do not terminate a runner, discover
an unrecorded Retry-After, or delete staged files. Discarded unpublished candidates
must not subsequently be published. Recovery preserves registry bindings and last
admission, records the decision and count, and clears the in-flight marker. It
does not fetch or publish. Stale reviews and shortened deadlines are rejected.
Version 1 state is read without changing its schedule and becomes version 2 on
its next successful write; version 2 includes recovery history's latest record
and cumulative count.

## Optional GitHub checkpoint custody

`--remote-repo owner/repo` selects authoritative GitHub Contents API storage using
`GITHUB_TOKEN` from the environment. Explicitly create the `source-state` branch
before initialization; the checkpoint path is `redump-state.json`. The token
needs repository contents write permission. Never include credentials in state.
Public repository state and commit history are public, so use only nonsensitive
catalog configuration and scheduling metadata.

Remote operations read the checkpoint afresh and condition writes on its Git blob
SHA. Missing state is an error except during explicit initialization. Admission
must receive a matching write receipt before upstream access. Conflicting or
ambiguous writes stop the operation; they are not automatically retried. An
accepted write whose response was lost may leave the run marked interrupted for
inspection. The local directory provides only a same-machine lock in this mode;
remote conditional writes arbitrate separate runners.

This is trusted operator state, not TUF-authenticated client distribution. It is
separate from the signed publication's `state.zip`. Tests use local TLS fixtures
and a conditional in-memory store for conflicts, ambiguous writes, retained retry
deadlines, and recovery; no live GitHub checkpoint deployment has been verified.

No acquisition CLI or polling workflow is enabled. The existing synthetic
publication workflow is unchanged. Further infrastructure expansion is paused:
next prove upstream acquisition, redistribution conditions, and complete coverage,
then one real platform's update, diff, review, and activation in ROMD. A failed
update may preserve the last working catalog and offer an explicit retry; full
candidate replay is not a prerequisite for that beta experience.
