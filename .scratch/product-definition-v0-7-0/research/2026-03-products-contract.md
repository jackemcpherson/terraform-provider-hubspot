# Products 2026-03 Contract Research

This research records the evidence used to freeze `hubspot_product` for v0.7.0.

## Official Contract

HubSpot documents standard Products at
`POST /crm/objects/2026-03/products`. Exact reads and updates use the generated
product ID. Archival uses `DELETE` on that same identity.

The documented standard Product properties are:

- `name`.
- `hs_sku`.
- `description`.
- `price`.
- `hs_cost_of_goods_sold`.
- `hs_recurring_billing_period` in `P#M` form.

The Product guide omits `hs_folder` from its standard create example. The generic
object guide lists `hs_folder` among Product create properties. The v0.7.0 client
therefore omits `hs_folder` and the guarded live probe must prove root placement
before publication. Publication must stop if HubSpot requires an account-specific
folder value.

The exact least-privilege scopes are `crm.objects.products.read` and
`crm.objects.products.write`. The legacy `e-commerce` scope is not used.

Tiered pricing requires Revenue Hub. It is outside the standard Free-compatible
surface.

## Runtime Evidence

No local HubSpot credential was available on 2026-08-15. The runtime property
schema and disposable-product probe remain guarded acceptance preflight work.
The probe must confirm:

- An omitted `hs_folder` creates a root Product.
- Required property behavior matches the frozen interface.
- HubSpot normalises decimal strings without changing their numeric meaning.
- `P#M` recurrence values round-trip and empty strings clear them.
- Duplicate active SKUs receive an API rejection.
- Exact archived reads return the same generated ID.
- Repeated archival and cleanup are idempotent.

## References

- [HubSpot Products API guide](https://developers.hubspot.com/docs/api-reference/latest/crm/objects/products/guide)
- [HubSpot object API guide](https://developers.hubspot.com/docs/api-reference/latest/crm/using-object-apis)
- [HubSpot app scopes](https://developers.hubspot.com/docs/apps/developer-platform/build-apps/authentication/scopes)

