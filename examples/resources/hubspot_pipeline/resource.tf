resource "hubspot_pipeline" "sales" {
  object_type = "deals"
  label       = "Sales pipeline"

  stages = {
    qualification = {
      label    = "Qualification"
      metadata = { probability = "0.1" }
    }
    closed_won = {
      label    = "Closed won"
      metadata = { probability = "1.0" }
    }
  }
}
