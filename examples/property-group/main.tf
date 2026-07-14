terraform {
  required_providers {
    hubspot = {
      source = "registry.terraform.io/jackemcpherson/hubspot"
    }
  }
}

provider "hubspot" {}

resource "hubspot_property_group" "marketing" {
  object_type = "contacts"
  name        = "marketing"
  label       = "Marketing"
}
