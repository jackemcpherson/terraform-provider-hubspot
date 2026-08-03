resource "hubspot_form_definition" "contact" {
  name = "Contact us"

  field_groups = [{
    fields = [{
      label                  = "Email address"
      description            = "Contact email"
      placeholder            = "name@example.com"
      required               = true
      blocked_email_domains  = []
      use_default_block_list = true
    }]
  }]

  configuration = {
    language                         = "en"
    allow_link_to_reset_known_values = false
    pre_populate_known_values        = false
    recaptcha_enabled                = true
    thank_you_text                   = "Thank you"
  }

  display_options = {
    submit_button_text = "Submit"
    style = {
      label_text_size          = "13px"
      label_text_color         = "#33475b"
      legal_consent_text_size  = "12px"
      legal_consent_text_color = "#33475b"
      help_text_size           = "11px"
      help_text_color          = "#516f90"
      font_family              = "Arial, sans-serif"
      background_width         = "100%"
      submit_font_color        = "#ffffff"
      submit_alignment         = "left"
      submit_size              = "12px 24px"
      submit_color             = "#ff7a59"
    }
  }
}
