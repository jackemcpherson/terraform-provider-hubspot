# Product Definition

`hubspot_product` manages one standard Product through HubSpot's `2026-03`
Products API. It works with HubSpot Free and needs only
`crm.objects.products.read` and `crm.objects.products.write`. It never uses the
legacy `e-commerce` scope.

The resource stores HubSpot's numeric Product `id` as its only state and import
identity. SKU and name are managed values, never lookup or recovery keys. Create
omits `hs_folder` and requires the live guarded preflight to prove that the
account-independent root is accepted before publication. If HubSpot stops
accepting omission, the release stops rather than guessing an account-specific
folder value.

`name`, `sku`, `description`, and `price` are required. Price is an exact
non-negative decimal string. HubSpot may normalize its spelling, such as
`1200.00` to `1200`. The provider compares decimals semantically and preserves
the configured spelling, so normalization causes no PATCH.

`cost` and `recurring_billing_period` are optional managed properties. Null
means unmanaged, while an empty string explicitly clears the remote value.
Recurrence accepts positive whole-month ISO periods in `P#M` form. Import adopts
supported nonempty optional values. Subsequent configuration chooses whether
they remain managed.

PATCH includes only changed managed properties. Refresh and every recovery path
read the exact generated ID. An ambiguous create with a returned ID retains it
for exact read-back. Without an ID, the provider fails with import guidance and
does not search by SKU or name. Active SKU conflicts remain authoritative API
rejections and are never treated as adoption.

Destroy archives the exact ID, verifies active absence, then verifies the same
archived identity. Already archived or absent IDs complete idempotently.
HubSpot may retain archived Products for up to 90 days. This provider offers no
restore or purge operation.

Tiered pricing, folders, status, tax, terms, URLs, custom properties, line
items, associations, and search-based adoption are outside this surface.

## Northstar

The sibling demo's `product-definition` module uses stable map keys and manages
one annual-support Product. Its cumulative journey covers normalization, no-op
planning, controlled drift and repair, exact-ID adoption, external archival,
replacement, and verified teardown under OpenTofu and Terraform.

## References

- [HubSpot Products API guide](https://developers.hubspot.com/docs/api-reference/latest/crm/objects/products/guide)
- [HubSpot Products read endpoint](https://developers.hubspot.com/docs/api-reference/crm-products-v3/basic/get-crm-v3-objects-2026-03-products-productId)
- [HubSpot app scope catalogue](https://developers.hubspot.com/docs/apps/developer-platform/build-apps/authentication/scopes)
