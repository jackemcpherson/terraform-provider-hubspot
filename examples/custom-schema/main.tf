terraform {
  required_providers {
    hubspot = {
      source = "registry.opentofu.org/jackemcpherson/hubspot"
    }
  }
}

provider "hubspot" {}

resource "hubspot_custom_object_schema" "widget" {
  name = "widget"
  labels = { singular = "Widget", plural = "Widgets" }
  primary_display_property = "name"

  properties = {
    name = {
      label = "Name"
      type = "string"
      field_type = "text"
    }
  }
}
