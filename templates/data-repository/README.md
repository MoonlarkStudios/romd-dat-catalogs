# ROMD DAT data

Complete, uncompressed catalogs live at stable platform paths, such as
`redump/psx/discs.dat`. The signed latest index and RSS are served by this
repository's GitHub Pages site. RSS is a notification hint; ROMD verifies the
signed metadata and exact document bytes before reviewing an update.

The publishing implementation and platform definitions live in
[romd-dat-catalogs](https://github.com/MoonlarkStudios/romd-dat-catalogs).
The small caller workflow pins both that implementation and its tooling checkout
to the same commit. Update both pins together. Configure Pages for Actions and
follow the tooling repository's deployment runbook before enabling the schedule.
The existing root/online keys must be preserved when migrating an existing site.

Only extracted DAT changes produce data commits. Metadata renewal uses Pages
artifacts, without data-branch commits. Git retains history: never force-push,
rewrite commits, enable Git LFS for DATs, or trim history while signed indexes
may reference it. ROMD retains its installed document locally.

Public upstream mirroring is disabled until redistribution is qualified. The
initial publication can use synthetic fixtures. No credentials or private keys
belong in this repository.
