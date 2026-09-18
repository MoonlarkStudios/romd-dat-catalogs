# Cartridge batch 3 qualification

Recorded 2026-09-18 UTC. Master System, Game Gear, PC Engine/TurboGrafx-16
and 32X passed real acquisition and independent public database reconciliation.
Production publication and ROMD acceptance are separate gates, recorded below
when completed. Standing redistribution approval is in `AGENTS.md`.

## Reviewed identities and inclusion

The official No-Intro selector binds SMS to 26 (`Sega - Master System - Mark III`),
GG to 25 (`Sega - Game Gear`), TG16 to 12 (`NEC - PC Engine - TurboGrafx-16`),
and 32X to 17 (`Sega - 32X`). Each reviewed form labels format 0 Default and
its DAT header matches the selector. All use the standard representation.
SuperGrafx, PC Engine CD and Sega CD remain separate scopes.

These four forms have no collection or adult selector. Only SMS exposes nodump.
SMS includes the x-ROM and z-ROM groups; GG includes x-ROMs. Exposed MIA controls
for GG/TG16 are included. All exposed regions, languages, release, license,
lifespan, storage and special categories are included. Unexpected controls and
missing required selections fail closed. No local filtering modifies DAT bytes.

## Measured evidence

Acquisition completed at `2026-09-18T16:17:23.486573Z`.

| System | Version | Entries / files | Bytes | Nodump / baddump | Entry/file floor |
| --- | --- | --- | --- | --- | --- |
| sms | 20260918-065535 | 1243 / 1243 | 567084 | 2 / 0 | 1100 |
| gg | 20260822-061808 | 928 / 928 | 423838 | 0 / 1 | 850 |
| tg16 | 20260124-120557 | 510 / 510 | 215442 | 0 / 0 | 450 |
| 32x | 20260317-140429 | 227 / 227 | 115434 | 0 / 0 | 200 |

SMS contains 1,241 `.sms` files and two extensionless nodump records. GG contains
902 `.gg` and 26 `.sms` files. TG16 contains 510 `.pce` files. 32X contains
224 `.32x` and three `.bin` files. All are retained exactly as supplied.

| System | DAT SHA-256 | Database XML SHA-256 / bytes |
| --- | --- | --- |
| sms | `da1ac3653d48edef50fa6901b61f07fcb48fb700faeb46c97e223e940c3d2148` | `ff8d4fda4aa8bad2e98f333476326897eb942c710a78d202b3b166800e03bfae` / 2003538 |
| gg | `42dd51682a8e8165e625ea79a93ea5fd430aa06802a46da8d72ac96c6218f605` | `bc984a60238c25acd9c838e6e287ae631cee93159534fbe0dc1111a43dfe5636` / 1768225 |
| tg16 | `b417844bdb408dfc81cb711391fef26d774535a79a98427c162524e5730dfcc2` | `3a8bcca794645be423f84e94d39d339be0107bb3f1234412d16b4ed23c118c99` / 1060000 |
| 32x | `0e6e08409166359950a9b9680333c61bb98aaca12b70f27ea3c8b599c1243060` | `cceb48b0c1624cc2d73f02bfa3007db6eb62398a2c40fa0a1cc11227dcaaeb79` / 202036 |

Independent official database exports contain 1,245 SMS IDs (explicit dat=0
exclusions `1158`, `1183`), 928 GG IDs (no exclusions), 510 TG16 IDs (no
exclusions), and 228 32X IDs (explicit dat=0 exclusion `0216`). Eligible archive
IDs exactly match DAT IDs, including prefixed IDs. Every dumped file's size and
all supplied CRC32/MD5/SHA1/SHA256 fields match a file in its corresponding
archive: zero missing IDs, extra IDs or hash/size mismatches in all four.

SMS and GG database versions match their DATs. The older TG16 and 32X database
export structures have no header/version element, so no database version match
is claimed for those two; exact eligible membership and file-field reconciliation
still passed. All downloads fit existing 16 MiB input / 32 MiB expanded limits.
Raw upstream documents and disposable qualification scripts stay outside this
repository. Original attribution and notices are preserved in published bytes.

## Implementation and validation

Each catalog has an explicit opt-in workflow variable and measured anomaly floors.
The existing shared provider batch, pacing, trust root and metadata version
progression are preserved. Synthetic tests cover reviewed inclusion forms,
missing required controls, cross-system form/document rejection, signed exact-byte
candidates, immutable exports, RSS and authenticated restoration.

`mise run check` passed race tests, vet, formatting, workflow lint and builds.
The synthetic smoke publication passed. Real qualification acquisition used
permissive temporary floors to measure the documents before production floors
were chosen; all measured counts exceed the resulting production floors.
This does not claim ROM import, normalization, emulator/hardware acceptance,
a changed real revision or a scheduled observation.

## Release authorization

The operator authorized this next batch directly after batch 2 completed. This
rollout proceeds through qualification, publication and isolated acceptance in
the current task; no hourly automation was created or resumed, and no scheduled
observation is substituted for the explicit authorization.

Tooling [PR #27](https://github.com/MoonlarkStudios/romd-dat-catalogs/pull/27)
passed CI and merged at `8585a9c5967e28a3bdd139a306c43098ccc3a418`.
Data [PR #7](https://github.com/MoonlarkStudios/romd-dat-data/pull/7) pins both
workflow references to that revision. The four explicit variables are
`NOINTRO_SMS_PUBLISH_ENABLED`, `NOINTRO_GG_PUBLISH_ENABLED`,
`NOINTRO_TG16_PUBLISH_ENABLED` and `NOINTRO_32X_PUBLISH_ENABLED`.

## Production publication

[Run 35368690680](https://github.com/MoonlarkStudios/romd-dat-data/actions/runs/35368690680)
passed the complete workflow, including deployed signature-chain restoration.
Independent discovery authenticated publication **3030**, registry schema **3**,
with all **12 real catalogs healthy**. All four new successful acquisitions are
at `2026-09-18T16:33:33.894751084Z`; their counts, hashes and bytes exactly match
the qualification table. Candidate verification passed for all four using the
original pinned `trust/1.root.json`. All four signed content events are present
at snapshot sequence 70. Immutable assets reference data commit
`ebbf052235f3116e236c178b7153275df51048b0`.

The existing eight catalogs, including PSX and SNES, remain healthy. Synthetic
fixtures are separate from the 12-system production count. Existing GitHub Action
Node-20 deprecation/forced Node-24 execution and upcoming Ubuntu-image notices
remain; this release did not alter those action pins.

## Isolated ROMD acceptance

All four systems passed browser discovery, verified first-import review, explicit
approval, processing and Active source checks using the existing separate
acceptance Docker project and its own database/data volumes. Reviews showed
14 SMS, three GG, eight TG16 and three 32X BIOS entries. Counts and versions
matched the qualification table. Existing application images/frontend were used;
no application source or generated client changed in this task. Concurrent
deployment work in the application checkout was left untouched; the outage test
used a temporary copy of the original Compose definition.

Normal test-account OAuth calls to `/api/dat-subscriptions/check` returned
UpToDate without errors for all four. Active downloads matched signed byte
lengths and SHA-256 hashes. An unreachable publisher URL was then injected only
into the isolated admin. All four checks returned CheckFailed while the same
active DAT IDs and exact downloads remained available. Restoring the normal
publisher returned UpToDate with no errors and zero consecutive failures. A
fresh browser view confirmed Active and Up to date after recovery.

Before and after outage evidence is identical apart from unordered SQL row
order: ten Active DAT versions, 24,205 entries, 24,208 files and ten import jobs
(the six previously accepted catalogs plus these four). No duplicate imports
or replacement active versions resulted from unchanged checks or retries.

Batch 3 is complete within acquisition, signed publication and ROMD catalog
ingestion scope. Local checks, implementation CI and the production workflow
passed. No new backend suite, frontend build, ROM import, normalization,
emulator/hardware launch, NAS deployment, changed real revision or scheduled
observation was exercised. Batch 4 (Saturn, Sega CD and Dreamcast) remains planned
and disabled. No recurring rollout automation was created or resumed.
