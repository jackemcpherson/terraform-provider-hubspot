resource "hubspot_account_membership" "operator" {
  email              = "operator@example.com"
  first_name         = "Release"
  last_name          = "Operator"
  send_welcome_email = false
  allow_removal      = false
}
