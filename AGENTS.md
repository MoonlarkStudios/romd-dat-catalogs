# ROMD DAT catalogs

Python companion publisher; independent of ROMD application runtime.

- Run `python -m unittest discover -s tests -v` in an environment with
  `requirements.txt` installed. Tests are offline and use synthetic DATs.
- Preserve exact DAT bytes and complete catalogs; no 1G1R filtering here.
- Never treat checksums or unsigned experimental metadata as authentication.
- Publish artifacts before references; retain working artifacts on acquisition
  failures. Keep publication lock files rather than unlinking between owners.
- Do not commit upstream DATs or enable public mirroring until redistribution
  conditions have been established. Do not bundle credentials.
- Report prototype coverage separately from production and ROMD acceptance.
- Do not spawn agents unless the user explicitly requests them.
