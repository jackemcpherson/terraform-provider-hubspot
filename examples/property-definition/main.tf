terraform {
  required_providers {
    hubspot = {
      source  = "registry.opentofu.org/jackemcpherson/hubspot"
      version = "= 0.1.0-alpha.1"
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
