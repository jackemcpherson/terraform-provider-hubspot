terraform {
  required_providers {
    hubspot = {
      source = "registry.opentofu.org/jackemcpherson/hubspot"
    }
  }
}

resource "hubspot_property_group" "marketing" {
  object_type = "contacts"
  name        = "marketing"
  label       = "Marketing"
}
