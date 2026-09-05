# ROMD DAT catalogs

Go companion publisher; independent of ROMD application runtime. Standard
library only; supports macOS and Linux.

- Run `go test -race ./...`, `go vet ./...`, and
  `go build -trimpath -o bin/publisher ./cmd/publisher`. Use `gofmt` for Go files.
  Tests are offline and use synthetic DATs; `fixtures/python-snapshot` is a
  compatibility fixture, not a Python runtime dependency.
- Preserve exact DAT bytes and complete catalogs; no 1G1R filtering here.
- Never treat checksums or unsigned experimental metadata as authentication.
- Publish artifacts before references; retain working artifacts on acquisition
  failures. Keep publication lock files rather than unlinking between owners.
- Do not commit upstream DATs or enable public mirroring until redistribution
  conditions have been established. Do not bundle credentials.
- Report prototype coverage separately from production and ROMD acceptance.
- Do not spawn agents unless the user explicitly requests them.
