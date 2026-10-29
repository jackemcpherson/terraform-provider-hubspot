# 06 Amend the Release Order

Type: task

Status: open

Label: ready-for-agent

Blocked by: None

## What to Build

The release order contains only surfaces a Free portal can qualify.
Association definition is deferred indefinitely, deal pipeline configuration
becomes v0.9.0, and ticket pipeline configuration becomes v0.10.0.

## Acceptance

- Record the amendment as an ADR through the explicit rescheduling path. Name
  the release-order decision it amends.
- Keep one surface per release and confirm that neither pipeline surface
  depends on association definition.
- Keep both pipeline surfaces' scope, ordering rationale, and shared internal
  pipeline implementation unchanged.
- Resolve the conflict between "requires paid-tier qualification" and the
  Free-only delivery contract in the written record.
- State the conditions for restoring association definition: a paid-tier
  qualification portal, a protected environment for it, and an ADR that admits
  paid-tier evidence into the delivery contract.
- Append a correction to the v0.8.0 map's "deferred to v0.9.0" decision that
  points at the ADR, without editing the original decision.
- Land before v0.8.0 release notes mention what comes next.

## Comments
