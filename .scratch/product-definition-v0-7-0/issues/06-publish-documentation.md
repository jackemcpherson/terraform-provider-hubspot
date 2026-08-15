# 06 Publish Documentation and Release Metadata

Type: task

Status: resolved

Blocked by: 03, 04, 05

## Acceptance

- Publish resource, permissions, import, destroy, troubleshooting, README, and
  portal documentation.
- Add the dated changelog and `docs/releases/v0.7.0.md` entry.
- Pin the exact merged demo commit in maintenance.

## Comments

- 2026-08-15: Resource, surface, permissions, import, destroy,
  troubleshooting, portal, README, changelog, and release-note documentation is
  complete. The maintenance pin awaits the merged demo commit.
- 2026-08-15: Demo PR 24 merged as
  `cbec2d72dcfd2bd316c1b213f91a5fc728a4e469`; provider maintenance now pins
  that exact commit.
- 2026-08-15: Protected maintenance exposed a nullable Product-output defect
  during refresh-only reconciliation. Demo PR 25 fixed the defect and merged as
  `3daa63da44464c971ab1ca55ac91345ed1279dd7`. Maintenance now pins that exact
  reviewed commit.

## Answer

The complete v0.7.0 documentation surface is published in the provider branch,
and maintenance consumes the exact reviewed and merged cumulative demo commit.
