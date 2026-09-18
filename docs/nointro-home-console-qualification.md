# Home console batch qualification

Recorded 2026-09-18 UTC. Batch 2 targets NES, Genesis and Nintendo 64.
All three passed real acquisition and independent public database reconciliation.
NES exports were initially queued upstream, then became available during the
rollout. Qualification completed before its production definition was added.

## Reviewed representations

The official selector binds Genesis to ID 32 and N64 to ID 24. Genesis uses
standard format 0. N64 explicitly selects format 0 (BigEndian), with exact DAT
header `Nintendo - Nintendo 64 (BigEndian)`. The selector label omits that suffix;
the adapter explicitly maps this one reviewed identity. ByteSwapped format 1 is
not selected. N64DD and Mario no Photopi are separate catalogs and excluded.
These forms expose no collection selector; N64 also has no adult control.
All exposed inclusion categories, regions and languages are included, including
nodump and untagged MIA where available. No local filtering changes the DAT.

NES ID 45's reviewed form labels format 0 Headered and format 1 Headerless.
Prepared form support selects Headered and header_plugin=0 (Ignore). Its verified exact DAT header is
`Nintendo - Nintendo Entertainment System (Headered)`, explicitly mapped from
the unsuffixed selector label. FDS is outside this scope.

## Measured evidence

| System | Version | Entries / files | Bytes | Nodump / baddump | Floors |
| --- | --- | --- | --- | --- | --- |
| genesis | `20260914-071043` | 3,575 / 3,576 | 1,685,879 | 1 / 8 | 3300 / 3300 |
| n64 | `20260918-065107` | 1,262 / 1,262 | 519,966 | 0 / 5 | 1150 / 1150 |

Genesis has one entry containing two files; entries and files are deliberately
counted separately. N64 contains 1,252 `.z64` and ten `.bin` records, including
BIOS. Original document bytes and notices are preserved.

Independent anonymous database XML exports match each DAT version. Exact archive
ID comparison found no missing eligible or extra DAT IDs. Every dumped ROM's size
and all supplied CRC32/MD5/SHA1/SHA256 fields match a file within its corresponding
archive. There were zero mismatches; nodump fields remain unknown.

Genesis: 3,638 public database archive IDs, 63 explicitly marked `dat="0"`, leaving
3,575 eligible IDs. N64: 1,262 public IDs, zero exclusions. The selector's larger
headline counters are not used as exact membership counts.

| System | DAT SHA-256 | Database XML SHA-256 / bytes |
| --- | --- | --- |
| genesis | `f3fe106b07cd8ff3e485e55cf8a6d9763ed38141df10456165ffd5b32d068b3d` | `236478fe0a9676c06cbb9b56c15240f8c0f96d589d1f205e1ce58945e42f67d3` / 6,222,497 |
| n64 | `bac6c59884618137d036b2ee67c5b2694b1ba53d961ce7953c3389b2c9074012` | `fd7a13555f03bfa013cefa2c6da06b4a55f477db43ce85e8d91923c1a2e7fc38` / 2,694,748 |

Genesis excluded archive IDs: `1865`, `2828`, `2829`, `2830`, `2831`, `2832`, `2833`, `2834`, `2835`, `2836`, `2837`, `2869`, `2888`, `2889`, `2890`, `2891`, `2892`, `2893`, `2894`, `2895`, `2896`, `2897`, `2898`, `2899`, `2900`, `2901`, `2902`, `2903`, `2904`, `2905`, `2906`, `2907`, `2908`, `2909`, `2910`, `2911`, `2912`, `2913`, `2914`, `2915`, `2916`, `2917`, `2918`, `3001`, `3002`, `3210`, `3211`, `3212`, `3213`, `3214`, `3215`, `3216`, `3217`, `3218`, `3219`, `3220`, `3221`, `3222`, `3223`, `3224`, `3586`, `3587`, `3588`.


Downloads fit the existing 16 MiB input / 32 MiB expanded bounds. Upstream bytes
and disposable qualification scripts remain outside this tooling repository.
Standing redistribution approval is recorded in `AGENTS.md`.

## Implementation and validation

Explicit per-catalog variables select NES, Genesis and N64 in the existing shared
provider batch. Missing or false variables leave them disabled. Existing root,
metadata versions, catalog identities and acquisition pacing are preserved.
Acquisition failures now print catalog ID and adapter failure code to stderr;
JSON summaries remain on stdout, and no session cookies or raw requests are logged.
This makes form/identity rejection distinguishable from transient request failure.

Synthetic tests cover reviewed forms, missing/duplicate format rejection,
cross-system document rejection, signed candidate exact bytes, immutable export,
RSS and authenticated restoration. `mise run check` passes race tests, formatting,
vet, workflow lint and builds. Final measured-definition local acquisition passed
for all three systems with the hashes recorded here. The initial N64 attempt failed closed
because its header includes BigEndian; the explicit selector/header mapping fixed
that mismatch. NES passed after its upstream exports became available; the gate was not waived.

Production publication and isolated ROMD acceptance are recorded separately below. This qualification does not claim ROM ingestion, normalization,
emulator/hardware acceptance, a real changed revision or a scheduled observation.


## NES qualification after upstream regeneration

The available Headered DAT version `20260918-071113` has 7,660 entries and
7,662 files in 3,920,005 bytes. SHA-256:
`d619acb8dc082ae8b9acd56c116627e8a67211de8f97d59061578c88f4e82300`.
It retains 16 nodump and 18 baddump files; 7,645 names end in `.nes` and one
in `.sav`, with undumped names left unchanged. Floors are 7,000 entries/files.

The independent database export has the same version and is 16,411,568 bytes,
SHA-256 `c5e84145d7dc29b4d50ce074c63b596ec819f565aba6bd4fa21127d8a230952a`.
Of 7,685 public archive IDs, 25 explicitly have `dat="0"`; the remaining 7,660
exactly match the DAT. There are zero missing/extra IDs or dumped-file size/hash
mismatches. The documented limits apply to the compressed response and expanded
XML separately; the database export fits the 32 MiB expanded qualification bound.

Excluded IDs: `0022`, `0023`, `3125`, `3175`, `3284`, `3916`, `3940`, `3952`,
`4835`, `5100`, `5123`, `5131`, `5175`, `5331`, `5332`, `5333`, `5371`, `5543`,
`5812`, `6099`, `6302`, `7217`, `7395`, `7872`, `8105`.

NES has its own explicit publication variable. Synthetic tests reject a
Headerless DAT under this Headered identity and a ByteSwapped DAT under N64's
BigEndian identity. No application-side header stripping or byte-order conversion
is introduced by this catalog release.

## Production rollout

Genesis/N64 tooling [PR #24](https://github.com/MoonlarkStudios/romd-dat-catalogs/pull/24)
passed CI and merged at `097cb20743d43f256d6ee6b5e8cbe7a2b7eafe76`.
Data [PR #5](https://github.com/MoonlarkStudios/romd-dat-data/pull/5) updated both
workflow pins. With the two explicit opt-ins enabled,
[run 35363137044](https://github.com/MoonlarkStudios/romd-dat-data/actions/runs/35363137044)
produced publication **3026**. Independent authenticated discovery and candidate
downloads verified both new catalogs healthy with the qualified hashes; all five
previous real catalogs were also healthy.

NES tooling [PR #25](https://github.com/MoonlarkStudios/romd-dat-catalogs/pull/25)
passed CI and merged at `281faa37e2a654fd3602ca0cf55bb1eae765019a`.
Data [PR #6](https://github.com/MoonlarkStudios/romd-dat-data/pull/6) pins this final
revision. `NOINTRO_NES_PUBLISH_ENABLED=true` was set after qualification.
The first [NES run 35363750534](https://github.com/MoonlarkStudios/romd-dat-data/actions/runs/35363750534)
published metadata **3027** but NES acquisition failed with `form_changed`; the
other seven catalogs remained healthy. Independent candidate verification rejected
NES because it had no healthy artifact. This run was not counted as NES release
success. The exact failing form stage was not retained, so it is not proof of the
same queue response seen locally. A fresh run was dispatched without weakening
the reviewed form policy or importing unsigned qualification state.

The next [run 35364339804](https://github.com/MoonlarkStudios/romd-dat-data/actions/runs/35364339804)
recorded `request_failed` for all seven No-Intro acquisitions at 30-second
intervals. It deployed **3028**, retaining existing artifacts but leaving NES
without one. Its final restoration check failed with HTTP 404 for
`metadata/3027.snapshot.json` while expecting 3028. Subsequent independent
verification authenticated 3028 successfully. This is evidence of the observed
post-deployment verification inconsistency, not a diagnosed CDN root cause.
A further fresh runner was dispatched to recover healthy acquisition. No caches,
trust roots, validation rules or metadata version checks were reset.


[Recovery run 35365045874](https://github.com/MoonlarkStudios/romd-dat-data/actions/runs/35365045874)
passed the full workflow, including deployed restoration. Independently verified
publication **3029**, schema **3**, is healthy for all eight real catalogs.
The three new catalogs record successful acquisition at
`2026-09-18T15:56:00.832380342Z`; their counts, bytes and hashes exactly match
qualification. The immutable data commit is
`25cf4fb7e9a9c4dad633374358743c6dbfad8511`. Independent candidate verification
passed for all three identities, and all three signed change events are present.
The original pinned root and retained verification cache were used throughout.

## Isolated ROMD acceptance

The existing separate acceptance Docker project was reused, with its own
PostgreSQL/data volumes and admin on localhost. The ordinary development stack
was untouched. Existing admin/worker images and the previously built frontend
were used; no application source or generated client changed.

Browser acceptance for each new system verified subscription discovery, the
exact expected DAT header/version and entry/file counts, explicit review approval,
processing completion and an Active source. Reviews identified 32 Genesis, three
N64 and 12 NES BIOS entries. NES's two extra file records and Genesis's one extra
file record were preserved rather than collapsed into entry counts.

Normal test-account OAuth calls to `/api/dat-subscriptions/check` returned
UpToDate with no error for all three. Each active download matched the signed
byte length and SHA-256 above. Genesis/N64 outage acceptance ran while NES was
publishing; NES outage acceptance ran after its activation. In each case, an
unreachable publisher URL was injected only into the isolated admin. Enrollment
checks returned CheckFailed while the active file remained downloadable with
identical bytes. Restoring the real publisher returned UpToDate with no error
and zero consecutive failures.

Final database evidence: six Active DAT versions (three handheld plus three new),
21,297 entries, 21,300 files and six import jobs. NES's before/after outage counts
were identical; Genesis/N64 also retained one version/job each through their
outage test. No duplicate imports or replacement active documents resulted from
unchanged checks or retries.

Batch 2 is complete within catalog acquisition/publication/ingestion scope.
Local `mise run check`, both implementation PR CIs and the final production
workflow passed. No new backend suite, frontend build, ROM import, header/byte-order
normalization, emulator/hardware launch, NAS deployment, changed real revision
or scheduled observation was exercised. Existing GitHub Action Node-20 deprecation
(forced Node-24 execution) and upcoming runner-image notices remain. The transient
acquisition and post-deployment verification failures above are retained in this
record rather than claimed as passing runs.

Batch 3 (Master System, Game Gear, PC Engine/TurboGrafx-16 and 32X) subsequently
completed its [qualification and rollout](nointro-batch-three-qualification.md). No recurring rollout automation was created or resumed.
