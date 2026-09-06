# Redump acquisition library

`internal/redump` implements the first acquisition layer. No CLI or scheduled
workflow calls it, and no upstream DATs are published by this change. Tests
use synthetic data and local HTTP servers. The adapter explicitly targets
`http://redump.org`, matching the upstream-supported transport. It does not first
try HTTPS or perform a fallback. Publisher signatures authenticate the published
bytes; they do not authenticate the upstream HTTP connection.

## Contract

Construct one adapter with `redump.New()` and reuse it across batches. Supply
an explicit allowlist of at most 25 reviewed catalog definitions. Each records
catalog ID, Redump system slug, expected DAT header, platform, representation,
policy version, and minimum game/ROM counts. There is no remote discovery or
automatic enrollment of newly listed platforms.

`Acquire(ctx, catalogs, newStagingDirectory)` returns one result per catalog,
including the reviewed definition, exact extracted-document SHA-256, counts,
status code, retry time, and a `publisher.Attempt`. Successful attempts point
to the original ZIP/raw input in the staging directory. The existing publisher
revalidates that input and stores exact extracted DAT bytes. Failed attempts
let it retain the previous artifact and mark the catalog unhealthy.

The caller must keep staging until publication completes, then remove it.
Staging must not already exist; directories use mode 0700 and files mode 0600.
Configuration/filesystem failures return a batch error: do not publish a
partial result from such a call. Acquisition never changes publisher state.

## Network and validation behavior

- HTTP to the fixed Redump origin, no redirects, no credentials, and an
  identifying User-Agent. Transport injection is private to synthetic tests.
- One request per catalog with no automatic retries. Calls on the same adapter
  serialize with context-aware admission. Requests have a 30-second timeout
  and share a two-minute context budget per batch, including admission.
- Inputs are bounded at 16 MiB; extracted documents at 32 MiB through the
  publisher's existing ZIP/XML validator. Unexpected content encodings,
  truncated responses, malformed documents, wrong headers, empty catalogs,
  and count floors reject the candidate. HTTP 200 is not acceptance by itself.
- 429 and 503 stop remaining requests and preserve Retry-After as a date or
  delay. Missing/invalid values default to one minute. Cooldown persists across
  calls on that adapter. Other failures are isolated to their catalog.
- Same document bytes in raw or differently packaged ZIP downloads have the
  same hash. Publishing them produces no duplicate content event. Failure
  retains the previous artifact; a missing endpoint cannot delete a catalog.

Count floors are reviewed configuration, not an automatic relative-change
policy or completeness proof. The caller must approve appropriate thresholds.
Result identity metadata is returned to the caller but is not yet incorporated
into the signed publication schema. Diagnostic codes remain in acquisition
results; the existing publisher exposes its generic acquisition-failure code.

## Before live enablement

Establish redistribution conditions, then qualify coverage
and capacity for the first explicit platform. Connect that candidate to ROMD
review before expanding source coverage. See the [qualification follow-up](upstream-qualification.md)
for the current gates and Fresh1G1R takeaways.

The existing [source-state scheduler](source-state.md) can preserve retry
deadlines and registry bindings. Optional remote custody and interrupted-run
recovery are implemented but have not been deployed; their deployment or expansion
is not a prerequisite for the first operator-reviewed slice. Respect upstream
retry deadlines when manually retrying. Each process must share one adapter per
provider.
A new process using the adapter alone does not inherit its in-memory cooldown.
The scheduler persists admission separately; its integration and recovery
boundaries are documented in the linked guide.

The deterministic tests cover raw/ZIP acquisition, packaging-only changes,
publication failure retention, wrong identity, HTML/empty/truncated documents,
count floors, input/expanded limits, redirect rejection, 429/503 batch and
cross-call cooldown, cancellation, staging ownership, and catalog isolation.
Run `mise run check` and `mise run smoke`. These tests do not establish real
Redump availability, sustained throughput, or unattended publication readiness.


The HTTP transport decision supersedes the earlier HTTPS-only qualification gate.
Acquisition results record the HTTP source URL. Existing source-state snapshots
bind their origin and therefore reject an older HTTPS binding; no deployed real
source state is known. Do not rewrite a checkpoint to disguise that identity
change. Use a separate explicitly initialized HTTP state for a trial. No registry
migration machinery or scheduled real-source publication is added here.

## HTTP validation evidence

On 2026-09-06 UTC, `mise run check` passed (race tests across all seven Go
packages, vet, formatting, actionlint, and all three CLI builds). `mise run smoke`
passed with one synthetic catalog and zero failures. `git diff --check` passed.
The production-constructor test records the exact HTTP request and source URL;
local HTTP tests exercise unchanged/repackaged content, rejected candidates,
redirect rejection, backoff, and retention of the prior working artifact.

A disposable local harness used `redump.New()` against
`http://redump.org/datfile/psx/` and passed the result to the existing local
publisher. Acquisition plus local publication took 3.947 seconds in this single
run, with 10,914 disc entries and 60,168 ROM/track records. The extracted-document
SHA-256 was
`0d5cffb7feb15aa4297ccaf722c62d2b08f17eb859d6ab7890088d481bf8a18e`,
matching the earlier baseline. This proves one real HTTP acquisition through
the changed adapter, not a new upstream version or sustained reliability.
Temporary upstream data and the probe were removed; nothing was publicly
published. No ROMD runtime code or deployed demo was changed by this check.
