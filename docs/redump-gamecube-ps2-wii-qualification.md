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
