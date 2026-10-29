# 06 Publish Documentation and Release Metadata

Type: task

Status: resolved

Blocked by: 03, 04, 05

## Acceptance

- Publish generated resource reference and executable example.
- Update surface, permissions, import, drift, destroy, troubleshooting,
  portal, and README documentation.
- Add the dated changelog and `docs/releases/v0.8.0.md` release notes.
- Merge the final demo change first and pin its exact demo `main` merge commit
  in provider maintenance before provider source freeze.

## Comments

- Provider-owned reference, example, surface guide, permissions, import,
  drift, destroy, troubleshooting, portal, README, changelog, and v0.8.0
  release-note changes are drafted. Final generation and source digest remain
  blocked on the exact merged demo commit required by this ticket.
- 2026-10-29: Release train recovery adds a blocker: `release-train-recovery`
  ticket 06 (amend the release order), so release notes name the amended
  v0.9.0 surface. See `.scratch/release-train-recovery/map.md`.
- 2026-10-29: Demo PR #35 merged to demo `main` as
  `7b1531b9ac43883920291bcdcc19dd8c6b6ed1c0`. Provider maintenance now pins
  that exact commit in place of `2ef32f7`, and the workflow pin guard passes.
  No other provider file pinned the demo. Release-train recovery ticket 06 is
  resolved, so the release notes name deal pipeline configuration as v0.9.0
  and keep association definition deferred under ADR 0005.
- 2026-10-29: The changelog entry is `## [0.8.0] - 2026-10-29`; ticket 07
  corrects the date if publication slips. The release notes now carry upgrade
  guidance. `make docs` regenerates the reference and example with no diff.
  `make docs-portal-update` refreshed `docs/portal-source.sha256` from a
  fresh demo clone at `7b1531b`, and a following `make docs-portal` passes.
  `make check`, `make test-hermetic` (`HUBSPOT_DEMO_REPO` at `7b1531b`), and
  `make check-release` pass locally. Live dual-engine qualification remains
  with ticket 07.
