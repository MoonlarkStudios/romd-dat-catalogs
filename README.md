# ROMD DAT catalogs

Companion publisher for ROMD's planned one-click No-Intro and Redump DAT
subscriptions. Complete upstream catalogs come first; future 1G1R filtering
belongs in ROMD's library layer.

**Current status: live signed synthetic publisher.**
[Public RSS](https://moonlarkstudios.github.io/romd-dat-catalogs/feed.xml) ·
[Deployment runbook](docs/deployment.md)

The local publisher still emits explicitly unsigned intermediate state. The
distribution command wraps it with TUF-authenticated catalog/feed targets for
GitHub Releases and Pages. A [Redump acquisition library](docs/redump-adapter.md)
is tested against synthetic servers but is not wired into live polling. No
public upstream DAT mirrors or ROMD integration are implemented. Do not enable production
AutoApply against synthetic catalogs.

## Run

Use mise on macOS or Linux (POSIX file locking required). `.mise.toml` pins
Go 1.27.1, actionlint, and ShellCheck for development and CI. `go.mod` records
the minimum language version; `GOTOOLCHAIN=local` prevents silent toolchain
switching. Each CLI builds into a standalone executable:

```sh
mise trust
mise install
mise run check
mise run smoke
```

CLI flags precede the manifest argument. The example hostname is deliberately non-routable. For local serving tests,
the objects remain accessible under `output/`; HTTPS hosting is a later step.

The manifest is a trusted local registry of acquisition attempts. Each entry
contains a stable `catalogId`, exact expected DAT header name, public source
URL, and either a path to a DAT/single-DAT ZIP or a `failure` marker. Paths
are relative to the manifest. Never put credentials in URLs or manifests.
This prototype's limits are intentionally suitable for small fixtures, not
yet qualified for the entire upstream daily pack.

## What works

- Exact extracted-document hashes, preserving upstream bytes and notices.
- Bounded DAT/ZIP handling, safe XML parsing, expected-identity checks.
- Per-catalog failure state retaining the prior working artifact.
- Immutable content-addressed artifacts, RSS, and complete catalog snapshots.
- Stable feed event identities; an upstream restoration can reuse old content
  with a new event. Feed truncation does not limit index-based catch-up.
- Atomic replacement of `current.json` after its objects are written and
  verified, with a persistent OS lock serializing local publisher processes.
- Rejected-version exclusion in a small reference catch-up function.

The Go implementation resumes existing Python prototype snapshots; a committed
synthetic snapshot from Python commit `02d300c` exercises compatibility without
a Python runtime. JSON/XML serialization can differ while DAT hashes and event
identities remain stable. DAT comparison remains byte-based: semantic handling
of header-only or formatting-only changes is a separate future feature.

XML validation reads tokens without a full document tree, although the bounded
original document and a set of game names remain in memory. UTF-8 XML is
supported; other declared encodings fail closed pending explicit qualification.
The full archive and retained-object verification paths still need large-catalog
qualification. To measure the synthetic 10,000-game parser workload:

```sh
mise run bench
```

This benchmark is a reproducible measurement tool, not a full upstream run or
evidence of a speedup over Python.

`current.json` points to an exact snapshot by path, length, and SHA-256 and
explicitly declares `trust: unsigned-development`. The snapshot references
an immutable RSS file and every current catalog. RSS is a notification hint;
the complete index is the basis for reconciliation. This local intermediate state
has no mutable `feed.xml`; signed distribution adds the stable RSS alias and
TUF-authenticated targets described in the deployment runbook.

Publisher tests exercise local process interruption before pointer commit;
distribution tests exercise signed metadata and client verification over local
HTTP fixtures. They do not prove host power-loss durability, GitHub deployment,
upstream authenticity, or ROMD activation safety. DAT checks here are structural;
full ROM/hash semantics and anomaly policies remain pending.

## Next acceptance gates

1. Complete an operational recovery drill using
   [the deployment runbook](docs/deployment.md). Initial and ordinary live
   synthetic publications passed; unattended outage recovery remains separate.
   The signed catalog format remains experimental.
2. Qualify acquisition and public redistribution conditions for each upstream.
   Preserve system/variant distinctions and explicit inclusion settings.
3. Add upstream adapters, qualify retention at full catalog scale, and collect
   actual unattended change/outage observations. No GitHub API token required by readers.
4. Integrate with ROMD's durable jobs, CAS, exact-version diff, validated
   activation, curation preservation, history/rollback, and library convergence.

Only hand-authored synthetic DATs belong in current fixtures. No upstream
content is distributed by this repository. No-Intro and Redump remain the
authorities for their catalogs; this project is not affiliated with them.
