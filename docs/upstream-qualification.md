# Upstream qualification: No-Intro and Redump

Current PlayStation decision: see [the official redump.info qualification](redump-psx-qualification.md). It supersedes the acquisition and redistribution gates below for the standard PSX DAT only. Other providers remain unqualified.

Historical observations, 2026-09-06 UTC (2026-09-05 America/Chicago). This report covers bounded
public requests and local validation, not permission to mirror, full coverage,
unattended reliability, or ROMD activation. Public publication remains synthetic.
No credentials or upstream messages were used. No upstream DATs are committed.

## Historical recommendation

Engineer the Redump adapter first: its download listing exposes per-system
archive endpoints, with separate BIOS identifiers. Keep live acquisition and
mirroring disabled until transport policy and redistribution conditions are
resolved. No-Intro remains required; qualify its acquisition method before
implementing a production adapter. An HTTP 200 HTML page is not a valid DAT.

The first implementation can proceed with synthetic HTTP fixtures, without
waiting for permission to redistribute upstream catalogs. Production enablement
is a separate acceptance decision supported by upstream terms or confirmation.

## Current observations

| Surface | Result | Implication |
| --- | --- | --- |
| [Redump downloads](http://redump.org/downloads/) | HTTP 200; 60 distinct DAT links | Discovery count only, not 60 validated catalogs. System listings and public DAT coverage differ. |
| [PlayStation DAT](http://redump.org/datfile/psx/) | ZIP 4,021,187 bytes; XML 12,756,749 bytes; 10,914 game/disc records and 60,168 ROM/track records | Fits the current 16 MiB input / 32 MiB document limits for this sample. Not full-source scale evidence. |
| PlayStation document identity | Header `Sony - PlayStation`; version `2026-06-15 11-55-46` | Preserve upstream version as provenance; acquisition time is not content freshness. |
| Redump transport | HTTPS connection refused from this environment; HTTP succeeded | Do not silently downgrade HTTPS in a production adapter. Publisher signatures cannot authenticate the upstream HTTP leg. |
| Redump response validators | No ETag or Last-Modified observed for the sample | Extracted-document hashes remain authoritative for content change. Filename/HEAD metadata are hints. |
| [No-Intro Daily](https://datomatic.no-intro.org/index.php?page=download&op=daily) | Public HTTPS form exposes output type, collection, standard, numbered, and aftermarket controls | Make selections explicit and version the acquisition policy. |
| [No-Intro selector](https://datomatic.no-intro.org/index.php?page=download&op=select&s=64) | SNES maps to upstream system 49 | Bind the upstream identifier and representation to a reviewed platform mapping; do not identify catalogs by dated filenames. |
| SNES per-system form | Prepare succeeded; final request returned HTML rather than ZIP/XML | Download workflow not qualified by this probe. Cause not established; no browser or production reliability claim. |
| No-Intro sample defaults | All regions/languages, BIOS and several release/license categories selected; aftermarket and nodump not selected | Default output is not evidence of all-category completeness. No SNES DAT size/count measurement obtained. |

PlayStation extracted-document SHA-256:
`0d5cffb7feb15aa4297ccaf722c62d2b08f17eb859d6ab7890088d481bf8a18e`.
The merged Go publisher accepted the local PSX ZIP with the exact expected
header: one catalog, zero failures. This tests structural parsing and local
publication only. It does not validate every ROM hash against disc content.

Earlier local qualification measured a No-Intro 3DS document and observed
changing Redump ZIP bytes with identical extracted document hashes. Those are
historical observations, not a pass for this SNES workflow. A previous daily
pack request exceeded a 40 MB probe cap; a complete daily pack has not been
validated. The former 64 MiB synthetic recovery archive is superseded for public
publication by latest-only signed indexes and complete DATs in a separate Git
repository. Full-source acquisition still requires its own size qualification.

## Automation and distribution evidence

[No-Intro robots guidance](https://datomatic.no-intro.org/robots.txt) currently
specifies a five-second crawl delay and one request per five seconds. Use a
shared per-host limiter across discovery and download requests. This is request
pacing guidance, not a supported API contract or redistribution license.
[Redump robots](http://redump.org/robots.txt) has an empty Disallow directive;
that likewise does not grant mirroring rights or justify aggressive polling.

The [official No-Intro guide](https://wiki.no-intro.org/index.php?title=DAT-o-MATIC_Guide)
describes per-system and daily downloads. The reviewed
[No-Intro terms](https://datomatic.no-intro.org/stuff/terms.txt) discuss attribution,
trademarks, disclaimers, and project purpose; no explicit grant for our proposed
public DAT mirror was identified. No explicit equivalent grant was found in
the inspected Redump homepage, downloads page, or sample header. This limited
review does not establish that redistribution is prohibited.

Before enabling public mirroring, establish automated-access expectations,
redistribution/retention conditions, attribution, and removal handling. No
upstream contact has been sent. No third-party mirror is treated as authority
to grant upstream rights.

## Fresh1G1R reference

Reviewed [Fresh1G1R at commit 22dadc6](https://github.com/UnluckyForSome/Fresh1G1R/tree/22dadc6c8ef9b9da7d6f27eb9fa26533335f65b4).
Its [workflow](https://github.com/UnluckyForSome/Fresh1G1R/blob/22dadc6c8ef9b9da7d6f27eb9fa26533335f65b4/.github/workflows/daily-dat-processing.yml)
runs daily/manual acquisition and commits outputs. Its
[acquisition code](https://github.com/UnluckyForSome/Fresh1G1R/blob/22dadc6c8ef9b9da7d6f27eb9fa26533335f65b4/automate.py)
uses direct Redump HTTP downloads and a browser for No-Intro. It attempts to
uncheck Source Code, Unofficial, and Non-Redump; effective selection was not
verified by executing it. Redump skip decisions use filenames, not document
hashes. Processed outputs retain prior files on acquisition failures.

Adopt the daily/manual workflow and failure-retention principle. Keep ROMD's
hash identity, bounded extraction, signed publication, and complete-catalog
coverage policy. Retool filtering belongs in a later ROMD library feature.
No reference code was executed or copied. No root license file was observed;
code reuse would require checking its applicable license separately.

## Adapter acceptance contract

- Registry identity: source, upstream system identifier, representation,
  collection, inclusion-policy version, expected header, and explicit ROMD
  platform mapping. Keep BIOS, encrypted/decrypted, headered/headerless, and
  aftermarket distinctions. New or changed identities require review.
- Preserve full upstream documents and attribution. Do not filter regions,
  languages, revisions, demos, or prototypes in the companion publisher.
  Define nodump handling explicitly; never invent hashes or silently describe
  a filtered selection as a complete source catalog.
- Fetch into staging with bounded time/bytes, archive-entry limits, an allowed
  host/redirect policy, and content validation. Record actual transport.
  Respect Retry-After; bound retries and isolate per-provider failures.
- Retain prior artifacts on failures and treat a missing listing entry as an
  anomaly requiring review, not automatic deletion. Reject identity mismatch,
  HTML responses, empty/truncated documents, and suspicious count collapse.
- Hash exact extracted DAT bytes. Same hash means no new content event, even
  if archive packaging or download dates change. A changed header or formatting
  changes the exact-document hash; semantic change detection remains separate.
- Validate all intended source catalogs and measured memory/archive limits
  before broadening beyond a small explicit platform allowlist.

Minimum implementation checks: synthetic server tests for successful ZIP/raw
DAT, unchanged bytes/repackaging, redirects, HTML, truncation, limits, wrong
identity, 429/backoff, and per-catalog failure retention; `mise run check` and
`mise run smoke`; bounded opt-in live checks separated from offline CI. No
parallel agents are needed for this slice.

## Unsent upstream inquiry

We maintain an open-source ROM catalog tool and would like to check your public
DAT catalogs once daily, preserving complete documents and attribution. May we
redistribute immutable versions through a public companion repository's release
assets for clients to download, with a signed index and RSS notifications?
Which acquisition endpoint and request rate do you recommend? Please identify
any attribution, historical retention, removal, or restricted-catalog conditions.
We would not distribute ROMs, restricted data, or credentials.

This is a draft for operator review, not a message that has been sent.

## Real PlayStation document through ROMD

A follow-up on 2026-09-06 UTC exercised ROMD main
`7cc78c45` (companion main `3096287`). This was an isolated API/worker experiment,
not subscription implementation or a deployed instance change.

The bounded PlayStation request again returned HTTP 200 with a 4,021,187-byte
ZIP and the same 12,756,749-byte document/hash recorded above. HTTPS port 443
still refused the connection from this environment. No HTTP fallback was added
to the production adapter. The unchanged document is a real no-update case;
there is not yet a pair of genuinely changed upstream versions in this trial.

A temporary local xUnit probe used ROMD's authenticated admin test client,
real application PostgreSQL fixture, and existing upload/replacement execution.
The fixture uses in-memory Hangfire transport, so this is not split-host or
PostgreSQL transport acceptance. All four developer connection-string exports
were removed for the test.

- Full extracted DAT uploaded through `/api/upload/dat`, with no platform
  override. ROMD routed it to the seeded PlayStation platform (`psx`; numeric
  ID 21 in this fixture). Numeric database IDs are not the distribution identity.
- API counts and persisted rows matched all 10,914 disc entries and 60,168 track
  records. Original-document download matched the exact SHA-256 above.
- Upload admission to completion took 22.307 seconds in this one local run;
  this is not a throughput benchmark or unattended reliability result.
- A malformed replacement exposed a parse error while preserving the Active
  version, all its game rows, and byte-identical original download.
- The replacement job remained `Ingesting` with errors and scheduler retries.
  It did **not** establish the desired clear terminal failed-update/retry UX.

Final focused run: one test passed in 29.294 seconds, after correcting the local
probe's query and its incorrect expectation that malformed replacement would
immediately become terminal. The earlier terminal-state wait hit the 60-second
hang guard. Expected injected parse errors and a fixture Data Protection
unencrypted-key warning appeared; no product behavior was changed to hide them.
No full backend suite, browser acceptance, NAS deployment, ROM-content hash
verification, or public redistribution was performed. Source data and the
one-off probe were not committed. Local probe/logs are disposable evidence;
this report records their relevant results.

### Consequences for the first user flow

Use the existing source identity and activation transaction, but do not simply
pause the replacement job after ingestion. Current pending ingestion upserts
source claims, and projection code considers pending claims; its pending marker
is not evidence of isolation suitable for a long-lived review screen. Hold the
candidate document separately until approval and test that reviewing it cannot
change browse, ownership, or library results.

The next implementation should let a user select PlayStation, check for a
candidate, and understand its exact-version diff before approval. Show disc and
track changes with samples and explicit truncation; do not label track counts
as titles or imply uncomputed library impact. Identical document bytes should
report up to date without entering replacement. Invalid candidates should show
an actionable validation failure while the working catalog remains usable.
Approval must reject a stale active-version baseline and use the existing
activation path only after validation. First activation of a subscription and
binding an existing source both need explicit tests.

Direct HTTP acquisition for an opt-in reviewed trial is an operator decision,
not an implicit transport downgrade. Public mirroring still needs established
redistribution conditions. Neither unresolved item justifies adding more remote
scheduling infrastructure. Complete public PSX disc-document counts do not prove
BIOS coverage, restricted catalogs, all Redump systems, or No-Intro coverage.


## Follow-up after ROMD review acceptance (2026-09-06 UTC)

ROMD [PR #209](https://github.com/JackSkylark/romd/pull/209) is merged. The user
accepted the local Docker review demo. Large-diff browsing is tracked separately
in [ROMD #210](https://github.com/JackSkylark/romd/issues/210); it does not block
qualification of the first publisher candidate. Manual reviewed replacement is
implemented; saved subscriptions and publisher-to-ROMD fetching remain pending.

A fresh bounded network check returned HTTP 200 for Redump's homepage and
download listing. HTTPS port 443 again refused the connection from this host.
The public companion timestamp returned HTTP 200; this is availability evidence,
not a fresh cryptographic verification of the publication. No DAT was downloaded
or publicly published in this follow-up.

### Reuse evidence: correct the earlier search gap

Search now exposes an indexed version of the official
[Redump overview](https://wiki.redump.org/index.php?title=Redump.org) answering
whether Redump owns contributed database content. It describes the metadata as
public information available for reuse by individuals and organizations. This
is supporting evidence missed by the earlier limited review, not evidence that
Redump forbids redistribution. The indexed page reports modification on
2025-12-28. Direct retrieval here failed: HTTP returned 404 and the web fetch of
HTTPS returned 502. Verify a current accessible copy before treating this as
settled operational policy. It does not establish a supported HTTPS acquisition
endpoint, polling rate, or restricted-catalog access. The homepage links to the
wiki and identifies `admin@redump.org` as a contact; no message was sent.

The accessible [No-Intro terms](https://datomatic.no-intro.org/stuff/terms.txt)
remain insufficient to establish the proposed mirror conditions. Do not apply
Redump's metadata statement to No-Intro.

### Fresh1G1R technical takeaways, rechecked

The GitHub API still resolves Fresh1G1R HEAD to
[22dadc6](https://github.com/UnluckyForSome/Fresh1G1R/tree/22dadc6c8ef9b9da7d6f27eb9fa26533335f65b4).
Reading the workflow and acquisition functions confirms:

- Daily and manual CI are sufficient triggers for the initial source slice.
- Redump acquisition uses HTTP, a listing GET, filename-oriented HEAD/GET
  checks, and ZIP extraction. This does not solve ROMD's upstream transport
  decision. Preserve ROMD's document hashes and bounded archive validation.
- No-Intro uses Chromium/Playwright to navigate the daily form and wait for a
  download. This is a useful description of the interaction to qualify; it is
  not evidence that ROMD must add a browser runtime or that the form is a stable
  API. Its unchecked categories must not silently define ROMD completeness.
- Successful outputs replace prior files individually; download failures do not
  bulk-delete working outputs. Keep that observable behavior in the ROMD slice.

No third-party code was executed or copied. No root license file was present in
the returned repository listing. Keep the existing Go/mise toolchain; do not add
Retool filtering, filename-based version identity, or a second language merely
to reproduce this example.

### Next slice and explicit gates

1. Target `http://redump.org` explicitly, as authorized after this follow-up.
   Redump supports HTTP; the adapter uses that transport directly, with no HTTPS
   probe or fallback. This resolves the transport decision for Redump acquisition.
   Distribution signatures do not authenticate the upstream HTTP leg.
2. Acquire one complete public PSX disc document using the existing reviewed
   identity and size/count checks. Preserve source attribution and distinguish
   this from BIOS, restricted-catalog, or full-Redump coverage.
3. Run the existing publisher/signing path and hand the exact authenticated
   candidate to ROMD's preview and approval flow. A private/local trial can
   establish this connection before public mirroring is enabled; signing keys
   for such a test must be separate disposable test keys.
4. Demonstrate unchanged document/repackaged ZIP producing no content event,
   a validated changed candidate requiring review, and acquisition/validation
   failure preserving the working catalog with an actionable error. Label
   synthetic changes as synthetic until two actual upstream versions are seen.
5. Enable a narrow public source only after reuse conditions are established.
   Observe real runs before adding automation or recovery based on actual gaps.

Existing scheduler/checkpoint tools remain available, but extending or deploying
them is not an acceptance requirement for this slice. Do not bundle large-diff
UX, broad platform enrollment, No-Intro automation, or unattended activation.

Validation for an acquisition code change remains `mise run check` and
`mise run smoke`, with synthetic boundary/failure fixtures. ROMD endpoint/UI
changes require its backend, integration, generated-client, and web routes.
This documentation follow-up checked whitespace and referenced local files;
it did not rerun Go or ROMD runtime suites.


### Authorized Redump transport

The operator confirmed Redump is HTTP-only and directed the adapter to target
HTTP. This supersedes historical statements above that describe transport as an
unresolved decision. Existing bounds, no-redirect policy, document validation,
content hashes, and last-working-catalog retention still apply. The production
constructor and acquisition provenance use `http://redump.org`; public signed
Pages/Release distribution continues to use HTTPS. Public upstream mirroring
has not been enabled by this transport change.
