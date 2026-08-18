# Official API Contract

This note records the official HubSpot evidence for the v0.8.0 contact segment
definition surface. It separates documented behaviour from facts that the
protected Free portal must prove before implementation.

---

## Source Boundary

The primary machine-readable source is HubSpot's `2026-03` Lists OpenAPI file
at immutable commit `69008de6146c22cf4ded3348c5d0fd96307e5e7e`. That commit
was authored on 17 August 2026. The evidence below also uses HubSpot's current
Lists guide, filter guide, endpoint references, scope catalogue, and product
documentation. [Inspect the official `2026-03` OpenAPI specification][spec].

The human guide calls this surface Lists and Segments. This repository uses
the domain term "contact segment definition" and excludes memberships.
HubSpot also distinguishes the list definition from its record memberships.
[Read the Lists API guide][lists-guide].

## Confirmed Definition Contract

HubSpot documents three processing types:

- `MANUAL` has no background membership processing. Users or API calls manage
  membership.
- `DYNAMIC` continually reevaluates records against filters in the background.
- `SNAPSHOT` evaluates filters at creation, then permits only manual membership
  changes after initial processing.

These values and behaviours come from the
[Lists API processing-type reference][lists-guide]. The create reference lists
the same three accepted values in its `processingType` description.
[Read the create endpoint reference][create].

The contact CRM object type identifier is `0-1`. HubSpot uses that value in its
contact-list create and read examples. [Read the Lists API guide][lists-guide].

HubSpot states that segment creation is available on all products and plans.
Its product catalogue includes both static and dynamically updated CRM segments
in Free tools, subject to Free limits.
[Read the segment creation documentation][create-segments] and
[review the HubSpot product catalogue][catalogue]. This supports Free
qualification, but does not prove that every API processing type works in the
protected portal.

## Confirmed Routes

### Create

Create a definition with `POST /crm/lists/2026-03`. The body requires `name`,
`objectTypeId`, and `processingType`. `filterBranch` is optional in the OpenAPI
schema. A successful response is HTTP 200 and contains a required `list`
object. [Read the create endpoint reference][create] and
[inspect the official OpenAPI specification][spec].

The response model requires `listId`, `listVersion`, `name`, `objectTypeId`,
`processingStatus`, and `processingType`. `listId` is a string without a
documented pattern. `listVersion` is a 32-bit integer. `processingStatus` is an
unconstrained string. Optional fields include `deletedAt` and `filterBranch`.
[Inspect the `PublicObjectList` schema in the official specification][spec].

HubSpot calls `listId` the ILS list ID and directs callers to use it for later
operations. The public schema does not prove that every ID is a positive
decimal string, even though the guide examples use decimal values.
[Read the create and retrieval guidance][lists-guide].

The name must be globally unique among public lists in the portal. The API is
therefore authoritative for name conflicts.
[Read the create endpoint reference][create].

### Read by Exact ID

Read one definition with `GET /crm/lists/2026-03/{listId}`. Send
`includeFilters=true` when the filter definition is required. A successful
response is HTTP 200 with a required `list` object.
[Read the exact-ID endpoint reference][read-id].

The exact-ID operation has only one documented query parameter,
`includeFilters`. It has no `archived`, `deleted`, or `includeDeleted`
parameter. The response schema contains an optional `deletedAt` timestamp, but
neither the endpoint reference nor the guide says that this route can read a
deleted definition. [Read the exact-ID endpoint reference][read-id] and
[inspect the official OpenAPI specification][spec].

The name-based read route also exists. Its existence does not change the ILS
ID identity documented for later operations.
[Read the retrieval guidance][lists-guide].

### Update Name

Rename a definition with
`PUT /crm/lists/2026-03/{listId}/update-list-name`. Supply the new value in the
`listName` query parameter. `includeFilters=true` requests filters in the
returned definition. A successful response is HTTP 200 with `updatedList`.
[Read the name-update endpoint reference][update-name].

The guide applies this operation to a list without limiting its processing
type. The new name must remain globally unique among public portal lists.
[Read the Lists API update guidance][lists-guide].

The machine schema marks `listName` as optional even though the operation needs
a new name. The provider must always send it. A live negative probe should
record the server response when it is missing.
[Inspect the name-update operation in the official specification][spec].

### Update Dynamic Filters

Replace a dynamic definition's complete filter branch with
`PUT /crm/lists/2026-03/{listId}/update-list-filters`. The request body requires
`filterBranch`. A successful response is HTTP 200 with `updatedList`.
[Read the filter-update endpoint reference][update-filters].

HubSpot explicitly limits this operation to `DYNAMIC` lists. The submitted
branch replaces the existing branch, and HubSpot then reevaluates membership.
[Read the Lists API update guidance][lists-guide].

The official surface has no filter-update operation for `SNAPSHOT` lists. The
documented snapshot model fixes filters at creation. Replacing a Terraform
resource when snapshot filters change is consistent with this API boundary.
[Read the processing-type and update guidance][lists-guide].

### Delete and Restore

Delete by exact ILS ID with `DELETE /crm/lists/2026-03/{listId}`. Success is
HTTP 204 with no response body. HubSpot retains the deleted list for up to 90
days, then purges it.
[Read the delete endpoint reference][delete].

Restore by exact ILS ID with
`PUT /crm/lists/2026-03/{listId}/restore`. Success is HTTP 204 with no response
body. HubSpot documents eligibility only for the 90 days after deletion.
[Read the restore endpoint reference][restore].

The delete and restore references do not define idempotent success for an
already deleted, already restored, unknown, or purged ID. They expose only the
success response and a generic default error.
[Inspect both operations in the official specification][spec].

## Confirmed Filter Logic

Every filter definition must use a root `OR` branch with one or more nested
`AND` branches. Filters belong in the nested `AND` branches rather than on the
root. Multiple nested branches form OR groups, and multiple filters in one
branch form an AND group. HubSpot states that this shape is enforced so the
definition can render in its user interface.
[Read the filter structure reference][filter-guide].

An exact request branch has these machine-required fields:

```json
{
  "filterBranchType": "OR",
  "filterBranchOperator": "OR",
  "filters": [],
  "filterBranches": [
    {
      "filterBranchType": "AND",
      "filterBranchOperator": "AND",
      "filterBranches": [],
      "filters": []
    }
  ]
}
```

The `2026-03` OpenAPI models require all four fields on both branch types, but
do not declare `minItems` on either array. The filter guide, rather than the
machine schema, supplies the one-or-more nested-group rule.
[Inspect the branch schemas in the official specification][spec] and
[read the filter structure reference][filter-guide].

Every supported simple predicate is a `PROPERTY` filter with required
`property`, `operation`, and `filterType` fields. The operation is a typed
union in the exact OpenAPI schema.
[Inspect the `PublicPropertyFilter` schema][spec].

The exact operation models document these fields:

- `STRING` requires one string `value`, an `operator`, and
  `includeObjectsWithNoValueSet`.
- `MULTISTRING` requires string `values`, an `operator`, and
  `includeObjectsWithNoValueSet`.
- `ENUMERATION` requires string `values`, an `operator`, and
  `includeObjectsWithNoValueSet`.
- `ALL_PROPERTY` requires an `operator` and
  `includeObjectsWithNoValueSet`, but no value.

[Inspect the four operation schemas in the official specification][spec].

The filter guide documents `IS_EQUAL_TO` and `IS_NOT_EQUAL_TO` for string and
multi-string operations. It documents `IS_KNOWN` and `IS_NOT_KNOWN` for any
property type. [Read the property-operation reference][filter-guide].

`includeObjectsWithNoValueSet=false` rejects records that have no value for
the selected property. That value provides the intended exclusion semantics
for equality and inequality predicates.
[Read the operation-field reference][filter-guide].

The current guide and exact schema do not provide one consistent wire mapping
for a public text-or-select equality predicate. The guide's contact example
uses `MULTISTRING`, `IS_EQUAL_TO`, and one value in `values`. The exact
`ENUMERATION` schema instead describes operators such as `IS_ANY_OF` and
`IS_NONE_OF`. The guide's later enumeration prose names `IS_EQUAL_TO` and
`IS_NOT_EQUAL_TO`, but its example uses `IS_ANY_OF`.
[Read the filter examples and operation reference][filter-guide] and
[inspect the exact operation schemas][spec].

Presence operations have a similar inconsistency. Current examples return
`operationType: ALL_PROPERTY` with `IS_KNOWN`. The exact schema description
names `IS_KNOWN` and `IS_UNKNOWN`, while the filter guide names `IS_KNOWN` and
`IS_NOT_KNOWN`. [Read the list-update example][lists-guide],
[read the property-operation reference][filter-guide], and
[inspect the exact operation schema][spec].

These inconsistencies prevent a frozen text-and-select wire schema from being
derived from official documentation alone.

## Versioning and Processing Evidence

`listVersion` and `processingStatus` are required response fields. The guide
shows `listVersion: 1` and `processingStatus: COMPLETE` in examples.
[Read the create and update examples][lists-guide].

HubSpot documents background processing for dynamic lists and says a dynamic
filter update begins reevaluating memberships. It does not publish an enum for
definition `processingStatus`, terminal failure fields, or a transition model
for these operations. [Read the Lists API guide][lists-guide] and
[inspect the unconstrained response fields][spec].

No create, rename, filter-update, delete, or restore request accepts
`listVersion`. The specification does not define version preconditions,
monotonic increments, or a stale-version error. Polling the exact ID can
observe a returned version, but official sources do not prove how that value
changes after each write. [Inspect the six operations][spec].

The default API error model requires `category`, `correlationId`, and
`message`. It can also include `context`, detailed `errors`, `links`, and
`subCategory`. The operation specifications do not enumerate status codes or
categories for missing, deleted, purged, rejected, or stale list operations.
[Inspect the `Error` model and operation responses][spec].

`pending`, `complete`, `failed`, `stale-version`, `timeout`, `rejected`, and
`ambiguous` can be provider outcome classes. Official sources directly prove
only a returned status string, the example value `COMPLETE`, ordinary HTTP
success, and a generic error shape. All other classifications need live or
hermetic provider evidence.

## Minimum Scopes

The exact-ID read requires `crm.lists.read`. Create requires
`crm.lists.write`. Rename, filter update, delete, and restore offer the normal
OAuth combination of both `crm.lists.read` and `crm.lists.write`.
[Inspect each operation's security requirements][spec].

The provider therefore needs both list scopes for its full lifecycle. HubSpot
describes `crm.lists.read` as permission to view contact-list details and
`crm.lists.write` as permission to create, change, or delete contact lists.
Both scopes are available to all accounts.
[Read HubSpot's current scope catalogue][scopes].

The documented definition routes do not require contact-record read scopes.
Official sources do not prove that the provider can infer a property's remote
operation type without reading contact property schema.

## Supported Plan Decisions

Official sources support these v0.8.0 decisions:

- Use ILS `listId` as the sole remote identity.
- Fix `objectTypeId` to contact type `0-1`.
- Map public processing types to `MANUAL`, `DYNAMIC`, and `SNAPSHOT`.
- Keep memberships, size, folders, permissions, and conversions outside the
  managed definition.
- Use a root OR branch containing AND groups.
- Permit in-place name updates for all processing types.
- Permit in-place filter replacement only for dynamic definitions.
- Replace a snapshot definition when its filters change.
- Delete and restore only by exact ID within the documented retention window.
- Require both `crm.lists.read` and `crm.lists.write` for the full lifecycle.

These decisions follow the cited identity, processing, route, filter, and
scope contracts. Provider-only restrictions can be narrower than the complete
HubSpot surface.

## Unknowns and Live Probe Obligations

Implementation must not start until the protected Free portal records each
item below against `/crm/lists/2026-03`.

1. Prove `MANUAL`, `DYNAMIC`, and `SNAPSHOT` creation in the protected Free
   portal. Record the response status, `listId`, `listVersion`, and
   `processingStatus` for each variant.
2. Create text and enumeration property predicates. Prove the accepted exact
   operation shape for equality, inequality, known, and not-known predicates.
   Read each definition with `includeFilters=true` and freeze the returned
   shape. This test must resolve the `STRING`, `MULTISTRING`, `ENUMERATION`,
   and `ALL_PROPERTY` documentation inconsistencies.
3. Prove whether the required operation type can be chosen without an
   additional contact-property schema scope. If it cannot, the proposed public
   contract and minimum-scope contract are incompatible and must stop.
4. Prove that a deleted definition remains readable by exact ID with
   `includeFilters=true`. Record its HTTP status, body, `deletedAt`, complete
   filter definition, and processing fields. The official route has no deleted
   read parameter and does not promise this behaviour.
5. Restore that same ID within the retention window. Prove exact identity,
   name, processing type, and filter preservation after restore.
6. Delete manual, dynamic, and snapshot definitions separately. Prove whether
   every variant remains restorable and whether restore starts asynchronous
   processing.
7. Record exact responses for a second delete, a second restore, an unknown
   ID, and a purged or otherwise permanently absent ID. Official sources do
   not prove idempotent classifications.
8. Record every observed `processingStatus` value and transition after create,
   dynamic filter replacement, delete, and restore. Prove whether
   `listVersion` changes, when it changes, and whether the filter read-back is
   tied to that version.
9. Prove name updates for all three variants and record the exact duplicate-name
   rejection shape.
10. Attempt a filter update on `SNAPSHOT` and `MANUAL`. Record the exact
    rejection response that supports replacement and prohibition rules.
11. Attempt `MANUAL` with a filter branch and `DYNAMIC` or `SNAPSHOT` without a
    nonempty group. Record whether the server rejects each shape. The create
    schema makes `filterBranch` optional and has no array minimums.
12. Create logically equivalent filters in different orders and read them
    repeatedly. Prove returned ordering, duplicate handling, and safe provider
    canonicalisation. Official sources document boolean logic but not ordering
    stability.
13. Prove the actual ILS ID lexical format. The official schema says only
    `string`, so a positive-decimal import validator is a provider assumption.
14. Record timeout and ambiguous transport outcomes in the live probe harness.
    Accept a write only when exact-ID read-back proves the requested definition
    and identity.

Failure of tombstone readability, same-ID restoration, Free availability, or
text-and-select round-trip compatibility triggers the agreed stop condition.
Those facts are not established by the official documentation reviewed here.

[catalogue]: https://legal.hubspot.com/hubspot-product-and-services-catalog
[create-segments]: https://knowledge.hubspot.com/segments/create-active-or-static-lists
[create]: https://developers.hubspot.com/docs/api-reference/latest/crm/lists/create-list
[delete]: https://developers.hubspot.com/docs/api-reference/latest/crm/lists/delete-list
[filter-guide]: https://developers.hubspot.com/docs/api-reference/latest/crm/lists/list-filters
[lists-guide]: https://developers.hubspot.com/docs/api-reference/latest/crm/lists/guide
[read-id]: https://developers.hubspot.com/docs/api-reference/latest/crm/lists/get-list-listId
[restore]: https://developers.hubspot.com/docs/api-reference/latest/crm/lists/restore-list
[scopes]: https://developers.hubspot.com/docs/apps/developer-platform/build-apps/authentication/scopes
[spec]: https://github.com/HubSpot/HubSpot-public-api-spec-collection/blob/69008de6146c22cf4ded3348c5d0fd96307e5e7e/PublicApiSpecs/CRM/Lists/Rollouts/144891/2026-03/lists.json
[update-filters]: https://developers.hubspot.com/docs/api-reference/latest/crm/lists/update-list-filters
[update-name]: https://developers.hubspot.com/docs/api-reference/latest/crm/lists/update-list-name
