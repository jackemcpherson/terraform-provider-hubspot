# Prove deterministic one-portal Free-tier lifecycle coverage

Type: task
Status: resolved
Blocked by: 02, 03

## Question

What black-box acceptance and cleanup contract proves every advertised Free-tier
behavior against the single disposable portal—create, import, refresh, drift,
warnings, archive/absence, destroy/recreate, discovery, and quota-safe cleanup—on
both OpenTofu and Terraform without mutating CRM records or leaving test state?

## Answer

`make one-portal-free-lifecycle` serializes the disposable portal across local
checkouts and CI, reserves full demo-rebuild capacity, runs the Free shard, and
always rebuilds then plans the Git-authored demo under state locking. The
black-box wrapper test proves successful and failed paths plus concurrent-run
rejection.

Every named Free lifecycle now has live OpenTofu and Terraform evidence: typed
validation, create, import, refresh, drift repair, destructive-plan warnings,
archive/confirmed absence, recreate, both discovery sources, and safe destroy
across contacts, companies, deals, and tickets. Final destruction independently
proves both active absence and the HubSpot archive terminal state.

HubSpot exposes archive, but no public permanent-purge or restore API for these
objects; the documented terminal invariant is therefore no active prefix-owned
configuration, verified archival, unique prefixes, and deterministic demo
reconstruction—not an unavailable physical purge. See
[archived property terminal-state research](../research/archived-property-terminal-state.md).
`make check` passes without creating or reading CRM records.

## Comments

- 2026-07-17 — Post-implementation review reopened this ticket. The initial
  coordinator lacked mutual exclusion and exact post-rebuild verification; its
  quota preflight did not reserve the full ten-property reconstruction capacity;
  and its expanded object-type tracer did not prove the lifecycle on Terraform.
- 2026-07-17 — Final review found the original resolution overstated evidence.
  The ticket remains claimed while portal-scoped coordination, dual-engine
  lifecycle coverage, and archived-configuration cleanup evidence are completed.
