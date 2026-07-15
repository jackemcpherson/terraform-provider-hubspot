# Development-only: hubspot_custom_object_schema is not registered in
# v0.1.0-alpha.1. This example requires a later paid-account-qualified release.
terraform {
  required_providers {
    hubspot = {
      source = "registry.opentofu.org/jackemcpherson/hubspot"
    }
  }
}

provider "hubspot" {}

resource "hubspot_custom_object_schema" "widget" {
  name                         = "widget"
  labels                       = { singular = "Widget", plural = "Widgets" }
  primary_display_property     = "name"
  expected_external_properties = ["category"]

  properties = {
    name = {
      label      = "Name"
      type       = "string"
      field_type = "text"
    }
  }
}

resource "hubspot_property" "widget_category" {
  object_type = hubspot_custom_object_schema.widget.object_type_id
  name        = "category"
  label       = "Category"
  group_name  = "widget_information"
  type        = "string"
  field_type  = "text"
}
