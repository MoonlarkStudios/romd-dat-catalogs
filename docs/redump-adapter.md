# Redump acquisition library

`internal/redump` implements the first acquisition layer. No CLI or scheduled
workflow calls it, and no upstream DATs are published by this change. Tests
use synthetic data and local TLS servers. The real HTTPS endpoint was not
qualified by the earlier source probe; the adapter does not fall back to HTTP.

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

- HTTPS to Redump, no redirects, no HTTP fallback, no credentials, and an
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

Establish upstream transport and redistribution conditions; qualify full-source
coverage and capacity; use the [source-state scheduler](source-state.md) for
retry deadlines and registry bindings; and integrate review of suspicious changes.
Optional remote custody and explicit interrupted-run recovery are implemented
but have not been deployed. Each process must share
one adapter per provider.
A new process using the adapter alone does not inherit its in-memory cooldown.
The scheduler persists admission separately; its integration and recovery
boundaries are documented in the linked guide.

The deterministic tests cover raw/ZIP acquisition, packaging-only changes,
publication failure retention, wrong identity, HTML/empty/truncated documents,
count floors, input/expanded limits, redirect rejection, 429/503 batch and
cross-call cooldown, cancellation, staging ownership, and catalog isolation.
Run `mise run check` and `mise run smoke`. These tests do not establish real
Redump availability, sustained throughput, or unattended publication readiness.
