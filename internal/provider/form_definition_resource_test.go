// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"reflect"
	"slices"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/jackemcpherson/terraform-provider-hubspot/internal/hubspot"
)

func TestFormDefinitionResourceSchemaIsExactExplicitContract(t *testing.T) {
	var response resource.SchemaResponse
	NewFormDefinitionResource().Schema(context.Background(), resource.SchemaRequest{}, &response)

	if response.Schema.Version != 0 {
		t.Fatalf("schema version = %d, want 0", response.Schema.Version)
	}
	assertAttributeNames(t, response.Schema.Attributes, "configuration", "display_options", "field_groups", "id", "name")
	if !response.Schema.Attributes["id"].IsComputed() || response.Schema.Attributes["id"].IsRequired() || response.Schema.Attributes["id"].IsOptional() {
		t.Fatal("id must be computed only")
	}
	for _, name := range []string{"name", "field_groups", "configuration", "display_options"} {
		if !response.Schema.Attributes[name].IsRequired() {
			t.Fatalf("%s must be required", name)
		}
	}

	groups := requireListNestedAttribute(t, response.Schema.Attributes["field_groups"])
	assertAttributeNames(t, groups.NestedObject.Attributes, "fields")
	fields := requireListNestedAttribute(t, groups.NestedObject.Attributes["fields"])
	assertAttributeNames(t, fields.NestedObject.Attributes,
		"blocked_email_domains", "description", "label", "placeholder", "required", "use_default_block_list",
	)

	configuration := requireSingleNestedAttribute(t, response.Schema.Attributes["configuration"])
	assertAttributeNames(t, configuration.Attributes,
		"allow_link_to_reset_known_values", "language", "pre_populate_known_values", "recaptcha_enabled", "thank_you_text",
	)
	display := requireSingleNestedAttribute(t, response.Schema.Attributes["display_options"])
	assertAttributeNames(t, display.Attributes, "style", "submit_button_text")
	style := requireSingleNestedAttribute(t, display.Attributes["style"])
	assertAttributeNames(t, style.Attributes,
		"background_width", "font_family", "help_text_color", "help_text_size", "label_text_color", "label_text_size",
		"legal_consent_text_color", "legal_consent_text_size", "submit_alignment", "submit_color", "submit_font_color", "submit_size",
	)
}

func TestFormDefinitionWriteOwnsNarrowInvariants(t *testing.T) {
	model := formDefinitionResourceModel{
		Name: types.StringValue("Managed form"),
		FieldGroups: formFieldGroupsValue([]formFieldGroupModel{{Fields: formFieldsValue([]formFieldModel{{
			Label: types.StringValue("Email address"), Description: types.StringValue("Contact email"),
			Placeholder: types.StringValue("name@example.com"), Required: types.BoolValue(true),
			BlockedEmailDomains: types.ListValueMust(types.StringType, nil), UseDefaultBlockList: types.BoolValue(true),
		}})}}),
		Configuration: formConfigurationValue(formConfigurationModel{
			Language: types.StringValue("en"), AllowLinkToResetKnownValues: types.BoolValue(false),
			PrePopulateKnownValues: types.BoolValue(false), RecaptchaEnabled: types.BoolValue(true), ThankYouText: types.StringValue("Thank you"),
		}),
		DisplayOptions: formDisplayOptionsValue(formDisplayOptionsModel{
			SubmitButtonText: types.StringValue("Submit"),
			Style: formStyleValue(formStyleModel{
				LabelTextSize: types.StringValue("13px"), LabelTextColor: types.StringValue("#33475b"),
				LegalConsentTextSize: types.StringValue("12px"), LegalConsentTextColor: types.StringValue("#33475b"),
				HelpTextSize: types.StringValue("11px"), HelpTextColor: types.StringValue("#516f90"),
				FontFamily: types.StringValue("Arial, sans-serif"), BackgroundWidth: types.StringValue("100%"),
				SubmitFontColor: types.StringValue("#ffffff"), SubmitAlignment: types.StringValue("left"),
				SubmitSize: types.StringValue("12px 24px"), SubmitColor: types.StringValue("#ff7a59"),
			}),
		}),
	}

	got, diagnostics := formWriteFromModel(context.Background(), model)
	if diagnostics.HasError() {
		t.Fatalf("formWriteFromModel diagnostics: %#v", diagnostics)
	}
	want := hubspot.FormDefinitionWrite{
		FormType: "hubspot", Name: "Managed form",
		FieldGroups: []hubspot.FormFieldGroup{{GroupType: "default_group", RichTextType: "text", Fields: []hubspot.FormField{{
			ObjectTypeID: "0-1", Hidden: false, Name: "email", DependentFields: []hubspot.FormDependentField{},
			Label: "Email address", Description: "Contact email", Placeholder: "name@example.com", FieldType: "email", Required: true,
			Validation: hubspot.FormFieldValidation{BlockedEmailDomains: []string{}, UseDefaultBlockList: true},
		}}}},
		Configuration: hubspot.FormConfiguration{
			CreateNewContactForNewEmail: false, Editable: true, AllowLinkToResetKnownValues: false,
			PostSubmitAction: hubspot.FormPostSubmitAction{Type: "thank_you", Value: "Thank you"},
			Language:         "en", PrePopulateKnownValues: false, Cloneable: true, NotifyContactOwner: false,
			RecaptchaEnabled: true, Archivable: true, NotifyRecipients: []string{},
		},
		DisplayOptions: hubspot.FormDisplayOptions{
			RenderRawHTML: false, Theme: "default_style", SubmitButtonText: "Submit",
			Style: hubspot.FormStyle{
				LabelTextSize: "13px", LabelTextColor: "#33475b", LegalConsentTextSize: "12px", LegalConsentTextColor: "#33475b",
				HelpTextSize: "11px", HelpTextColor: "#516f90", FontFamily: "Arial, sans-serif", BackgroundWidth: "100%",
				SubmitFontColor: "#ffffff", SubmitAlignment: "left", SubmitSize: "12px 24px", SubmitColor: "#ff7a59",
			},
		},
		LegalConsentOptions: hubspot.FormLegalConsentOptions{Type: "none"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("write = %#v, want %#v", got, want)
	}
}

func assertAttributeNames(t *testing.T, attributes map[string]resourceschema.Attribute, want ...string) {
	t.Helper()
	got := make([]string, 0, len(attributes))
	for name := range attributes {
		got = append(got, name)
	}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("attribute names = %#v, want %#v", got, want)
	}
}

func requireListNestedAttribute(t *testing.T, attribute resourceschema.Attribute) resourceschema.ListNestedAttribute {
	t.Helper()
	nested, ok := attribute.(resourceschema.ListNestedAttribute)
	if !ok {
		t.Fatalf("attribute type = %T, want schema.ListNestedAttribute", attribute)
	}
	return nested
}

func requireSingleNestedAttribute(t *testing.T, attribute resourceschema.Attribute) resourceschema.SingleNestedAttribute {
	t.Helper()
	nested, ok := attribute.(resourceschema.SingleNestedAttribute)
	if !ok {
		t.Fatalf("attribute type = %T, want schema.SingleNestedAttribute", attribute)
	}
	return nested
}
