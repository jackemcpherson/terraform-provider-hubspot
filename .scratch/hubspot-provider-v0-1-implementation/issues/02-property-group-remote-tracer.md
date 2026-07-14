# Manage a property group through the real provider boundary

Type: task
Status: resolved
Blocked by: 01

## Outcome

An OpenTofu user can configure an isolated HubSpot client and manage a property
group through create, refresh, update, import, drift repair, and archive. This is
the first full configuration-to-HTTP-to-state tracer and proves the architecture.

## Scope

- Implement environment-first sensitive `access_token` configuration and guarded
  `api_base_url`, including deferred client creation and per-alias isolation.
- Build the plain-Go deep `internal/hubspot` transport, typed client-set seam,
  adaptive instance rate limiting, operation-declared replay, bounded retry,
  exact status handling, error-envelope parsing, body/time limits, safe user agent,
  structured secret-safe events, and deterministic injected test seams.
- Implement the pinned `/2026-03/` property-group client with separate write/read
  models and canonical identity validation.
- Implement the complete `hubspot_property_group` Framework schema and lifecycle.
- Support exact composite import, explicit adoption, observation-only refresh,
  scalar drift, immutable replacement, canonical ambiguous-create recovery,
  archive verification, immediate-name-reuse behavior, and safe nonempty-group
  deletion failure.
- Add unit, HTTP contract, Framework, engine, Free live acceptance, docs, example,
  and changelog coverage.

## Non-goals

- No generic raw API client, token introspection, OAuth, other resources, or CRM
  record endpoint.
- Do not auto-adopt conflicts, restore archived groups, or heal drift on refresh.

## Acceptance

- Token omission/fallback, sensitivity, alias isolation, base-URL validation, and
  redirect rejection pass without leaking token or URL data.
- Transport tests cover every retry status, `Retry-After`, 423 delay, attempt and
  body limits, cancellation, malformed/enveloped errors, replay classifications,
  and safe event fields.
- Property-group create/read/update/import/drift/archive/recreate and rejection
  paths pass HTTP, Framework, engine, and eligible Free live suites.
- Confirmed absence/archive removes state; 403/transient/malformed/ambiguous reads
  retain state and diagnose.
- An ambiguous create recovers only through exact `object_type/group_name`; update
  and archive succeed only after readback verification.
- Documentation states scopes, import syntax, archival, nonempty-group risk, drift,
  and environment-first authentication.

## Authorities

- [Normative specification](../spec.md)
- [Client boundary](../../hubspot-provider-rewrite/issues/07-design-api-client-boundary.md)
- [Resource model](../../hubspot-provider-rewrite/research/04-v0-1-resource-model.md)
- [Lifecycle semantics](../../hubspot-provider-rewrite/research/06-state-lifecycle-semantics.md)

## Answer

Implemented the isolated transport and typed property-group client, including
bounded retries, replay safety, secret-safe events, exact identity validation,
Framework CRUD/import/archive lifecycle, drift observation, and documentation.
Validated with `go test ./...`, `make check`, generated docs, and local OpenTofu
and Terraform engine smoke tests. Free-account live acceptance remains pending
because no eligible live fixture is configured.
