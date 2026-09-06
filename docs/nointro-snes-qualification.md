# No-Intro SNES acquisition and publication gate

Investigated 2026-09-06 UTC. Scope: `no-intro/snes/standard`, stable system
`snes`, Datomatic system `49`, expected header
`Nintendo - Super Nintendo Entertainment System`. This continues the earlier
SNES investigation. No ROMD application checkout was changed.

**Status: real anonymous acquisition and local publication demonstrated;
public mirroring remains disabled and unqualified.** Synthetic signed-distribution
checks are separate from real upstream and production acceptance below.

## Official acquisition

The [official guide](https://wiki.no-intro.org/index.php?title=Database_Navigation_Guide)
describes per-system and daily downloads. The selected
[Standard DAT form](https://datomatic.no-intro.org/index.php?page=download&op=dat&s=49)
works with ordinary HTTPS and an anonymous, acquisition-local cookie session:

1. GET the per-system Standard DAT form and check its selected system/name and
   reviewed inclusion controls.
2. POST its current Prepare control and the explicit complete-public selection.
3. Accept only the returned same-origin `/index.php?page=manager&s=49&download=…`
   redirect, then GET that manager page.
4. POST the visible Download control and validate the bounded ZIP/XML response.

This is a public form flow, **not a documented stable API**. The manager currently
contains a hidden submit and a separate visible submit with the same label.
Choosing the first matching label is unreliable. The adapter selects only the
reviewed visible form and rejects ambiguous or changed controls. It never logs
session cookies/download controls, persists them, logs in, runs a browser,
executes JavaScript, solves CAPTCHAs, or substitutes another mirror.
An authentication/challenge or form change is a failed acquisition requiring
review, not an invitation to bypass it. No upstream messages were sent.

The current [robots guidance](https://datomatic.no-intro.org/robots.txt) has an
empty Disallow, a five-second crawl delay, and one request per five seconds.
The adapter serializes requests with at least five seconds between starts,
30-second request timeouts, a two-minute total deadline, and at most four
requests per acquisition. It makes **no automatic retries**. 429/503 terminate
the attempt, honor numeric/date Retry-After (one-minute default), and return
that deadline through the existing signed snapshot. The CLI honors restored
retry deadlines across daily runs. Reuse one adapter per provider/process.
No new scheduler, checkpoint, recovery archive, or GitHub Release is introduced.

Forms are limited to 1 MiB, downloaded input to 16 MiB, and extracted documents
to 32 MiB. Only a single regular DAT member is accepted. Unexpected redirects,
encoding, truncation, ambiguous archives, changed forms, HTML, wrong header/name,
wrong header ID, malformed ROM records, and count-floor violations fail closed.
Acquisition failures preserve the last working catalog via the existing publisher.
Filesystem/configuration errors abort publication rather than replacing state.

## Inclusion policy v1 and completeness limit

The Standard DAT request explicitly chooses default representation, non-merged
collection, and canonical No-Intro names. It includes BIOS, full titles and
preproduction/demo entries, licensed/pirate/other-unlicensed records, original
lifespan and aftermarket records, adult records, physical/digital storage, and
nodump entries. All regions, languages, and both special-flag groups are selected.
The current Aftermarket checkbox submits the literal value `0`; its presence
selects it. No client-side filtering, 1G1R, Retool, or document rewriting occurs.

`nodump` records may omit size and hashes. Preserve that unknown status and the
original bytes; never invent values. Other ROMs need an unsigned numeric size
and correctly shaped CRC32/MD5/SHA1 fields; SHA256 is checked when supplied.
These checks establish structure, not authenticity of game dumps.

The official export measured below contains 4,358 games/ROMs, including 202 names
with the Aftermarket tag, 28 nodump ROMs, and 18 baddump ROMs. The contemporaneous
[download listing](https://datomatic.no-intro.org/index.php?page=download&s=49)
reports `#4375`. **The 17-record discrepancy is unresolved.** The official
[aftermarket guide](https://wiki.no-intro.org/index.php?title=Aftermarket_Guide)
describes private entries, but that alone does not prove the cause here.
Do not describe this export as independently reconciled against the whole
upstream database or enable mirroring until the discrepancy is understood.
Private records, Source Code SNES, Satellaview, and other separate systems are
not silently enrolled. The registry's 4,000-game/ROM floors are anomaly checks,
not completeness evidence or a license to omit entries.

## Redistribution gate

The [current terms](https://datomatic.no-intro.org/stuff/terms.txt) describe project
purpose, disclaimers, and trademarks. The sampled DAT preserves author credits,
trademark and piracy notices. Neither inspected source establishes an explicit
basis for ROMD's public mirror and indefinite Git retention. Public availability,
robots guidance, third-party mirrors, and Redump's separate terms do not supply
that qualification. This is an unresolved qualification, not a claim that
No-Intro forbids redistribution.

Before enabling `NOINTRO_SNES_PUBLISH_ENABLED=true`, record the applicable
redistribution/retention basis, attribution/removal obligations, resolution of
the export-count discrepancy, and approval of this one catalog. Keep the variable
unset or false until then. No upstream DAT, credentials, session material, or
private signing keys are committed. No public upstream data publication was
performed by this change.

## Evidence and acceptance boundaries

Real source evidence, local only:

- Anonymous form download: ZIP 542,753 bytes; DAT 1,911,272 bytes.
- Header ID `49`; version `20260818-050713`; 4,358 games and 4,358 ROMs.
- Extracted SHA256:
  `307fc5258970736d511c05b87d16268a9fbfff38d5bf6aa39dafd500e6500b19`.
- Production Go CLI acquisition/local publication succeeded at
  `2026-09-06T21:05:32.228022Z` and `2026-09-06T21:07:48.529476Z`.
  The second check retained the same artifact and first change timestamp.
  These are repeated same-version observations, not two actual upstream versions
  or evidence of sustained daily reliability.
- The same real document passed local TUF signing, authenticated candidate
  reading, asset/hash verification, and restoration with a disposable test root.
  Git asset resolution was simulated; this proves local signed-path compatibility,
  not a published immutable Git asset. The disposable probe was removed.
- Existing deployed Pages metadata and PSX documents restored successfully using
  the existing `trust/1.root.json`, without resetting the root or versions.

Synthetic/offline evidence:

- Explicit system/provider routing, policy controls, rejected form/redirect
  changes, header ID and ROM structure, nodump preservation, request pacing,
  size/encoding/truncation failures, 429/503 backoff, cancellation, and last-working
  artifact retention.
- Repackaged ZIPs retain document identity and RSS GUID; changed synthetic bytes
  produce new content identity and RSS. Real local Git commits prove stable
  `no-intro/snes/standard.dat` paths and full-commit URL history behavior.
- A hand-authored No-Intro-shaped fixture passes provider validation, export,
  TUF signing/verification, shared system binding, exact-byte candidate reading,
  immutable asset verification, RSS enclosure checks, and authenticated restore.
- `mise install`, `mise run check` (race tests, vet, gofmt, actionlint, three
  builds), and `mise run smoke` all passed on 2026-09-06 UTC.

No real No-Intro DAT has been publicly signed/published, no genuine changed
upstream pair was observed, and ROMD activation was not exercised. Consumers
must use publisher reader code that accepts the `no-intro` provider and its
hyphenated immutable paths; older strict readers can reject the added registry.
The signed contract remains schema 2 and Git format 2; there is no new schema or
ROMD database identity. Availability comes from the published catalog snapshot,
not merely the presence of this gated registry entry.

## Fresh1G1R comparison

Rechecked [Fresh1G1R commit 86e21ac](https://github.com/UnluckyForSome/Fresh1G1R/tree/86e21acf76cffd20051eb5bb0fd9fcbddba3eb0b).
Its No-Intro path uses headless Chromium on the daily pack and the first matching
Download label, then processes data with Retool. This informed investigation of
the form sequence, not ROMD's acquisition implementation or completeness policy.
No third-party code was copied or executed. The official per-system form was
independently exercised instead of downloading/filtering an entire daily pack.

If the form stops working, pause this catalog and retain its last version.
The smallest fallback is an operator-downloaded, all-category official Standard
DAT passed through the same No-Intro validator and publisher/signing/export path
in an explicit reviewed local run. The existing generic manifest path alone does
not enforce provider-specific completeness/ID checks. Do not silently call manual
ingestion automatic support or enable a fallback mirror. No manual upload service
or credentials-based fallback is added in this slice.
