# Destroy semantics

`terraform destroy` (or removing a resource block and applying) never deletes
CRM configuration outright. HubSpot's schema APIs only support archival:
archived CRM configuration is removed from active use, not absent. It stops
appearing in default listings and stops accepting new values, but the
underlying record HubSpot holds for it is retained. Neither managed resource
in this provider has a restore operation; reversing an archive requires
HubSpot's own UI or API outside this provider.

## hubspot_property_group

Destroy archives the property group (`DELETE` on the group's endpoint, which
HubSpot implements as archival). The provider reads the group back afterward
and only removes it from state once the archived state is confirmed; a
transient, permission, or unverifiable response leaves the resource in state
so the next apply can retry.

HubSpot rejects archiving a group that still owns active properties. When
that happens the provider surfaces the rejection and retains state; archive
or reassign the member properties first.

Archived property groups are not name-reserved: the same `object_type/name`
can be recreated immediately after archival, and HubSpot treats it as a new,
unrelated active group.

**Non-destructive alternative:** `tofu state rm hubspot_property_group.<name>`
(or `terraform state rm ...`) removes the resource from Terraform state
without contacting HubSpot. The group stays active and unmanaged.

## hubspot_property

Destroy archives the property definition (`DELETE` on the definition's
endpoint, which HubSpot implements as archival). The provider reads the
definition back afterward, requesting the archived view, and only removes it
from state once the archived state is confirmed.

Unlike property groups, an archived property definition's name is reserved:
HubSpot rejects creating a new property with the same `object_type/name`
while the archived definition still exists. Recreating a property under that
name requires restoring or permanently deleting the archived definition
through HubSpot directly; this provider does not implement either operation.
Plan for this before destroying and reapplying a property definition under
an unchanged name.

HubSpot-defined and read-only definitions are never destroyed by this
provider because they cannot be created or imported as managed resources in
the first place; see [property lifecycle](property-lifecycle.md).

**Non-destructive alternative:** `tofu state rm hubspot_property.<name>` (or
`terraform state rm ...`) removes the resource from Terraform state without
contacting HubSpot. The definition stays active and unmanaged.
