# Destroy semantics

`terraform destroy` (or removing a resource block and applying) archives the
managed configuration surfaces in this provider rather than purging them.
Archived configuration is removed from active use while the underlying HubSpot
record is retained. No managed resource in this provider has a restore
operation; reversal, where HubSpot supports it at all, is outside the provider.

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

An archived property definition's name can be reused immediately on the stable
`/crm/properties/2026-03` API. Reapplying the same `object_type/name` creates a
new active definition while the older tombstone remains available through
archived discovery. Reuse does not restore the previous definition or migrate
CRM record values.

HubSpot-defined and read-only definitions are never destroyed by this
provider because they cannot be created or imported as managed resources in
the first place; see [property lifecycle](property-lifecycle.md).

**Non-destructive alternative:** `tofu state rm hubspot_property.<name>` (or
`terraform state rm ...`) removes the resource from Terraform state without
contacting HubSpot. The definition stays active and unmanaged.

## hubspot_form_definition

Destroy archives the exact generated form UUID. The provider verifies active
absence and the same UUID in the archived view before removing state. An already
archived or permanently absent identity completes idempotently; an ambiguous
archive retains state unless that terminal evidence is available.

Archival stops new submissions and cannot be restored through this provider.
HubSpot retains the Archived form definition as a tombstone for about three
months. Reapplying declared configuration creates a new generated UUID; it does
not restore the tombstone.

**Non-destructive alternative:** `tofu state rm
hubspot_form_definition.<name>` (or `terraform state rm ...`) leaves the active
form in HubSpot without provider ownership.

## hubspot_file and hubspot_file_folder

Destroy deletes the exact active generated file or empty generated folder. The
provider removes state only after direct read proves active absence. HubSpot may
retain deleted assets in its managed Trash; destroy never claims physical purge,
requests GDPR deletion, or restores a retained asset.

Folders with any active child file or folder are rejected before deletion.
Destroy files first and folders leaf-first. An already absent generated ID
completes idempotently; an ambiguous response retains state for a safe retry.

**Non-destructive alternative:** `tofu state rm hubspot_file.<name>` or `tofu
state rm hubspot_file_folder.<name>` (or the equivalent `terraform` command)
leaves the active asset unmanaged.

## hubspot_account_membership

Destroy is blocked unless `allow_removal = true` has already been reviewed and
applied. The provider rereads the canonical Settings ID, requires the exact
state email, and refuses a membership HubSpot currently reports as Super Admin.
After DELETE it verifies absence by ID, explicit email lookup, and the paginated
membership collection before removing state.

DELETE removes membership from this HubSpot account. It does not delete the
global user identity, deactivate the user elsewhere, or alter a CRM user
profile.

**Non-destructive alternative:** `tofu state rm
hubspot_account_membership.<name>` (or the equivalent `terraform` command)
leaves the account membership active and unmanaged.

## hubspot_crm_user_profile

Destroy performs no remote write. The resource stops managing the selected CRM
profile properties and removes only local state. HubSpot retains job title,
availability, timezone, and working-hours values.

This behavior is the resource lifecycle, so a separate state-removal
alternative has the same remote result. Remove the linked account membership
through `hubspot_account_membership` only when account removal is intended.
