terraform {
  required_providers {
    hubspot = {
      source = "registry.opentofu.org/jackemcpherson/hubspot"
    }
  }
}

provider "hubspot" {}

data "hubspot_property_definition" "email" {
  object_type = "contacts"
  name        = "email"
}

data "hubspot_property_definitions" "contacts" {
  object_type = "contacts"
}
