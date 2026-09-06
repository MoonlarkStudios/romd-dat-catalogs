# Shared system and catalog definitions

`definitions/systems.json` in the tooling repository is the single editable
source. It is a schema-versioned dictionary of systems and catalogs. Keys are
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
./bin/distribution validate-definitions --definitions definitions/systems.json
./bin/publisher --output output --definitions definitions/systems.json \
  --catalog redump/psx/discs
```

The registry loader rejects duplicate keys at any depth (including escaped
spellings), unknown or incorrectly cased fields, missing/null fields, unsupported
schema versions, invalid IDs, blank labels, dangling system references, unknown
providers/representations, conflicting upstream mappings, and nonpositive count
floors. Input size and nesting are bounded. JSON dictionaries serialize in sorted
key order; no behavior depends on insertion order. Catalog IDs must equal
`provider/systemId/representation`, which also determines the stable DAT path.
CI validates the committed definition through tests; publication explicitly
validates definitions before restoring or acquiring any source.

Publication selection is separate: the workflow still selects only
`redump/psx/discs` and requires `REDUMP_PSX_PUBLISH_ENABLED=true`. Adding a
catalog definition does not opt it into acquisition or public mirroring.
Unknown selection or malformed definitions fails before acquisition. The old
hardcoded `redump-psx` command has been replaced by `--catalog`.

## Signed distribution copy

Git-backed staging with `--definitions definitions/systems.json` includes the
same registry in the signed catalog index and in the TUF `systems.json` target.
The data repository's Pages artifact also exposes `/systems.json` for inspection.
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
Update both pins in the data repository's caller after this tooling change is
reviewed and merged to publish the definitions. The live workflow is not upgraded
by merely editing the tooling repository. Public PSX mirroring still requires
upstream redistribution qualification.
