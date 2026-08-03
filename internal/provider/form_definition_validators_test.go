// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestFormDefinitionStringValidatorsPinNarrowContract(t *testing.T) {
	tests := []struct {
		name      string
		validator validator.String
		valid     []string
		invalid   []string
	}{
		{name: "required text", validator: formRequiredTextValidator{kind: "form name"}, valid: []string{"Managed form"}, invalid: []string{"", " ", " padded"}},
		{name: "language", validator: formLanguageValidator{}, valid: []string{"en"}, invalid: []string{"fr", "EN", "en-US"}},
		{name: "alignment", validator: formAlignmentValidator{}, valid: []string{"left", "center"}, invalid: []string{"right", "justify", " center"}},
		{name: "color", validator: formColorValidator{}, valid: []string{"#00a4bd", "#FFFFFF"}, invalid: []string{"#fff", "00a4bd", "rgba(0,0,0,1)"}},
		{name: "font", validator: formFontFamilyValidator{}, valid: []string{"Arial, sans-serif", "Helvetica Neue, sans-serif"}, invalid: []string{"", " Arial", "Arial; color: red", `"Arial", sans-serif`}},
		{name: "pixel size", validator: formPixelSizeValidator{}, valid: []string{"1px", "13px", "1.5px"}, invalid: []string{"0px", "-1px", "13em", "13 px"}},
		{name: "percentage width", validator: formPercentageSizeValidator{}, valid: []string{"1%", "100%", "99.5%"}, invalid: []string{"0%", "100px", "100 %"}},
		{name: "submit padding", validator: formSubmitSizeValidator{}, valid: []string{"12px 24px", "1.5px 2px"}, invalid: []string{"12px", "0px 24px", "12px 24px 12px", "12em 24px"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, value := range test.valid {
				if diagnostics := validateFormString(test.validator, value); diagnostics.HasError() {
					t.Errorf("valid value %q rejected: %#v", value, diagnostics)
				}
			}
			for _, value := range test.invalid {
				if diagnostics := validateFormString(test.validator, value); !diagnostics.HasError() {
					t.Errorf("invalid value %q accepted", value)
				}
			}
		})
	}
}

func TestFormBlockedEmailDomainValidator(t *testing.T) {
	validatorUnderTest := formBlockedEmailDomainsValidator{}
	valid := [][]string{{}, {"example.com"}, {"mail.example.co.uk", "xn--bcher-kva.example"}}
	invalid := [][]string{{"https://example.com"}, {"user@example.com"}, {"Example.com"}, {"-bad.example"}, {"example.com/path"}, {" example.com"}}
	for _, values := range valid {
		if diagnostics := validateFormDomains(validatorUnderTest, values); diagnostics.HasError() {
			t.Errorf("valid domains %#v rejected: %#v", values, diagnostics)
		}
	}
	for _, values := range invalid {
		if diagnostics := validateFormDomains(validatorUnderTest, values); !diagnostics.HasError() {
			t.Errorf("invalid domains %#v accepted", values)
		}
	}
}

func validateFormString(validatorUnderTest validator.String, value string) diag.Diagnostics {
	response := validator.StringResponse{}
	validatorUnderTest.ValidateString(context.Background(), validator.StringRequest{
		Path: path.Root("test"), ConfigValue: types.StringValue(value),
	}, &response)
	return response.Diagnostics
}

func validateFormDomains(validatorUnderTest validator.List, values []string) diag.Diagnostics {
	elements := make([]attr.Value, len(values))
	for index, value := range values {
		elements[index] = types.StringValue(value)
	}
	response := validator.ListResponse{}
	validatorUnderTest.ValidateList(context.Background(), validator.ListRequest{
		Path: path.Root("blocked_email_domains"), ConfigValue: types.ListValueMust(types.StringType, elements),
	}, &response)
	return response.Diagnostics
}
