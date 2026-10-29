# 01 Patch the Go Toolchain and x/net

Type: task

Status: resolved

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

## Answer

### Changes

- The `go.mod` toolchain line moves from `go1.26.6` to `go1.26.10`. That line
  is the single source from PR 109. The Makefile guard, `setup-go` through
  `go-version-file`, and release builds all read it. No parallel copy changed.
  Go 1.26.10 is the newest Go 1.26 release on `go.dev/dl` as of 2026-10-29.
- `golang.org/x/net` moves from v0.58.0 to v0.62.0. Tidiness also moves
  `golang.org/x/sys` to v0.49.0 and `golang.org/x/text` to v0.43.0.
- `CHANGELOG.md` records the fix under `[Unreleased]`.
- The demo pin stays at `2ef32f7`.

### Acceptance conflicts

- **Language version.** The module `go` directive moves from `1.25.0` to
  `1.26.0`. The "unchanged" criterion can't be met together with the x/net
  criterion. Every qualifying release (v0.60.0, v0.61.0, and v0.62.0)
  declares `go 1.26.0`. With `go 1.25.0`, `go build` fails with "updates to
  go.mod needed", and `go mod tidy` raises the directive again. The new
  directive is still below the 1.26.10 toolchain, so the module adopts no
  newer language features. The maintainer should confirm this outcome.
- **x/net v0.62.0 instead of v0.60.0.** GO-2026-6860 (`x/net/html` quadratic
  text-node coalescing, fixed in v0.62.0) was published after the spec.
  `docs-portal` reaches it through `docsportal.ValidateRenderedHTML`, so
  `make check-security` still fails with v0.60.0. v0.62.0 is the lowest version
  that passes.

### Gate results

All gates ran locally with Go 1.26.10 on 2026-10-29.

- `make check-security`: govulncheck v1.1.4 reports "No vulnerabilities
  found", and zizmor reports no findings.
- `make check` passes in full on the branch after merging `main` at
  `cbf18a1`, which includes the engine-pin move from ticket 02.
  `make check-security` was rerun on that merge and is still clean.
- `make test-hermetic`, `make test-race`, `make fuzz-seeds`, and
  `make check-release` pass. `test-hermetic` used `HUBSPOT_DEMO_REPO`
  pointing at a clean demo clone at `2ef32f7`.

### Reachability before the bump

govulncheck v1.1.4 ran on `c5b3be9` with Go 1.26.6 and x/net v0.58.0. It
reported 14 symbol-reachable findings, one more than the 13 in the spec
because GO-2026-6860 was new. Each `main` package was scanned on its own, and
`./...` attributed the `html/template` findings.

| Binary | Role | Reachable findings |
| --- | --- | --- |
| `.` (`terraform-provider-hubspot`) | Shipped provider binary | 6603, 6605, 6607, 6608, 6610, 6611, 6612, 6613, 6617, 6809 |
| `cmd/docs-portal` | Docs-portal generator | 6603, 6607, 6608, 6611, 6612, 6613, 6617, 6809, 6860 |
| `cmd/docs-portal-serve` | Docs-portal server | 6599, 6600, 6603, 6605, 6607, 6608, 6609, 6610, 6611, 6612, 6613, 6617, 6809 |
| `cmd/northstar-lifecycle` | Maintenance tool | 6603, 6605, 6607, 6608, 6610, 6611, 6612, 6613, 6617, 6809 |
| `cmd/validate-checkout` | Provenance tool | None |

All IDs above use the `GO-2026-` prefix.

Verdict for ticket 04: **the shipped provider binary was affected** by 10
findings. These are GO-2026-6809 (`crypto/x509`, fixed in 1.26.10) and the
following, all fixed in 1.26.9:

- `net/http`: GO-2026-6603, 6605, 6610, 6611, 6612, 6613, and 6617
- `crypto/tls`: GO-2026-6607
- `net/textproto`: GO-2026-6608

The provider does not import x/net directly. It reaches `x/net/http2` through
gRPC and `terraform-plugin-go`, so the x/net side of 6603, 6611, 6612, and
6617 also applies to the provider.

These findings reach only tools and never the provider binary:

- GO-2026-6599 and 6600 (`html/template`): docs-portal server
- GO-2026-6609 (`net/http` Range header limit): docs-portal server
- GO-2026-6860 (`x/net/html`): docs-portal generator

The provenance tool `validate-checkout` had no reachable findings.
