# Free-only v0.1.0 public contract

Status: normative for the Free-only v0.1.0 readiness effort as of 2026-07-17.
This contract supersedes conflicting v0.1.0 scope, account-tier, acceptance, and
release-gate statements in the earlier implementation specification. It does not
weaken its protocol, source-identity, security, or artifact requirements.

## Product boundary

`jackemcpherson/hubspot` v0.1.0 is an OpenTofu-first public beta for declarative,
non-sensitive HubSpot CRM property configuration on a HubSpot Free account. It
does not promise v1 compatibility.

The provider manages CRM configuration, never CRM records or their values. It
requires a sensitive static app access token supplied at the provider boundary,
usually through `HUBSPOT_ACCESS_TOKEN`; tokens never enter configuration state,
diagnostics, examples, reports, or logs.

## Public v0.1.0 surface

The release registers and documents exactly these provider types:

| Type | Contract |
| --- | --- |
| `hubspot_property_group` | One ordinary property group, identified and imported as `object_type/name`. |
| `hubspot_property` | One ordinary, non-sensitive property definition, including its keyed enumeration options where HubSpot Free accepts the chosen property shape. `data_sensitivity` is fixed to `non_sensitive`; `sensitive` and `highly_sensitive` are rejected at plan time. |
| `hubspot_property_definition` | Read-only discovery of one active or archived non-sensitive property definition; it does not read record values. |
| `hubspot_property_definitions` | Read-only discovery collection of active or archived non-sensitive property definitions; it does not read record values. |

The supported Free-tier configuration domain is property groups and ordinary
non-sensitive properties on the standard CRM object types available to the
portal—contacts, companies, deals, and tickets. The release evidence must cover
each object type that the Northstar portal exposes through the Schema API. A
HubSpot rejection caused by an account feature, scope, or quota is a diagnostic;
it must never be hidden as successful convergence.

## Free-account limits and lifecycle

HubSpot Free permits ten custom properties in total. Provider acceptance and the
Northstar demo share one disposable portal and must therefore serialize access,
preflight available capacity, use unique owned names, and leave either verified
absence or a deterministic reconstruction of the Git-authored demo configuration.
Neither flow may create or inspect CRM records.

The public lifecycle contract includes typed validation; create, import, refresh,
authored drift repair, warning-visible destructive changes, archive/confirmed
absence handling, safe destroy, name reuse after verified absence, and
non-sensitive property-definition discovery. Refresh observes remote reality; it
does not mutate it. State uses canonical composite identities and remains portable
between the two provider source identities after the documented
`state replace-provider` operation.

## Explicitly deferred

The v0.1.0 provider binary, generated references, examples, release notes,
capability manifests, and qualification report must not advertise or register:

- `hubspot_pipeline`, including deal, ticket, and custom-object stages;
- `hubspot_custom_object_schema`, associations, bootstrap properties, or deletion
  protection;
- `sensitive` or `highly_sensitive` property definitions or sensitive-definition
  discovery;
- custom-object configuration, association labels, CRM records, OAuth, raw HTTP
  escape hatches, or migration from other HubSpot providers.

The pre-reduction implementation is retained only at
`deferred/v0.2-paid-enterprise-baseline`. A later release may selectively restore
a capability only after defining its product contract and passing its own eligible
account-tier acceptance evidence.

## Engine and distribution compatibility

OpenTofu is primary. v0.1.0 serves protocol 6 and supports OpenTofu and Terraform
`>= 1.8.0`; qualification runs OpenTofu `1.8.8`, `1.10.10`, `1.11.11`, and
`1.12.3`, plus Terraform `1.8.5` and `1.15.8`.

One immutable, signed artifact set is prepared for both:

- `registry.opentofu.org/jackemcpherson/hubspot`
- `registry.terraform.io/jackemcpherson/hubspot`

The candidate must prove installation and state-source portability under both
addresses. OpenTofu remains release-blocking; Terraform parity does not relax any
OpenTofu behavior.

## Qualification and publication readiness

A candidate is ready for Jack to publish only when all of the following are green
for one exact main-reachable commit:

1. The public schema, registration, generated docs, examples, and changelog expose
   only this contract.
2. The deterministic local/CI gate and exact engine matrix pass.
3. One-portal live acceptance proves every named Free-tier lifecycle and cleanup
   behavior, without unavailable-capability skips.
4. The rebuilt Northstar demo stays within quota and rehearses the exact candidate
   through local filesystem mirrors under both registry addresses.
5. The protected release workflow, immutable action pins, scoped secrets, signing,
   SBOM/provenance, reproducible archive comparison, registry configuration, and
   human approval controls are configured and verified.

Publication and public-registry rehearsal are outside this readiness map. They
remain deliberate, post-qualification operations owned by Jack.
