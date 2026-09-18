# Catalog definitions and ROMD identities

ROMD owns its built-in application reference catalog, including system names,
compact labels, companies, regions, languages, ratings, and artwork references.
This repository owns DAT acquisition and signed updates.

`definitions/catalogs.json` maps each stable publisher catalog ID to a ROMD
`systemId`, an upstream provider identity, representation, and validation limits.
These mappings are explicit. Providers must not create or reassign canonical
ROMD identities as a side effect of discovery or ingestion.

`definitions/system-keys.json` is a pinned copy of ROMD's
`reference-data/dist/system-keys.json`. To add a canonical system, author it in
ROMD, regenerate the export, and copy that reviewed export here. A catalog may
only reference a key in this file. This check constrains publisher authoring;
it does not mean older ROMD clients must know every published key. Receivers
show an unresolved system until an explicit server registration resolves it.

## Signed format

New publications use registry schema 3: `schemaVersion` and `catalogs` only.
The signed index includes those definitions and TUF authenticates the
`catalog-definitions.json` target. The reader supports schemas 1 and 2 solely
to verify and restore existing signed publications. Frozen legacy test fixtures
are not authoring sources and must not be edited to add new systems.

```sh
./bin/distribution catalogs --root trust/1.root.json \
  --site https://moonlarkstudios.github.io/romd-dat-data \
  --out /tmp/romd-catalogs
```

The output directory contains verified `catalog.json` and
`catalog-definitions.json`. This command downloads no DAT. The separate
`candidate` command verifies the selected DAT artifact. RSS advertises updates;
TUF and the verified index remain the trust boundary.

Build and test with `mise run check`. Coordinate the immutable reader pin in
ROMD before adopting this tooling revision in the live data-site workflow.
Pushing a tooling review branch does not publish a feed or enable a mirror.

MAME metadata belongs in versioned imported catalogs. Machines, devices, BIOS
dependencies, software lists, items, and parts are distinct records. Map upstream
identities explicitly to ROMD systems; never turn machine names into an exhaustive
application system registry.
