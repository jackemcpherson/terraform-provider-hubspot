terraform {
  required_providers {
    hubspot = {
      source = "registry.opentofu.org/jackemcpherson/hubspot"
    }
  }
}

provider "hubspot" {}
