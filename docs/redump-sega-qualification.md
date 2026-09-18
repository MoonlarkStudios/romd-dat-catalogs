# Sega disc batch qualification

Recorded 2026-09-18 UTC. Scope: complete official standard Redump DATs for
Saturn, Sega CD/Mega CD and Dreamcast. Serial/version variants, BIOS exports,
Sega arcade systems and game files are outside this release.

## Identities and coverage

The [official downloads page](https://redump.info/downloads) binds Saturn to
SS, Sega CD/Mega CD to MCD and Dreamcast to DC. Reviewed lowercase HTTPS endpoints
`/datfile/ss`, `/datfile/mcd` and `/datfile/dc` returned standard ZIPs without
redirects. ROMD keys are `saturn`, `segacd`, `dc`; representation is `discs`.
Exact headers are `Sega - Saturn`, `Sega - Mega CD & Sega CD` and
`Sega - Dreamcast`. Standing redistribution approval is recorded in AGENTS.md.
Original bytes, attribution, every region and separate disc entry are preserved.

All public listing pages were read (25 Saturn, six Sega CD, 16 Dreamcast) and
unique disc IDs compared with DAT IDs. Saturn has 2,466 public IDs versus 2,464
exported; Sega CD has 553 versus 549; Dreamcast has 1,518 in both. There are no
DAT IDs missing from the public lists. The six public-only IDs are all marked
Questionable on their official detail pages and absent from both standard DAT
and cuesheet exports:

- Saturn [25249](https://redump.info/disc/25249), Fighting Vipers (6S): possible bad dump;
  [61415](https://redump.info/disc/61415), Return Fire prototype: mastering-format uncertainty.
- Sega CD [61251](https://redump.info/disc/61251), TimeCop;
  [61254](https://redump.info/disc/61254), Star Strike;
  [61256](https://redump.info/disc/61256) and [61257](https://redump.info/disc/61257),
  Johnny Mnemonic discs 1/2: comments describe corrected errors and required verification.

These are observed upstream export exclusions, not local filtering. The mirror
covers the entire official standard export, not every questionable website record.
Independent companion cuesheet exports match every DAT disc name exactly, with
zero missing/extra entries. All referenced track filenames and order match the
DAT's binary records; all exported cue lengths and SHA-1 hashes match their DAT
records. This is exact public-membership and companion-export reconciliation,
not independent rehashing of game files or a second source for every track hash.

## Measurements

| System | DAT version | Disc entries | Files (cue + binary tracks) | ZIP bytes | DAT bytes | Entry/file floors |
| --- | --- | --- | --- | --- | --- | --- |
| saturn | 2026-09-15 01-11-08 | 2464 | 30673 | 1713863 | 6039981 | 2300 / 28000 |
| segacd | 2026-09-15 13-21-07 | 549 | 8426 | 493902 | 1613044 | 500 / 7500 |
| dc | 2026-09-17 07-52-35 | 1518 | 12616 | 641081 | 2597958 | 1400 / 11500 |

| System | DAT SHA-256 | Cues ZIP SHA-256 / bytes |
| --- | --- | --- |
| saturn | `be302f67d366c64cb94a9c44128660073f8dc29e4123806f0a1ec50495a43cef` | `cb3ff91fdcd1ffa427cb5a4c7251350e84b55b6b50925cf97045193a17c6d5cd` / 928140 |
| segacd | `057bac5049f388178aa054fe3c8fe23c01da6fa0cf39e54781de8b84380264cf` | `95a6922384e6803f157e3d32fda2ebe0f4bb7adf001f6262ce41a8d5ac08b81a` / 199286 |
| dc | `43320ca5d14784ca72f039af4aa7a803f8eaa06d6101018d01a452bce3e40bbb` | `5c002b0f03693d2fffb27913eba782733f468cd778418b9ba18edc9b7ecaed78` / 535709 |

The 51,715 file records contain 4,531 `.cue` records and 47,184 `.bin` tracks.
Saturn has 28,209 binary tracks, Sega CD 7,877 and Dreamcast 11,098. Maximum
binary tracks per disc are 98, 96 and 99 respectively. Explicit `(Disc ...)`
labels appear on 402 Saturn, 77 Sega CD and 151 Dreamcast entries; they remain
separate entries. Every file has size, CRC32, MD5 and SHA-1 fields. No raw upstream
DAT or temporary qualification script is committed to the tooling repository.
All inputs fit the existing 16 MiB input / 32 MiB expanded bounds.

## Implementation and validation

Only reviewed definitions, explicit workflow opt-ins, signed multi-disc candidate
tests and documentation change. One shared Redump adapter continues to serve
all selected disc catalogs with existing bounds, failure retention and cooldown.
No trust root, metadata schema, application API, generated client or database
migration changes. Missing/false opt-ins pause each corresponding catalog.

`mise run check` passed race tests, vet, formatting, workflow lint and builds.
Synthetic smoke publication, definition validation and diff checks passed.
New signed-candidate tests cover PSX and all three new system mappings, multi-disc
entries with multiple files, wrong-header rejection, exact bytes, immutable
exports, RSS and authenticated restoration. Existing adapter tests retain coverage
of malformed data, floors, bounds, rate limits and failed acquisition retention.
Real acquisition through the production adapter and final measured definitions
passed for all three at `2026-09-18T16:55:52.45367Z`, with exact hashes above.

Production publication and isolated ROMD acceptance are separate gates. This
qualification alone does not establish ROM import, cue parsing during game
launch, disc switching, emulator/hardware acceptance or unattended reliability.
