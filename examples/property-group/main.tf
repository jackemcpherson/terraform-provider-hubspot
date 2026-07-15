terraform {
  required_providers {
    hubspot = {
      source  = "registry.opentofu.org/jackemcpherson/hubspot"
      version = "= 0.1.0-alpha.1"
    }
  }
}

provider "hubspot" {}

resource "hubspot_property_group" "marketing" {
  object_type = "contacts"
  name        = "marketing"
  label       = "Marketing"
}
