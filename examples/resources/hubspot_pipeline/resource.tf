# Development-only: hubspot_pipeline is not registered in v0.1.0-alpha.1.
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

resource "hubspot_pipeline" "support" {
  object_type = "tickets"
  label       = "Support pipeline"

  stages = {
    open = {
      label    = "Open"
      metadata = { ticketState = "OPEN" }
    }
    closed = {
      label    = "Closed"
      metadata = { ticketState = "CLOSED" }
    }
  }
}
