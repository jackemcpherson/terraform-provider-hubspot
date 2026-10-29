# 01 Patch the Go Toolchain and x/net

Type: task

Status: open

Label: ready-for-agent

Blocked by: None

## What to Build

Weekly protected maintenance stops failing on `govulncheck`. The pinned Go
toolchain moves from 1.26.6 to the newest patched Go 1.26 point release that is
at least 1.26.10, and `golang.org/x/net` moves from v0.58.0 to at least
v0.60.0. The repair lands on `main` as its own provider-only pull request.

## Acceptance

- Raise the toolchain through the single Go version source established by
  provider PR 109. Local checks, CI, and release builds all follow it; no
  parallel copy is edited.
- Leave the module's language version unchanged.
- Limit other module bumps to what tidiness requires.
- Clear GO-2026-6809 and every `net/http`, `net/textproto`, `crypto/tls`, and
  `html/template` finding fixed in 1.26.9, plus GO-2026-6617, 6612, 6611,
  6610, and 6603.
- Pass `make check-security` and `make check` locally before opening the pull
  request.
- Keep the demo pin at `2ef32f7`.
- Merge through `main` with green `Required`.
- Record which findings reach the provider binary itself, and which reach only
  the docs-portal server or provenance tools, for ticket 04.

## Comments
