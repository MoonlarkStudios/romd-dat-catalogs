# Home console batch qualification

Recorded 2026-09-18 UTC. Batch 2 targets NES, Genesis and Nintendo 64.
Genesis and N64 passed real acquisition and independent public database
reconciliation. NES is not yet qualified or enabled: official Standard DAT and
database export pages report that the requested file is temporarily unavailable
and queued. Do not represent the entire batch as complete.

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
Prepared form support selects Headered and header_plugin=0 (Ignore). Its final
DAT header, contents and database membership must be verified before adding a
production definition. FDS is outside this scope.

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

Explicit per-catalog variables select Genesis and N64 in the existing shared
provider batch. Missing or false variables leave them disabled. Existing root,
metadata versions, catalog identities and acquisition pacing are preserved.
Acquisition failures now print catalog ID and adapter failure code to stderr;
JSON summaries remain on stdout, and no session cookies or raw requests are logged.
This makes form/identity rejection distinguishable from transient request failure.

Synthetic tests cover reviewed forms, missing/duplicate format rejection,
cross-system document rejection, signed candidate exact bytes, immutable export,
RSS and authenticated restoration. `mise run check` passes race tests, formatting,
vet, workflow lint and builds. Final measured-definition local acquisition passed
for both systems with the hashes above. The initial N64 attempt failed closed
because its header includes BigEndian; the explicit selector/header mapping fixed
that mismatch. NES remains blocked by upstream availability, not a waived gate.

Production publication and isolated ROMD acceptance are recorded separately below
when completed. This qualification does not claim ROM ingestion, normalization,
emulator/hardware acceptance, a real changed revision or a scheduled observation.
