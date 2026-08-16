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

Protected [maintenance run 31871546257](https://github.com/jackemcpherson/terraform-provider-hubspot/actions/runs/31871546257)
validated the runtime schema and disposable Product probe on exact main commit
`c96b591ad714ec7d5d163d8b3268b29e0c1754d3`. It confirmed:

- An omitted `hs_folder` creates a root Product.
- Required property behavior matches the frozen interface.
- Decimal values retain their numeric meaning across HubSpot normalization.
- `P#M` recurrence values round-trip and empty strings clear them.
- Duplicate active SKUs receive an API rejection.
- Exact archived reads return the same generated ID.
- Repeated archival and cleanup are idempotent.

The recurrence property uses a compatible text-backed runtime schema. Repeated
archival returns success on the protected portal. The probe accepts success or
`404` only after it proves active absence and the same archived identity.

The protected membership fixture was subsequently configured. Protected
[maintenance run 31922736244](https://github.com/jackemcpherson/terraform-provider-hubspot/actions/runs/31922736244)
passed on exact main commit
`d003c31ca02beaaff28b72d3f928689c9ff5c4a5`, including the guarded Product
probe, cumulative OpenTofu and Terraform journeys, and terminal owned cleanup.

## References

- [HubSpot Products API guide](https://developers.hubspot.com/docs/api-reference/latest/crm/objects/products/guide)
- [HubSpot object API guide](https://developers.hubspot.com/docs/api-reference/latest/crm/using-object-apis)
- [HubSpot app scopes](https://developers.hubspot.com/docs/apps/developer-platform/build-apps/authentication/scopes)
