resource "hubspot_property_group" "marketing" {
  object_type   = "contacts"
  name          = "marketing"
  label         = "Marketing"
  display_order = -1
}
