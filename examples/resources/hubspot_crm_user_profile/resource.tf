resource "hubspot_crm_user_profile" "operator" {
  account_membership_id = hubspot_account_membership.operator.id
  job_title             = "Release Engineer"
  availability_status   = "available"
  time_zone             = "Australia/Melbourne"
  working_hours = [
    {
      days         = "MONDAY_TO_FRIDAY"
      start_minute = 540
      end_minute   = 1020
    }
  ]
}
