# Game Boy family qualification

Recorded 2026-09-18 UTC. Scope: complete public Standard DATs for Game Boy,
Game Boy Color and Game Boy Advance. Acquisition and public membership
qualification passed locally. Game Boy is now publicly deployed and passed
isolated ROMD acceptance. GBC/GBA await the next scheduled healthy Game Boy
publication before enablement; see the production evidence below.

## Reviewed identities and selection

The official [system selector](https://datomatic.no-intro.org/index.php?page=download&op=select&s=49)
binds these exact headers to the upstream IDs below. Separate e-Reader, Video,
Multiboot and Play-Yan catalogs are not included in the GBA selection.

| ROMD key | Upstream ID | Exact header | Games / ROMs | Bytes | Version | Nodump / baddump | Minimum games / ROMs |
| --- | --- | --- | --- | --- | --- | --- | --- |
| `gb` | `46` | Nintendo - Game Boy | 2,332 / 2,332 | 969,988 | `20260915-022607` | 4 / 2 | 2,200 / 2,200 |
| `gbc` | `47` | Nintendo - Game Boy Color | 2,678 / 2,678 | 1,137,412 | `20260917-063400` | 2 / 4 | 2,500 / 2,500 |
| `gba` | `23` | Nintendo - Game Boy Advance | 3,790 / 3,790 | 1,503,446 | `20260915-015644` | 3 / 7 | 3,500 / 3,500 |

The floors are deliberately below the measured counts and detect gross
truncation. They are not exact expected counts or a replacement for membership
qualification. All downloads passed the existing 16 MiB input / 32 MiB expanded
document bounds and provider identity/ROM-structure validation.

Policy v2 preserves the SNES complete-public policy with reviewed form variants:

- Where exposed, `inc_mia=1` includes missing-in-action records with canonical,
  untagged names. Missing the control is valid for legacy forms; malformed or
  duplicate include controls fail closed.
- GBC's reviewed form has no collection selector. No collection control is
  invented or submitted. Its complete public membership is verified below.
- GBA selects `numbered=0`, `inc_xroms=1` and `inc_zroms=1`. Its reviewed form
  does not expose the adult control used by GB/SNES. Both additional ROM groups
  are included. GBA retains the explicit default, non-merged representation.
- GB uses the existing default, non-merged representation and all inclusion
  categories. Every system includes nodump records and all regions/languages.

The initial adapter rejected the extra/missing controls. The implementation was
updated after inspecting each official form, then acquisitions passed. No login,
browser automation, alternate mirror or client-side filtering was used. Unknown
controls continue to fail closed.

## Completeness reconciliation

Independent official database XML exports were downloaded with anonymous
sessions, bounded responses and five-second pacing. Their versions match the
DAT headers. These exports have sibling `header` and `datafile` elements; the
disposable analysis wrapped them in a root element in memory without changing
the downloaded files. IDs were compared exactly, including GBA's `x` prefixes.

| System | Public database IDs | Explicit `dat="0"` exclusions | DAT IDs | Missing eligible / extra IDs |
| --- | --- | --- | --- | --- |
| GB | 2,334 | 2: `1655`, `1786` | 2,332 | 0 / 0 |
| GBC | 2,678 | 0 | 2,678 | 0 / 0 |
| GBA | 3,793 | 3: `x314`, `x315`, `x317` | 3,790 | 0 / 0 |

Every dumped ROM matches its size and every supplied CRC32/MD5/SHA1/SHA256
against a file within its corresponding database archive. There were zero
mismatches. Nodump records retain unknown fields. This proves completeness
against these public DAT-enabled database snapshots, not non-public data or
independent ROM-content authenticity. Overview counters are not used as exact
acceptance counts.

Exact extracted-document SHA-256 values:

| System | DAT | Database XML |
| --- | --- | --- |
| GB | `297cc4a8e3805ca8072e4f020f4a6effdaaca9169d6d7827ddf51954d72e2522` | `61d6055e96c1288f32e8a8225e742bc49c3e3c68fd3c430fa4444c4e97fcf2f5` |
| GBC | `3e25d21b826bfac1cbd4397e3b69cf90d2a1ca3b51f67df8a80437f7b7e424c8` | `92ebbb636126bd7eb675043d9979398362257b3f488bf463b2f5089adb618d78` |
| GBA | `6874518e405797e7b0228fe4a8d3ab4a03ad776240f25f0c7e199ccb4a704ef1` | `e5482c3ff774a215132d4d4af5f78f80e6c145fa40f74bb641f42bd83ceb81a5` |

Database XML sizes were 3,575,357, 3,351,973 and 4,978,267 bytes respectively.
Upstream downloads and the disposable analysis remain outside Git.

## Batch publication implementation

`publisher` accepts repeated `--catalog ID` and `--pause-catalog ID` flags.
`--paused` remains compatible with the original single-selection CLI. Selection
is explicit, duplicate/unknown IDs are rejected before network access, and the
batch is limited to 25 catalogs. Manifest mode remains separate.

One adapter per provider is reused across the selected catalogs. No-Intro's
five-second request-start gap spans catalog boundaries. A returned retry deadline
stops subsequent acquisitions for that provider; the remaining selected catalogs
record the deadline and retain any working artifacts. On the next invocation,
the maximum persisted deadline across all known catalogs for that provider is
honored, including catalogs not selected that time. Pausing preserves deadlines.
Configuration/staging errors abort before any batch publication is committed.

The daily workflow uses one batch invocation with the existing PSX/SNES toggles
and three new opt-ins: `NOINTRO_GB_PUBLISH_ENABLED`,
`NOINTRO_GBC_PUBLISH_ENABLED`, `NOINTRO_GBA_PUBLISH_ENABLED`. Missing or false
values pause existing catalogs without inventing unavailable ones. No production
variables or workflow pins were changed during qualification.

## Validation and rollout boundaries

Passed:

- `mise run check`: race tests, formatting, vet, workflow lint and three builds.
- Synthetic tests for routing, preflight rejection, fatal batch errors, provider
  cooldown persistence, artifact retention and pacing across catalog boundaries.
- Reviewed handheld form selections, MIA ambiguity rejection and cross-system
  form/DAT rejection.
- Signed schema-3 candidate reading, exact-byte export, immutable URLs, RSS and
  restoration for SNES, GB, GBC and GBA, using synthetic documents/test roots.
- Synthetic smoke publication and definition validation.
- Authenticated restoration of the current live schema-2 publication into a
  disposable local directory using the original pinned public trust root.

The ROMD checkout's Dockerfile pins reader revision
`88ffe6af951f6d09f65a38ab457284b7b657a20e`, the schema-3 reader baseline used here.
This is source compatibility evidence, not verification of a deployed ROMD image.
No backend, browser, ROM import or hardware acceptance ran in this qualification.
No real changed-version pair or scheduled handheld acquisition has been observed.

The operator confirmed standing redistribution and retention approval for all
Redump and No-Intro catalogs on 2026-09-18 UTC. See `AGENTS.md`; this includes
these three additions and supersedes the earlier SNES-only scope.

Then deploy the reviewed tooling pin while retaining the existing trust root and
monotonic metadata versions, enable GB alone, and verify its signed public
candidate plus ROMD discovery/review/activation/failure retention. Observe the
next scheduled healthy check before enabling GBC/GBA. Do not reset the existing
publisher or mark the remaining expansion batches as qualified.


## Game Boy production rollout and acceptance

Verified 2026-09-18 UTC. Tooling [PR #21](https://github.com/MoonlarkStudios/romd-dat-catalogs/pull/21)
merged at `98988f0785d5119fb6bb376cbc272a8c55f5c608` after CI passed.
Data [PR #4](https://github.com/MoonlarkStudios/romd-dat-data/pull/4) pins both
workflow references to that exact commit. `NOINTRO_GB_PUBLISH_ENABLED=true`;
GBC and GBA variables remain unset/disabled.

[Production run 35306019995](https://github.com/MoonlarkStudios/romd-dat-data/actions/runs/35306019995)
passed acquisition, immutable Git export, signing, asset verification, Pages
publication and deployed restoration. Independently authenticated local
`catalogs` and `candidate` commands verified publication **3022**, registry schema
**3**, and the original pinned root. Game Boy is healthy with successful check
`2026-09-18T04:13:31.54340199Z`; counts, bytes and hash exactly match the local
qualification above. PSX and SNES are also healthy. GBC/GBA definitions are present
but neither has a published snapshot artifact, and ROMD does not advertise them
as available subscriptions.

Application acceptance used a separate Docker project with fresh PostgreSQL
and data volumes, the existing local admin/worker development images, and a fresh
admin frontend build. Existing development containers/data were untouched.
This is split-host local acceptance, not a NAS deployment or new standard image
build. No application source changes were needed.

Verified:

1. The browser directory advertises GB, PSX and SNES subscriptions. GBC/GBA
   remain bring-your-own-DAT systems.
2. Add Game Boy, choose No-Intro, and review the signed document: 2,332 added
   entries and files, version `20260915-022607`, and four BIOS entries.
3. Approve in the browser; processing finishes and the source becomes Active
   with 2,332 entries. Initial review is not activation.
4. A repeated check reports UpToDate. Database inspection shows one DAT version,
   2,332 entries, 2,332 files and one import job.
5. Inject an unreachable publisher URL into the isolated admin. The browser
   reports that update availability has not been verified and offers Retry
   availability. Download installed DAT still returns HTTP 200.
6. Through ordinary test-account OAuth, POST the catalog enrollment check
   (`/api/dat-subscriptions/check`). It returns CheckFailed with the message that
   installed catalogs are unchanged and the check can be retried. The active
   DAT download is 969,988 bytes and matches the qualification SHA-256 exactly.
7. Restore the real publisher URL and repeat the enrollment check. It returns
   UpToDate with a null error message; the active download still matches exactly,
   and there remains one DAT version and one import job.

The first API probe used the older per-DAT subscription-check endpoint; during
publisher unavailability it returned DatSubscription.Unavailable (HTTP 400)
before fetching. The successful CheckFailed/recovery evidence above is for the
catalog-enrollment endpoint. The UI's availability error is not evidence that
it displayed the enrollment CheckFailed message.

`mise run check` and GitHub publisher CI passed. The admin frontend build passed
with the existing SignalR annotation and large-bundle warnings. No backend suite,
ROM import, console launch, hardware, or changed real GB revision was exercised
in this rollout. The original reader/root/cache contracts were retained.

Next gate: observe a subsequent **scheduled** healthy GB acquisition, then enable
GBC/GBA and perform their public and application acceptance. A manual second run
must not be represented as the scheduled observation. An authorized hourly follow-up will verify that gate, complete GBC/GBA
rollout and acceptance, and stop when batch 1 is complete.
