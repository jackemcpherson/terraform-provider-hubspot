// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

//go:build acceptance

package acceptance_test

import "fmt"

// formDefinitionConfig is the single accepted direct-resource fixture shared
// by hermetic and live acceptance. Callers provide only the provider boundary,
// stable remote name, and whether the supported presentation is updated.
func formDefinitionConfig(providerSource, providerConfiguration, name string, updated bool) string {
	label := "Email address"
	description := "Contact email"
	placeholder := "name@example.com"
	required := true
	blockedDomains := "[]"
	useDefaultBlockList := true
	allowReset := false
	prePopulate := false
	recaptcha := true
	thankYou := "Thank you"
	submitText := "Submit"
	labelSize := "13px"
	labelColor := "#33475b"
	legalSize := "12px"
	legalColor := "#33475b"
	helpSize := "11px"
	helpColor := "#516f90"
	fontFamily := "Arial, sans-serif"
	backgroundWidth := "100%"
	submitFontColor := "#ffffff"
	submitAlignment := "left"
	submitSize := "12px 24px"
	submitColor := "#ff7a59"
	if updated {
		label = "Work email"
		description = "Updated contact email"
		placeholder = "work@example.com"
		required = false
		blockedDomains = `["example.com"]`
		useDefaultBlockList = false
		allowReset = true
		prePopulate = true
		recaptcha = false
		thankYou = "Updated thank you"
		submitText = "Send"
		labelSize = "14px"
		labelColor = "#123456"
		legalSize = "13px"
		legalColor = "#234567"
		helpSize = "12px"
		helpColor = "#345678"
		fontFamily = "Helvetica Neue, sans-serif"
		backgroundWidth = "95.5%"
		submitFontColor = "#456789"
		submitAlignment = "center"
		submitSize = "10px 20px"
		submitColor = "#00a4bd"
	}
	return fmt.Sprintf(`
terraform {
  required_providers {
    hubspot = {
      source = %q
    }
  }
}

provider "hubspot" {
%s
}

resource "hubspot_form_definition" "test" {
  name = %q

  field_groups = [{
    fields = [{
      label                  = %q
      description            = %q
      placeholder            = %q
      required               = %t
      blocked_email_domains  = %s
      use_default_block_list = %t
    }]
  }]

  configuration = {
    language                         = "en"
    allow_link_to_reset_known_values = %t
    pre_populate_known_values        = %t
    recaptcha_enabled                = %t
    thank_you_text                   = %q
  }

  display_options = {
    submit_button_text = %q
    style = {
      label_text_size          = %q
      label_text_color         = %q
      legal_consent_text_size  = %q
      legal_consent_text_color = %q
      help_text_size           = %q
      help_text_color          = %q
      font_family              = %q
      background_width         = %q
      submit_font_color        = %q
      submit_alignment         = %q
      submit_size              = %q
      submit_color             = %q
    }
  }
}
`, providerSource, providerConfiguration, name, label, description, placeholder, required, blockedDomains, useDefaultBlockList,
		allowReset, prePopulate, recaptcha, thankYou, submitText, labelSize, labelColor, legalSize, legalColor, helpSize, helpColor,
		fontFamily, backgroundWidth, submitFontColor, submitAlignment, submitSize, submitColor)
}
