# ADR 0005: Defer Association Definition From the Release Order

- Status: Accepted
- Date: 2026-10-29
- Amends: Configuration-surface release order, 2026-08-02
  (`.scratch/hubspot-free-configuration-coverage/research/06-surface-release-order.md`)

## Context

The configuration-surface release order of 2026-08-02 numbers association
definition as v0.9.0, deal pipeline configuration as v0.10.0, and ticket
pipeline configuration as v0.11.0. Its release rules require every line to be
qualified under the delivery contract, and they allow an independent later
surface to be rescheduled only through an explicit amendment. That amendment
must keep the dependency graph and the one-surface release unit intact.

The delivery contract qualifies each release on a disposable HubSpot Free
portal. The Free-only v0.1.0 contract already deferred association labels.
It allows a deferred capability back only after that capability passes
acceptance on an account tier that can actually qualify it. The v0.8.0 map
later recorded that association definitions require paid-tier qualification.
Those two records conflict. A surface that only a paid portal can qualify
cannot pass a Free-only delivery contract, so v0.9.0 described work that could
not ship.

## Decision

Remove association definition from the numbered release order and defer it
indefinitely. Renumber the remaining surfaces:

| Release | Configuration surface |
|---|---|
| `v0.9.0` | Deal pipeline configuration |
| `v0.10.0` | Ticket pipeline configuration |

This amendment changes only the release numbers. The scope and ordering
rationale of both pipeline surfaces stay as recorded in the 2026-08-02
decision. Deal pipeline configuration still establishes the shared internal
default-pipeline implementation. Ticket pipeline configuration still follows
it and reuses that implementation as a second adapter.

Each release still contains one configuration surface. The dependency graph
is intact. Neither pipeline surface depends on association definition. Deal
pipeline configuration depends on no unreleased surface. Ticket pipeline
configuration depends only on the deal pipeline implementation, which still
ships first. No other surface depends on association definition, so removing
it does not let a dependent surface pass a blocked one.

The Free-only delivery contract takes precedence over the earlier statement
that association definition would be the next release. Until the contract
changes, no session plans, specifies, or qualifies association definition
work against a Free portal.

## Restoring Association Definition

Association definition can return to the release order only when all of these
exist:

1. A paid-tier HubSpot portal that can qualify the surface.
2. A protected GitHub environment for that portal, separate from the Free
   portal's environment.
3. An ADR that admits paid-tier evidence into the delivery contract and
   defines how that evidence qualifies a release.

Once all three exist, a further amendment can give association definition a
release number. Like this one, it must keep one surface per release and the
dependency graph intact.

## Consequences

The next slice after v0.8.0 is deal pipeline configuration, and it can start
as soon as v0.8.0 publishes. Release notes and roadmaps that mention later
releases must name deal pipeline configuration as v0.9.0. They must list
association definition as deferred, not scheduled.
