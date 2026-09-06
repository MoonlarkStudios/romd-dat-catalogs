# Standard PlayStation DAT qualification

Reviewed 2026-09-06. Scope: `redump/psx/discs`, shared system `psx`,
complete standard `Sony - PlayStation` DAT. This is metadata, not game files.
Check the registry for the authoritative system ID and validation floors.

## Source and redistribution

- [Official community announcement](https://forum.redump.info/viewtopic.php?p=117559):
  staff announced the move to redump.info on June 20, 2026. The old .org service
  remains separately controlled; no automatic redirect or fallback is assumed.
- [Current About page](https://redump.info/about): submitted metadata is described
  as public domain and available for reuse; DATs are XML exports of this metadata.
- [Disclaimer](https://wiki.redump.info/Redump:_Disclaimer), revision 65922:
  game files are not distributed; trademarks and images remain their owners'.
  This mirror does not claim Redump endorsement or redistribute site imagery.
- [Downloads](https://redump.info/downloads): standard PlayStation DAT is separate
  from serial/version variants and BIOS exports. Only the standard DAT is selected.

These primary sources establish the redistribution basis for this metadata mirror.
The website software's AGPL license is not used as a license for DAT contents.
No upstream inquiry, credentials, scraping, or third-party messages are required.
Robots guidance alone is not treated as a redistribution prohibition. The publisher
makes a bounded daily download with an identifying user agent, respects rate-limit
responses and retry guidance, and retains the last working catalog on failures.

## Observed acquisition

Anonymous `https://redump.info/datfile/psx` returned the complete standard ZIP.
The lowercase path works without redirects; a trailing slash returns a redirect
and must not be added. No alternate-source fallback is configured.

Local sample, not yet evidence of a public publication:

- Version: `2026-09-05 14-23-40`.
- ZIP: 3,300,480 bytes; extracted DAT: 12,833,217 bytes.
- 10,974 disc records and 60,444 track records.
- Extracted SHA-256: `62572360b7abe18df80886283e39782c2c4a325e48c6134fc179d589936fe41d`.

The sample fits the 16 MiB input and 32 MiB document bounds. Record counts are
anomaly checks, not proof that every known disc exists in the upstream database.
The entire exported document is preserved, without regional or 1G1R filtering.

## Distribution and acceptance

The data repository schedules one daily acquisition. Extracted document hashes,
not ZIP bytes, determine content commits and RSS events. The signed index binds
DATs to exact Git commit URLs and hashes. Failed acquisition preserves the prior
artifact and reports failed health. ROMD keeps its installed document locally and
validates a candidate before review and activation.

Qualification does not prove unattended reliability or ROMD activation. Record
those separately from this source and redistribution review. No other Redump
platform, BIOS catalog, restricted export, or No-Intro source is qualified here.
