# Changelog

All notable changes to this project will be documented in this file.

## [0.2.2] - 2026-10-22

### Added
- Release workflow (`release.yml`): pushing a `v*` tag builds binaries for linux (amd64/arm64), darwin (amd64/arm64) and windows (amd64), generates `SHA256SUMS`, and publishes them to the GitHub Release.
- This CHANGELOG.

### Fixed
- The version reported at startup was stuck at `0.1.0`. It is now injected at build time from the tag (`-X main.Version`); local `go build` reports `dev`.

## [0.2.1] - 2026-10-22

### Security
- Built with Go 1.27.3 (`go` directive in go.mod raised from 1.26.1), fixing GO-2026-6809 (`crypto/x509`).
- `golang.org/x/net` v0.53.0 → v0.60.0, fixing GO-2026-6603, 6610, 6611, 6612, 6617 and 5026. govulncheck reported all of them as reachable via `GetUser` (HTTP/2, IDNA).
- `golang.org/x/text` v0.36.0 → v0.42.0, fixing GO-2026-5970 (infinite loop on invalid input in `x/text/unicode/norm`).
- `golang.org/x/crypto` v0.50.0 → v0.57.0 (required by `x/net` v0.60.0) and `golang.org/x/sys` v0.43.0 → v0.48.0.
- govulncheck now reports 0 vulnerabilities affecting the code.

## [0.2.0] - 2026-05-29

### Fixed
- Free tier compatibility issues (tools that relied on Entra ID P1/P2-only endpoints).

## [0.1.0] - 2026-05-27

### Added
- Initial release: read-only MCP tools for users, groups and reports via Microsoft Graph, with policy enforcement.
