# Private-app scopes for the Free demo

Date: 2026-07-17

## Required configuration scopes

The demo creates, reads, updates, imports, and archives non-sensitive property
groups and property definitions for contacts, companies, deals, and tickets.
Configure the private app with:

- `crm.schemas.contacts.read`
- `crm.schemas.contacts.write`
- `crm.schemas.companies.read`
- `crm.schemas.companies.write`
- `crm.schemas.deals.read`
- `crm.schemas.deals.write`
- `tickets`

HubSpot's scope catalogue describes the schema scopes as access to property
settings for the corresponding standard object. The Properties API and its
create-property operation list those schema scopes (and `tickets`) as accepted
authorizations. The demo intentionally does not manage sensitive data, so it
does not need sensitive-data scopes.

Sources:

- <https://developers.hubspot.com/docs/apps/developer-platform/build-apps/authentication/scopes>
- <https://developers.hubspot.com/docs/api-reference/latest/crm/properties/guide>
- <https://developers.hubspot.com/docs/api-reference/latest/crm/properties/create-property>

## Custom-property quota preflight

The rehearsal also calls `GET /crm/limits/2026-03/custom-properties` before it
creates configuration. HubSpot's English reference page currently renders the
Required Scopes area without a scope list, so it does not publish an exact
scope-to-endpoint mapping.

The endpoint's generated reference lists CRM object scopes as accepted
authorizations. The least-privileged practical addition is
`crm.objects.contacts.read`; it is distinct from the schema scopes and should
be treated as required by this preflight until a live request confirms it.
Adding the company and deal object-read scopes too is a conservative fallback,
not an established requirement. This conclusion is an inference, not a
published HubSpot guarantee.

Source:

- <https://developers.hubspot.com/docs/api-reference/latest/crm/limits-tracking/get-custom-properties>
