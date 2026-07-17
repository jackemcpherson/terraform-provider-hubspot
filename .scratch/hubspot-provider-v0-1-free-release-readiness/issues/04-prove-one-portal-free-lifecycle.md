# Prove deterministic one-portal Free-tier lifecycle coverage

Type: task
Status: claimed
Blocked by: 02, 03

## Question

What black-box acceptance and cleanup contract proves every advertised Free-tier
behavior against the single disposable portal—create, import, refresh, drift,
warnings, archive/absence, destroy/recreate, discovery, and quota-safe cleanup—on
both OpenTofu and Terraform without mutating CRM records or leaving test state?

## Comments

- 2026-07-17 — Post-implementation review reopened this ticket. The initial
  coordinator lacked mutual exclusion and exact post-rebuild verification; its
  quota preflight did not reserve the full ten-property reconstruction capacity;
  and its expanded object-type tracer did not prove the lifecycle on Terraform.
- 2026-07-17 — Final review found the original resolution overstated evidence.
  The ticket remains claimed while portal-scoped coordination, dual-engine
  lifecycle coverage, and archived-configuration cleanup evidence are completed.
