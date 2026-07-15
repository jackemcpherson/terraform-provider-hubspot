terraform {
  required_providers {
    hubspot = {
      source  = "registry.opentofu.org/jackemcpherson/hubspot"
      version = "= 0.1.0-alpha.1"
    }
  }
}

provider "hubspot" {}

resource "hubspot_property" "customer_tier" {
  object_type = "contacts"
  name        = "customer_tier"
  label       = "Customer tier"
  group_name  = "contactinformation"
  type        = "enumeration"
  field_type  = "select"

  options = {
    standard = { label = "Standard" }
    premium  = { label = "Premium" }
  }
}
