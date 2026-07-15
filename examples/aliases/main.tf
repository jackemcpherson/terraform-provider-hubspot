terraform {
  required_providers {
    hubspot = {
      source  = "registry.opentofu.org/jackemcpherson/hubspot"
      version = "= 0.1.0-alpha.1"
    }
  }
}

variable "sandbox_hubspot_access_token" {
  type      = string
  sensitive = true
}

provider "hubspot" {
  alias        = "sandbox"
  access_token = var.sandbox_hubspot_access_token
}

module "sandbox_groups" {
  source = "./modules/property_groups"
  providers = {
    hubspot = hubspot.sandbox
  }
}
