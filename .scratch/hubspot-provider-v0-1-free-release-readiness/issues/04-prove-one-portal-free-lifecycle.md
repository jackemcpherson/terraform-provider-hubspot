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

`make one-portal-free-lifecycle` now serializes the shared portal with an atomic
directory lock, applies the demo's reviewed destroy plan, runs only the
`free_properties` acceptance shard, and always rebuilds then verifies the
Git-authored demo. The wrapper's black-box test proves the successful and failed
acceptance paths, the exact rebuild/verify sequence, and rejection of a concurrent
run.

The quota preflight reserves the complete ten-property capacity needed to rebuild
the demo and verifies per-object headroom for contacts, companies, deals, and
tickets. The standard-object lifecycle tracer now runs on both OpenTofu and
Terraform: create, no-op refresh, drift detection and reconciliation, import,
destroy, and verified absence for each object type. The existing Free lifecycle
and Terraform parity tests cover archive/absence, recreate, discovery, and plan
warnings. All configuration remains prefix-owned; no test creates or reads CRM
records. `make check` passes.

## Comments

- 2026-07-17 — Post-implementation review reopened this ticket. The initial
  coordinator lacked mutual exclusion and exact post-rebuild verification; its
  quota preflight did not reserve the full ten-property reconstruction capacity;
  and its expanded object-type tracer did not prove the lifecycle on Terraform.
