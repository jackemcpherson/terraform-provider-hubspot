# Product Definition v0.7.0 Specification

This frozen specification defines the complete v0.7.0 Product definition
surface. Changes require a later release specification.

## Resource Contract

The provider registers `hubspot_product` with these attributes:

| Attribute | Contract |
| --- | --- |
| `id` | Computed canonical numeric HubSpot ID and sole state/import identity. |
| `name` | Required non-blank Product name. |
| `sku` | Required non-blank SKU mapped to `hs_sku`. |
| `description` | Required non-blank Product description. |
| `price` | Required non-negative exact decimal string. |
| `cost` | Optional non-negative exact decimal string. Null is unmanaged and empty clears. |
| `recurring_billing_period` | Optional `P#M`. Null is unmanaged and empty clears. |

Decimal equality is numeric. Equivalent decimal spellings do not cause a PATCH
or replace the configured state spelling. Import adopts supported nonempty
optional values.

## Lifecycle Contract

Create omits `hs_folder`. It never searches by SKU or name. A create response
without an ID fails with exact import guidance. A returned canonical ID is stored
before an exact read-back attempt and permits recovery after an ambiguous result.

Read uses only the state ID. Active `404` triggers an exact archived-ID read.
The same archived identity or absence removes state. Other failures retain state.

Update PATCHes only changed managed properties. It sends no request for semantic
decimal no-ops. Empty optional strings clear their properties.

Destroy archives the exact state ID. It verifies active absence and the same
archived identity. An already archived or absent Product completes idempotently.

Import accepts one canonical numeric ID. It rejects names, SKUs, composite
identifiers, absent Products, and archived Products.

## Deferred Surface

The release excludes tiered pricing, folders, status, tax, terms, URLs, custom
properties, line items, associations, and search-based adoption.

## Publication Boundary

Publication requires the guarded runtime schema and disposable Product probe.
Stop if root placement needs an account-specific value or protected credentials
lack Product read/write scopes.

