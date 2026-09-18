# GameCube, PlayStation 2 and Wii qualification

Recorded 2026-09-18 UTC. The operator selected GameCube plus PS2 and Wii as the
next batch. Scope is the complete official standard Redump disc catalogs;
BIOS, serial/version variants and game-file conversion are separate scopes.
Standing redistribution approval is recorded in AGENTS.md.

## Identity and representation

The [official downloads page](https://redump.info/downloads) binds GameCube to GC,
PS2 to PS2 and Wii to WII. Lowercase HTTPS paths `/datfile/gc`, `/datfile/ps2`,
`/datfile/wii` returned standard single-DAT ZIPs without redirects. Shared ROMD
keys are `gc`, `ps2`, `wii`; representation is `discs`. Exact headers are
`Nintendo - GameCube`, `Sony - PlayStation 2`, `Nintendo - Wii`.

GameCube contains 2,022 ISO records. Wii contains 3,782 ISO records. PS2 contains
8,642 ISO, 3,191 cue and 3,229 binary-track records. There is no region filtering,
1G1R selection, image trimming or conversion. Explicit `(Disc ...)` labels occur
on 158 GameCube, 166 PS2 and six Wii entries; every disc remains separate.
Every file has size, CRC32, MD5 and SHA-1 fields.

The largest expected game-file sizes are 1,459,978,240 (GC), 8,539,963,392 (PS2)
and 8,511,160,320 (Wii) bytes. PS2 has 1,953 and Wii has 3,777 records above 4 GiB.
These are metadata values, not sizes downloaded by the publisher. Catalog bytes
fit the existing 16 MiB input / 32 MiB expanded bounds without capacity changes.
This release does not add hashing support for compressed RVZ/WBFS/CHD images;
the catalog preserves the original standard image sizes/checksums.

## Measured acquisitions

| System | Version | Disc entries / files | ZIP bytes | DAT bytes | Entry/file floors |
| --- | --- | --- | --- | --- | --- |
| gc | 2026-09-18 04-52-35 | 2022 / 2022 | 164249 | 723053 | 1800 / 1800 |
| ps2 | 2026-09-16 09-14-09 | 11833 / 15062 | 1251807 | 4851115 | 11000 / 14000 |
| wii | 2026-09-17 01-22-35 | 3782 / 3782 | 311908 | 1381637 | 3500 / 3500 |

| System | Standard DAT SHA-256 | Serial/version DAT SHA-256 / bytes |
| --- | --- | --- |
| gc | `8682bf1a48926cf2319291a5b513e8f7d0809e48f87a79b40f100072fd6250cc` | `ea5f884c6ff55bf1198dda5c2e25f8dc23f55a05aec444b8cab511ad12ac07bd` / 795947 |
| ps2 | `8a9d1fe4fe4c143f636cb9b709229ee1afea74da2a5699ce0ebe02af38a0cafb` | `5f3d7c442c7ec47dd6a5aa99b0fe653044dfe2ee75fffdeb8345daff682523e4` / 5535185 |
| wii | `99d9fa34bc2634ed6a850e8fc73763e59a5837bab67d0267f82a7ec898d5a665` | `e63b7ce1c6f078d2568963c830c39cac229efa1f27021358c9c974fddd7ae05c` / 1515766 |

The independently downloaded official serial/version export has the same version
and exactly the same disc IDs for each system. Every binary record's size and
CRC32/MD5/SHA1 tuple matches by disc ID, including multiplicity: zero missing IDs,
extra IDs or binary hash/size mismatches. These are alternative exports of the
same upstream data, not independent rehashing of game images. Cue filenames change
in the alternate representation, so cues are checked against the companion export.

The PS2 cuesheet ZIP is 877,851 bytes, SHA-256
`a6971051a873c947eb8f0ff575be703d95db617010fa13a298e6ba2a3eb4aaba`.
Its 3,191 cues exactly match the standard DAT's CD subset by disc name, cue byte
length/SHA-1, and referenced binary filenames/order. There are zero mismatches.
Raw upstream documents and temporary qualification scripts remain outside this
repository; all published original bytes and notices are preserved.

## Implementation and validation

Reviewed definitions have explicit per-catalog opt-ins and measured floors.
The existing shared Redump adapter, cooldown, artifact retention, pinned root and
metadata version progression remain unchanged. No backend/API, generated client,
database schema or runtime changes are required for this catalog release.
Signed candidate tests now exercise all three new mappings along with the
previous Redump systems, preserving multi-disc/multi-file synthetic candidates,
wrong-header rejection, exact immutable downloads, RSS and restoration.

`mise run check` passed race tests, vet, formatting, workflow lint and builds.
Synthetic smoke publication, definition validation and diff checks passed.
Production-adapter acquisition with the final definitions passed all three at
`2026-09-18T18:43:24.18231Z`, with the exact qualified hashes and counts above.
Public deployment and isolated application acceptance are separate gates.

## Complete public membership

All public system-listing pages were read with paced, bounded requests: 21 GC,
119 PS2 and 38 Wii pages. Unique `/disc/{id}` values exactly match each standard
DAT's game IDs: 2,022 GC, 11,833 PS2 and 3,782 Wii. There are zero missing public
IDs or extra DAT IDs for all three. The public listing totals also match. No
questionable-record exclusions were needed for this batch. This proves observed
public export coverage at qualification time, not that every released disc is
known to Redump.

## Release authorization and tooling pin

The operator requested GameCube and selected PS2/Wii as its companion batch.
Tooling [PR #31](https://github.com/MoonlarkStudios/romd-dat-catalogs/pull/31)
passed CI and merged at `f1652d0ce396de9682caf564888e6eb6a05f71e8`.
Data [PR #9](https://github.com/MoonlarkStudios/romd-dat-data/pull/9) pins both
workflow references to that revision. Explicit variables `REDUMP_GC_PUBLISH_ENABLED`,
`REDUMP_PS2_PUBLISH_ENABLED` and `REDUMP_WII_PUBLISH_ENABLED` are enabled.
The existing root and metadata progression are retained. No rollout automation
was created or resumed, and no scheduled observation is claimed.

## Initial production publication

[Run 35382363216](https://github.com/MoonlarkStudios/romd-dat-data/actions/runs/35382363216)
passed the workflow, including deployed signature-chain restoration. Independent
verification authenticated publication **3032**, schema **3**. All three new
catalogs are healthy with successful acquisition at
`2026-09-18T18:53:47.495940887Z`; signed candidate downloads exactly match the
qualified bytes/hashes/counts. Immutable assets reference data commit
`b2765a244b9b1b8cb26bda5e11a03bbfca572c2f`.

This first run was not healthy for every existing source: NES acquisition logged
`prepare_failed`, and its signed health was failed while its prior document was
retained. No retry deadline was recorded. A fresh recovery run was dispatched
without changing validation, clearing caches or resetting trust. The exact failed
preparation stage was not retained, so no specific upstream root cause is claimed.

## Isolated ROMD acceptance

The existing separate acceptance Docker project and database/data volumes were
reused. GameCube, PS2 and Wii each passed browser discovery, verified first-import
review, explicit approval, processing and Active source checks. Review versions
and entry/file counts matched qualification; each reported zero BIOS entries.

Every imported game/file name, size, CRC32, MD5 and SHA-1 was compared against
the qualified standard DAT using counted tuples. All 20,866 file records match,
including 1,953 PS2 and 3,777 Wii records above 4 GiB. Maximum imported sizes
exactly match the measurements above. No narrowing, truncation or loss of CD
tracks or cues was observed in these metadata imports.

Normal test-account OAuth checks returned UpToDate without error, and active
DAT downloads matched signed lengths and SHA-256. Pointing only the isolated
admin at an unreachable publisher returned CheckFailed for all three while the
same active DAT IDs and exact downloads remained available. Restoring the real
publisher returned UpToDate, no errors and zero consecutive failures.

Before/after evidence is identical apart from SQL row order: 16 Active versions,
46,373 entries, 96,789 files and 16 import jobs (the previous 13 catalogs plus
these three). There were no duplicate imports or replaced active documents.
The saved original Compose definition was used so concurrent application deployment
edits remained untouched. Existing application images/frontend were reused; no
application source or generated client changed, and no new backend suite or
frontend build was run.

An automatic browser approval review timed out once while opening source discovery.
The permitted retry succeeded; the test account then signed in through the normal
flow after its session expired. The timeout did not require bypassing browser
security or changing app permissions.

Acceptance is for catalog ingestion and retention. No game-image import or
RVZ/WBFS/CHD conversion, game launch, disc switching, hardware/emulator acceptance,
NAS deployment, changed real upstream revision or scheduled observation is claimed.

## Final production recovery and closeout

[Recovery run 35385781139](https://github.com/MoonlarkStudios/romd-dat-data/actions/runs/35385781139)
passed the complete workflow. Independent verification with the retained cache
and original root authenticated publication **3033**, schema **3**, with all
**18 real catalogs healthy**, including NES. The three new catalogs and NES
record successful acquisition at `2026-09-18T19:29:45.792905232Z`. New-catalog
artifact hashes and sizes are unchanged from 3032 and qualification; their
original signed content events remain at sequence 74. This is a second healthy
manual acquisition of the new catalogs, not a scheduled observation or changed
upstream revision. The first NES failure is retained above rather than hidden
behind overall workflow success.

A refreshed GameCube browser page confirmed Active and Up to date after the
application outage test. Batch 5 is complete within acquisition, signed
publication and isolated ROMD catalog-ingestion scope. Implementation CI, local
checks, both production workflows and the acceptance checks described above
passed within their stated limits. Existing GitHub Action Node-20 deprecation /
forced Node-24 execution and upcoming Ubuntu-image notices remain unchanged.
