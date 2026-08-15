module "products" {
  source = "../../terraform-hubspot-demo/modules/product-definition"

  products = {
    annual_support = {
      name                     = "Annual support"
      sku                      = "support-annual"
      description              = "Priority support for one year"
      price                    = "1200.00"
      cost                     = "300.00"
      recurring_billing_period = "P12M"
    }
  }
}
