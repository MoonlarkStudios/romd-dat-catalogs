# Nintendo DS, Nintendo 3DS, PSP and PC Engine CD qualification

Recorded 2026-09-19 UTC. This batch adds the four missing subscriptions from the
requested set. Signed discovery also verified the existing Dreamcast, Wii,
Saturn and PC Engine/TurboGrafx-16 cartridge catalogs as healthy.
Standing redistribution approval is recorded in AGENTS.md.

## Identity and acquisition

[Datomatic](https://datomatic.no-intro.org/index.php?page=download) maps Nintendo
DS to provider ID 28 and Nintendo 3DS to 64. Both use explicit format 0
(Decrypted), canonical unnumbered names, all regions/languages/specials, BIOS,
prototype, physical/digital, x-ROM, z-ROM, nodump and untagged MIA inclusion.
DS also selects all license and lifespan categories, including aftermarket;
3DS exposes none of those controls. The 3DS provider default is Encrypted, so
format 0 is deliberately explicit. Exact header names include `(Decrypted)`;
an encrypted result is rejected. Catalog IDs retain the existing `standard`
convention and their representation must not silently change after release.
Digital-only 3DS, New 3DS, DSi and DS Download Play catalogs are separate sources.

[Redump downloads](https://redump.info/downloads) maps PSP to PSP and PC Engine CD
to PCE. Lowercase `/datfile/psp` and `/datfile/pce` returned standard single-DAT
ZIPs without redirects. ROMD keys are `psp` and `tgcd`; the latter is separate
from the existing `tg16` cartridge catalog. Every disc, track and cue remains
in its original upstream representation. PSP PSN and UMD Video/Music catalogs
are separate sources; compressed game-image hashing is outside this release.

## Measured acquisition

| System | Exact header | Version | Entries / files | DAT bytes | SHA-256 |
| --- | --- | --- | --- | --- | --- |
| psp | Sony - PlayStation Portable | 2026-09-15 07-22-30 | 3545 / 3545 | 1269384 | `ff961042f59497412daadd360ad7e888e2fbc2a476d79bc605f772e6ce003e2e` |
| tgcd | NEC - PC Engine CD & TurboGrafx CD | 2026-08-13 18-08-53 | 551 / 16311 | 3084883 | `45071e81f227a41b7ae67b999b67fb2b13c1f1223f21e6ebc8ee3022b41e3370` |
| nds | Nintendo - Nintendo DS (Decrypted) | 20260918-033341 | 7708 / 7708 | 3118839 | `0b763e63d13ede8215f600c9350e61206d91763fa7dff2df11cd8872e0a3dde0` |
| 3ds | Nintendo - Nintendo 3DS (Decrypted) | 20260914-103505 | 2166 / 2166 | 990369 | `83e995023f574dc9606493e9b1e93d1fdaa54cc538f10d94b4ff8f46731d2d4d` |

Entry/file floors: DS 7000/7000, 3DS 1900/1900, PSP 3300/3300, CD 500/15000.
All inputs fit the unchanged adapter size/time limits. Anonymous No-Intro
requests retain the shared five-second admission gap and reviewed form checks.
Redump retains its shared provider adapter and cooldown. Acquisitions preserve
exact bytes and upstream attribution; no raw DATs are committed here.

## Completeness evidence

- PSP: all 3,545 public disc IDs across 36 pages match the standard DAT.
- PC Engine CD: all 551 public disc IDs across six pages match the standard DAT.
- Both Redump serial/version exports match every disc ID and every binary
  size/CRC32/MD5/SHA1 tuple, including multiplicity. Zero discrepancies.
- All 551 PC Engine CD cues match byte length/SHA1 and the DAT's binary filenames
  and track order. The largest entry has 100 files; all 16,311 files carry hashes.
- The public 3DS database export has the same version, 2,166 IDs, zero exclusions,
  and matching size/hash fields for every dumped ROM. It preserves one nodump
  and two baddump records. This checks upstream consistency, not game-image rehashing.
- DS has no anonymous database export. Its full encrypted export independently
  matches all 7,708 decrypted entry IDs at the same version, with zero missing or
  extra IDs. This is a cross-representation check of the same provider, not an
  independent database inventory. The complete reviewed form selects every
  available inclusion category; encrypted/decrypted hash differences are expected.

## Validation and rollout

`mise run check` passed race-enabled offline and localhost integration tests,
vet, formatting, actionlint and builds. Signed candidate tests cover all four
new mappings, exact bytes, immutable artifacts, feeds and restoration. DS/3DS
form tests reject missing categories, wrong identities and ambiguous formats;
validation rejects encrypted headers. `validate-definitions` passed.
No ROMD backend, API, generated client, database schema or runtime changes are
required. The rollout adds four explicit workflow opt-ins, then pins the data
workflow to the merged tooling commit. Publication and isolated ROMD acceptance
are pending; local qualification is not a production claim.
