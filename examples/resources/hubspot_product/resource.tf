resource "hubspot_product" "support" {
  name                     = "Annual support"
  sku                      = "support-annual"
  description              = "Priority support for one year"
  price                    = "1200.00"
  cost                     = "300.00"
  recurring_billing_period = "P12M"
}
