# Configuration-surface release order

Decision date: 2026-08-02
Inputs: approved surface partition, dependency graph, and delivery contract

## Decision

Each eligible configuration surface receives its own additive minor release.
There are no cross-surface bundles.

| Release | Configuration surface | Ordering rationale |
|---|---|---|
| `v0.2.0` | CRM property schema | Begins with the strongest existing provider and lifecycle baseline, proves the complete delivery contract on familiar ground, and unlocks forms and filtered contact segments. Existing v0.1.x resource state and import compatibility are preserved. |
| `v0.3.0` | Form definition | Follows its property-definition dependency and adds one narrow authored-asset lifecycle before more asynchronous surfaces. |
| `v0.4.0` | Files configuration | Adds an independent, cohesive folder-and-file workflow with hierarchy and ordered teardown contained within the surface. |
| `v0.5.0` | Account membership | Establishes the Settings user identity and admission lifecycle before any CRM profile configuration. |
| `v0.6.0` | CRM user profile configuration | Follows membership and exposes the distinct asynchronous CRM-identity readiness contract honestly. |
| `v0.7.0` | Product definition | Adds the smallest remaining independent surface through a stable, flat, record-like definition lifecycle. |
| `v0.8.0` | Contact segment definition | Its property dependency is already satisfied; this release introduces asynchronous processing, restorable tombstones, and variant-specific filter mutability. |
| `v0.9.0` | Association definition | Directional identity, paired labels, limits, fail-closed references, and hard deletion make it a later, higher-complexity surface. |
| `v0.10.0` | Deal pipeline configuration | Establishes the shared internal default-pipeline lifecycle using the richer probability and record-usage safety contract. |
| `v0.11.0` | Ticket pipeline configuration | Reuses and validates the pipeline implementation through a second real adapter with opaque default discovery, `OPEN`/`CLOSED` stages, and the `tickets` permission model. |

## Bundle decision

No two approved configuration surfaces share a release. The surface partition
already contains every justified cohesive bundle:

- property groups, properties, and options form CRM property schema;
- labels and directional limits form association definition;
- folders and managed files form Files configuration; and
- manual, dynamic, and snapshot variants form contact segment definition.

Further bundling would couple unrelated verification, registry publication,
rollback, and user adoption. Dependency does not imply bundling: forms and
filtered segments consume property identities through ordinary inputs, while
CRM profiles consume membership identity and readiness through an ordinary
input. Deal and ticket pipelines share internal implementation but retain
different public vocabulary, authorization, validation, and safety behavior.

## Release rules

Every line is independently specified, qualified, demonstrated in the
cumulative Northstar configuration, documented, published, and verified from
both registries under the delivery contract in ticket 05. A blocked or failed
surface does not allow a later dependent surface to pass it. An independent
later surface may be rescheduled only through an explicit amendment that keeps
the dependency graph and one-surface release unit intact.

Each new surface is additive and therefore selects the stated minor version.
Compatible corrections use a new patch version; a breaking change to an existing
surface requires a separate migration decision and cannot be hidden in the next
surface release.
