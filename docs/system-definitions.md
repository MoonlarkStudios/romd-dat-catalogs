# Shared system, company, and catalog definitions

The tooling repository owns the editable source, split by category:

```text
definitions/
  systems.json
  companies.json
  catalogs.json
```

Each file is a dictionary, without a repeated ID inside each entry. The loader
assembles all three files into one schema-versioned snapshot. Keys are
stable IDs; catalog `systemId` references our system ID, while
`providerSystemId` selects the provider's distinct identifier. Only the complete
PSX disc catalog is defined initially. Emulator configuration, BIOS requirements,
and ROMD database IDs/user settings do not belong in these definitions.

Go implements acquisition and validation behavior. Data selects an implemented
provider; it cannot supply arbitrary URLs, scripts, or plugins. Redump endpoint
construction, HTTP-only transport, resource limits, and ZIP/XML parsing remain
in its adapter. Count floors are anomaly checks, not proof of completeness.

```sh
mise run check
./bin/distribution validate-definitions --definitions definitions
./bin/publisher --output output --definitions definitions \
  --catalog redump/psx/discs
```

The registry loader rejects duplicate keys at any depth (including escaped
spellings), unknown or incorrectly cased fields, missing/null fields, unsupported
schema versions, invalid IDs, blank labels, dangling system references, unknown
providers/representations, conflicting upstream mappings, and nonpositive count
floors. Input size and nesting are bounded. JSON dictionaries serialize in sorted
key order; no behavior depends on insertion order. Catalog IDs must equal
`provider/systemId/representation`, which also determines the stable DAT path.
The source loader requires all three files, rejects unexpected JSON categories,
and preserves duplicate keys until strict validation rather than silently losing
them during assembly. Each file and the complete snapshot are size-bounded.
The version-1 source format is owned by the loader; the generated snapshot carries
`schemaVersion: 1`. Regions/languages can be added through a reviewed schema
extension when their existing ROMD seeds are migrated. Files from different
published versions are never downloaded and mixed by consumers.
CI validates the committed definition through tests; publication explicitly
validates definitions before restoring or acquiring any source.

Publication selection is separate: the workflow still selects only
`redump/psx/discs` and requires `REDUMP_PSX_PUBLISH_ENABLED=true`. Adding a
catalog definition does not opt it into acquisition or public mirroring.
Unknown selection or malformed definitions fails before acquisition. The old
hardcoded `redump-psx` command has been replaced by `--catalog`.

## Signed distribution copy

Git-backed staging with `--definitions definitions` includes the
same registry in the signed catalog index and in the TUF `reference-data.json` target.
The data repository's Pages artifact also exposes `/reference-data.json` for inspection.
This alias alone is not authenticated: consumers must verify it as a TUF target,
or use the identical definitions from the verified catalog index. Definitions
are not copied into the data branch and metadata refreshes create no data commit.
The workflow and registry come from the same pinned tooling checkout.

Defined systems are not necessarily available catalogs: availability comes from
the published snapshot and its health. The existing synthetic fixture remains
explicitly separate and has no invented real-system mapping.

Restoration preserves the authenticated registry in disposable local state.
Acquisition and signing reject removal or reassignment of established catalog
IDs, provider identifiers, representations, and expected headers. System IDs
cannot be removed. Display labels and validation floors may change through code
review. An intentional identity migration needs a separate reviewed change;
there is no automatic migration override or registry migration engine.

## ROMD consumption

`distribution candidate` still binds the exact catalog ID and expected header.
When definitions are present, it also checks their catalog binding and emits the
shared `systemId` in `candidate.json`. Legacy signed publications remain readable
without inferring a system identity. ROMD can consume a verified pinned registry
and preserve its own database IDs and user settings; a newly published registry
must not automatically rewrite those identities.

This change supplies the shared contract and signed copy. It does not yet change
ROMD's system seeding/enrollment UI or its existing PSX subscription selection.
The live data workflow pins tooling revision
`19f983c0e58d62e7f97ba8bf860c6d1a8c289fde`. The published snapshot is available at
https://moonlarkstudios.github.io/romd-dat-data/reference-data.json; verify its TUF
target before consuming it. Future upgrades must update both caller pins after
review. Public PSX mirroring still requires upstream redistribution qualification.

## Company identities and grouping

The `companies` dictionary starts with the 15 distinct manufacturer labels in
ROMD's existing `src/Romd.Persistence/PlatformSeeder.cs`. This preserves existing
application terminology; it is not a researched legal-entity or corporate-history
registry. No parent-company, successor, developer, or publisher relationships
are inferred from those labels. Initial company aliases are empty; add them only
when their meaning has been reviewed.

Each company has a stable dictionary key, a display `name`, and an `aliases`
array. A system's `manufacturerIds` references those keys (`psx` references
`sony`). The array supports multiple explicitly attributed manufacturers; an
empty array means unspecified, as appropriate for a category such as Arcade.
It does not mean every company manufactured that system. Other relationship
roles can later reference the same company IDs without overloading manufacturer.

Validation rejects unknown/duplicate manufacturer references, invalid company
IDs, missing/null arrays, blank labels, and case-insensitive collisions between
company names and aliases, including aliases that duplicate their own name.
Established company IDs cannot disappear, and changing an existing system's
manufacturer set requires an explicit reviewed migration. Reference order does
not change the relationship. Display names and unambiguous aliases can evolve.
Company data is part of the same signed `reference-data.json` copy and verified index;
it is never maintained separately in the data repository.

Schema version 1 is now published as one signed reference-data snapshot.
It prepares shared seed data and grouping identities; it does not yet
replace ROMD's DB seeders, add a Fetch latest action, or implement grouping UI.
Those consumers should preserve DB IDs/local edits and review changes against
the last-applied shared version.
