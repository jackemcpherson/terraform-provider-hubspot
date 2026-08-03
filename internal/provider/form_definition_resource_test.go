// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"reflect"
	"slices"
	"strings"
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

func TestFormDefinitionRejectsEveryUnsupportedOwnedShape(t *testing.T) {
	tests := []struct {
		name     string
		category string
		mutate   func(*hubspot.FormDefinition)
	}{
		{name: "form type", category: "form type", mutate: func(form *hubspot.FormDefinition) { form.FormType = "captured" }},
		{name: "additional group", category: "field group", mutate: func(form *hubspot.FormDefinition) { form.FieldGroups = append(form.FieldGroups, form.FieldGroups[0]) }},
		{name: "group type", category: "field group", mutate: func(form *hubspot.FormDefinition) { form.FieldGroups[0].GroupType = "rich_text" }},
		{name: "rich text", category: "field group", mutate: func(form *hubspot.FormDefinition) { form.FieldGroups[0].RichText = "unsafe" }},
		{name: "additional field", category: "email field", mutate: func(form *hubspot.FormDefinition) {
			form.FieldGroups[0].Fields = append(form.FieldGroups[0].Fields, form.FieldGroups[0].Fields[0])
		}},
		{name: "object type", category: "email field", mutate: func(form *hubspot.FormDefinition) { form.FieldGroups[0].Fields[0].ObjectTypeID = "0-2" }},
		{name: "property name", category: "email field", mutate: func(form *hubspot.FormDefinition) { form.FieldGroups[0].Fields[0].Name = "firstname" }},
		{name: "field type", category: "email field", mutate: func(form *hubspot.FormDefinition) { form.FieldGroups[0].Fields[0].FieldType = "text" }},
		{name: "hidden field", category: "email field", mutate: func(form *hubspot.FormDefinition) { form.FieldGroups[0].Fields[0].Hidden = true }},
		{name: "dependent field", category: "dependent field", mutate: func(form *hubspot.FormDefinition) {
			form.FieldGroups[0].Fields[0].DependentFields = []hubspot.FormDependentField{{}}
		}},
		{name: "default value", category: "default value", mutate: func(form *hubspot.FormDefinition) { form.FieldGroups[0].Fields[0].DefaultValue = "unsafe" }},
		{name: "consent", category: "consent", mutate: func(form *hubspot.FormDefinition) { form.LegalConsentOptions.Type = "explicit_consent_to_process" }},
		{name: "notification recipient", category: "notification", mutate: func(form *hubspot.FormDefinition) {
			form.Configuration.NotifyRecipients = []string{"private@example.com"}
		}},
		{name: "owner notification", category: "notification", mutate: func(form *hubspot.FormDefinition) { form.Configuration.NotifyContactOwner = true }},
		{name: "lifecycle automation", category: "lifecycle", mutate: func(form *hubspot.FormDefinition) {
			form.Configuration.LifecycleStages = []hubspot.FormLifecycleStage{{ObjectTypeID: "0-1", Value: "lead"}}
		}},
		{name: "contact creation", category: "contact creation", mutate: func(form *hubspot.FormDefinition) { form.Configuration.CreateNewContactForNewEmail = true }},
		{name: "post submit redirect", category: "post-submit", mutate: func(form *hubspot.FormDefinition) { form.Configuration.PostSubmitAction.Type = "redirect_url" }},
		{name: "raw HTML", category: "rendering", mutate: func(form *hubspot.FormDefinition) { form.DisplayOptions.RenderRawHTML = true }},
		{name: "theme", category: "theme", mutate: func(form *hubspot.FormDefinition) { form.DisplayOptions.Theme = "canvas" }},
		{name: "custom CSS", category: "CSS", mutate: func(form *hubspot.FormDefinition) { form.DisplayOptions.CSSClass = "unsafe" }},
		{name: "not editable", category: "capability", mutate: func(form *hubspot.FormDefinition) { form.Configuration.Editable = false }},
		{name: "not cloneable", category: "capability", mutate: func(form *hubspot.FormDefinition) { form.Configuration.Cloneable = false }},
		{name: "not archivable", category: "capability", mutate: func(form *hubspot.FormDefinition) { form.Configuration.Archivable = false }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			form := supportedFormDefinitionForTest(t)
			test.mutate(&form)
			_, diagnostics := formModelFromDefinition(form)
			if !diagnostics.HasError() {
				t.Fatal("unsupported remote shape was accepted")
			}
			text := diagnostics[0].Summary() + " " + diagnostics[0].Detail()
			if !strings.Contains(strings.ToLower(text), strings.ToLower(test.category)) {
				t.Fatalf("diagnostic %q omitted category %q", text, test.category)
			}
			if strings.Contains(text, form.ID) || strings.Contains(text, form.Name) {
				t.Fatalf("diagnostic exposed form identity or content: %q", text)
			}
		})
	}
}

func TestFormDefinitionPatchSelectsOnlyChangedTopLevelSubtrees(t *testing.T) {
	state := supportedFormModelForTest()
	noOp, changed, diagnostics := formPatchFromModels(context.Background(), state, state)
	if diagnostics.HasError() || changed || !reflect.DeepEqual(noOp, hubspot.FormDefinitionPatch{}) {
		t.Fatalf("semantic no-op patch = %#v, changed=%v, diagnostics=%#v", noOp, changed, diagnostics)
	}

	plan := supportedFormModelForTest()
	plan.Name = types.StringValue("Updated form")
	display := formDisplayOptionsModel{
		SubmitButtonText: types.StringValue("Send"),
		Style:            formStyleValue(supportedFormStyleModelForTest()),
	}
	plan.DisplayOptions = formDisplayOptionsValue(display)
	patch, changed, diagnostics := formPatchFromModels(context.Background(), state, plan)
	if diagnostics.HasError() || !changed {
		t.Fatalf("changed patch diagnostics=%#v changed=%v", diagnostics, changed)
	}
	if patch.Name == nil || *patch.Name != "Updated form" || patch.DisplayOptions == nil {
		t.Fatalf("patch omitted changed subtrees: %#v", patch)
	}
	if patch.FieldGroups != nil || patch.Configuration != nil {
		t.Fatalf("patch included unchanged subtrees: %#v", patch)
	}
}

func TestFormDefinitionImportIDRequiresCanonicalGeneratedUUID(t *testing.T) {
	valid := []string{
		"01234567-89ab-cdef-0123-456789abcdef",
		"ffffffff-ffff-ffff-ffff-ffffffffffff",
	}
	invalid := []string{
		"", "managed-form", "form-1", "hubspot/01234567-89ab-cdef-0123-456789abcdef",
		"https://api.hubapi.com/marketing/v3/forms/01234567-89ab-cdef-0123-456789abcdef",
		" 01234567-89ab-cdef-0123-456789abcdef", "01234567-89AB-CDEF-0123-456789ABCDEF",
		"0123456789abcdef0123456789abcdef", "01234567-89ab-cdef-0123-456789abcdeg",
	}
	for _, id := range valid {
		if !validFormImportID(id) {
			t.Errorf("valid generated ID %q rejected", id)
		}
	}
	for _, id := range invalid {
		if validFormImportID(id) {
			t.Errorf("invalid generated ID %q accepted", id)
		}
	}
}

func supportedFormDefinitionForTest(t *testing.T) hubspot.FormDefinition {
	t.Helper()
	write, diagnostics := formWriteFromModel(context.Background(), supportedFormModelForTest())
	if diagnostics.HasError() {
		t.Fatalf("build supported form: %#v", diagnostics)
	}
	return hubspot.FormDefinition{ID: "generated-sensitive-id", FormDefinitionWrite: write}
}

func supportedFormModelForTest() formDefinitionResourceModel {
	return formDefinitionResourceModel{
		Name: types.StringValue("Managed form"),
		FieldGroups: formFieldGroupsValue([]formFieldGroupModel{{Fields: formFieldsValue([]formFieldModel{{
			Label: types.StringValue("Email address"), Description: types.StringValue("Contact email"), Placeholder: types.StringValue("name@example.com"),
			Required: types.BoolValue(true), BlockedEmailDomains: types.ListValueMust(types.StringType, nil), UseDefaultBlockList: types.BoolValue(true),
		}})}}),
		Configuration: formConfigurationValue(formConfigurationModel{
			Language: types.StringValue("en"), AllowLinkToResetKnownValues: types.BoolValue(false), PrePopulateKnownValues: types.BoolValue(false),
			RecaptchaEnabled: types.BoolValue(true), ThankYouText: types.StringValue("Thank you"),
		}),
		DisplayOptions: formDisplayOptionsValue(formDisplayOptionsModel{
			SubmitButtonText: types.StringValue("Submit"), Style: formStyleValue(supportedFormStyleModelForTest()),
		}),
	}
}

func supportedFormStyleModelForTest() formStyleModel {
	return formStyleModel{
		LabelTextSize: types.StringValue("13px"), LabelTextColor: types.StringValue("#33475b"),
		LegalConsentTextSize: types.StringValue("12px"), LegalConsentTextColor: types.StringValue("#33475b"),
		HelpTextSize: types.StringValue("11px"), HelpTextColor: types.StringValue("#516f90"),
		FontFamily: types.StringValue("Arial, sans-serif"), BackgroundWidth: types.StringValue("100%"),
		SubmitFontColor: types.StringValue("#ffffff"), SubmitAlignment: types.StringValue("left"),
		SubmitSize: types.StringValue("12px 24px"), SubmitColor: types.StringValue("#ff7a59"),
	}
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
