terraform {
  required_providers {
    hubspot = {
      source  = "registry.opentofu.org/jackemcpherson/hubspot"
      version = "= 0.1.0-alpha.1"
    }
  }
}

provider "hubspot" {}
