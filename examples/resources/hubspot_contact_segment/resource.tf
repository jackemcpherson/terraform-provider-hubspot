resource "hubspot_contact_segment" "active_customers" {
  name            = "Active customers"
  processing_type = "dynamic"

  filter_groups = [{
    filters = [{
      property      = "ns_onboarding_status"
      operator      = "is_equal_to"
      property_kind = "select"
      value         = "active"
    }]
  }]
}
