# 06 Publish Documentation and Release Metadata

Type: task

Status: open

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
