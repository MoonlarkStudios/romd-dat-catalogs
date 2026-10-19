# Source and publication workflow

Development uses private Forgejo hosting. `next` is the default development
branch; feature pull requests normally target `next`. Existing open pull
requests were retargeted during the 2026-10-14 migration. GitHub retains reviewed
public source and the reusable workflow consumed by the data repository.

| Branch | Purpose |
| --- | --- |
| `next` | Private development; default branch |
| `main` | Reviewed public source published to `MoonlarkStudios/romd-dat-catalogs` |

The import preserved all 33 source branches, three tags, one issue, 37 pull
requests, three releases and 12 release assets. Imported `main` became `next`,
then a separate `main` was created at the original public commit. There was no
history rewrite. Forgejo Actions remain disabled until local CI is qualified.

## Local work

Authenticate `fj` and register an SSH key with your Forgejo host. Substitute
your actual host and organization below; deployment-specific addresses and
operator records belong in private infrastructure documentation.

```sh
fj -H https://forgejo.example.org repo clone organization/romd-dat-catalogs
cd romd-dat-catalogs
git remote add public-github https://github.com/MoonlarkStudios/romd-dat-catalogs.git
fj repo view
```

Work on a feature branch based on current `next`. Open its pull request with
`next` as the base. Continue running the existing development checks from the
repository's `AGENTS.md`; import alone does not qualify a Forgejo CI pipeline.

## Reviewed public source

Forgejo `main` rejects direct pushes, including administrator pushes. Promote
reviewed changes through a squash-merged pull request from `next` to `main`.
Review the entire diff for public suitability. After promotion, merge updated
`main` back into `next` before preparing the next public promotion.

A server-side publisher fetches only `main` into an isolated cache and pushes
an explicit `refs/heads/main` refspec. Its repository-specific GitHub deploy key
cannot write the data repository. Tags, private branches, releases, packages
and signing credentials are not published by this service. Divergent GitHub
history blocks publication; reconcile it in Forgejo rather than force-pushing.
Do not configure Forgejo's native push mirror: Forgejo 15.0.9 can publish tags
even with a `main` branch filter.

## Production DAT publication

`MoonlarkStudios/romd-dat-data` remains the production data and Pages repository.
Its daily/manual caller pins both the reusable GitHub workflow and tooling
checkout to a reviewed full commit. Publishing tooling `main` does not update
those pins or deploy new catalogs. Update both pins together only after the
reader/publication compatibility checks in [the deployment runbook](deployment.md).

Preserve the existing Pages URL, immutable GitHub data commit URLs, trust root,
metadata versions and client verification caches. Keep the offline root key
outside CI. The existing online signing secret stays with the data publisher;
the source migration does not copy or rotate signing keys. Private trials use
independent synthetic fixtures and disposable keys, with separate state from
production.
