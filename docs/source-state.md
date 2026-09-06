# Durable local source state

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
An operator recovery/migration command is not implemented in this slice. Do not
clear the in-flight marker or reset the directory without resolving the previous
run's retry and candidate disposition.

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

Ephemeral GitHub runners do not retain this directory. The signed public
`state.zip` does not currently include source scheduling state. Before connecting
CI polling, add its durable custody/restoration, interrupted-run recovery, and
candidate-publication recovery, and qualify upstream transport and redistribution.
The existing synthetic publication workflow is unchanged.
