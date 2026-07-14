terraform {
  required_providers {
    hubspot = {
      source = "registry.terraform.io/jackemcpherson/hubspot"
    }
  }
}

provider "hubspot" {}
