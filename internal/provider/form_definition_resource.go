// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/jackemcpherson/terraform-provider-hubspot/internal/hubspot"
)

// FormDefinitionResource manages one deliberately narrow HubSpot form. The
// fixed structural and behavioral fields are provider-owned rather than part
// of the practitioner's configuration contract.
type FormDefinitionResource struct{ client *hubspot.FormClient }

var _ resource.ResourceWithImportState = (*FormDefinitionResource)(nil)

type formDefinitionResourceModel struct {
	ID             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	FieldGroups    types.List   `tfsdk:"field_groups"`
	Configuration  types.Object `tfsdk:"configuration"`
	DisplayOptions types.Object `tfsdk:"display_options"`
}

type formFieldGroupModel struct {
	Fields types.List `tfsdk:"fields"`
}

type formFieldModel struct {
	Label               types.String `tfsdk:"label"`
	Description         types.String `tfsdk:"description"`
	Placeholder         types.String `tfsdk:"placeholder"`
	Required            types.Bool   `tfsdk:"required"`
	BlockedEmailDomains types.List   `tfsdk:"blocked_email_domains"`
	UseDefaultBlockList types.Bool   `tfsdk:"use_default_block_list"`
}

type formConfigurationModel struct {
	Language                    types.String `tfsdk:"language"`
	AllowLinkToResetKnownValues types.Bool   `tfsdk:"allow_link_to_reset_known_values"`
	PrePopulateKnownValues      types.Bool   `tfsdk:"pre_populate_known_values"`
	RecaptchaEnabled            types.Bool   `tfsdk:"recaptcha_enabled"`
	ThankYouText                types.String `tfsdk:"thank_you_text"`
}

type formDisplayOptionsModel struct {
	SubmitButtonText types.String `tfsdk:"submit_button_text"`
	Style            types.Object `tfsdk:"style"`
}

type formStyleModel struct {
	LabelTextSize         types.String `tfsdk:"label_text_size"`
	LabelTextColor        types.String `tfsdk:"label_text_color"`
	LegalConsentTextSize  types.String `tfsdk:"legal_consent_text_size"`
	LegalConsentTextColor types.String `tfsdk:"legal_consent_text_color"`
	HelpTextSize          types.String `tfsdk:"help_text_size"`
	HelpTextColor         types.String `tfsdk:"help_text_color"`
	FontFamily            types.String `tfsdk:"font_family"`
	BackgroundWidth       types.String `tfsdk:"background_width"`
	SubmitFontColor       types.String `tfsdk:"submit_font_color"`
	SubmitAlignment       types.String `tfsdk:"submit_alignment"`
	SubmitSize            types.String `tfsdk:"submit_size"`
	SubmitColor           types.String `tfsdk:"submit_color"`
}

func NewFormDefinitionResource() resource.Resource { return &FormDefinitionResource{} }

func (r *FormDefinitionResource) Metadata(_ context.Context, _ resource.MetadataRequest, response *resource.MetadataResponse) {
	response.TypeName = "hubspot_form_definition"
}

func (r *FormDefinitionResource) Schema(_ context.Context, _ resource.SchemaRequest, response *resource.SchemaResponse) {
	response.Schema = schema.Schema{
		Version:             0,
		Description:         "Manages one narrowly typed HubSpot Form definition by its generated ID.",
		MarkdownDescription: "Manages one narrowly typed HubSpot Form definition by its generated `id`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				Description:         "HubSpot-generated form identifier used for reads and terminal archival.",
				MarkdownDescription: "HubSpot-generated form identifier used for exact reads and terminal archival.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": requiredString("Form display name; it is presentation, not identity.", formRequiredTextValidator{kind: "form name"}),
			"field_groups": schema.ListNestedAttribute{
				Required:            true,
				Description:         "Ordered form field groups; exactly one default group is supported.",
				MarkdownDescription: "Ordered form field groups. Exactly one provider-owned `default_group` is supported.",
				Validators:          []validator.List{exactlyOneListValidator{subject: "field_groups"}},
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"fields": schema.ListNestedAttribute{
						Required:            true,
						Description:         "Ordered group fields; exactly one contact email field is supported.",
						MarkdownDescription: "Ordered group fields. Exactly one provider-owned contacts `email` field is supported.",
						Validators:          []validator.List{exactlyOneListValidator{subject: "fields"}},
						NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
							"label":                  requiredString("Visible label for the email field.", formRequiredTextValidator{kind: "email label"}),
							"description":            requiredString("Visible help description for the email field.", formRequiredTextValidator{kind: "email description"}),
							"placeholder":            requiredString("Visible placeholder for the email field.", formRequiredTextValidator{kind: "email placeholder"}),
							"required":               requiredBool("Whether the email field must be completed before submission."),
							"blocked_email_domains":  requiredStringList("Explicit ordered email-domain block list; an empty list blocks no additional domains.", formBlockedEmailDomainsValidator{}),
							"use_default_block_list": requiredBool("Whether HubSpot's default email-domain block list is applied."),
						}},
					},
				}},
			},
			"configuration": schema.SingleNestedAttribute{
				Required:            true,
				Description:         "Explicit supported form behavior; unsupported automation, notifications, and contact creation remain disabled.",
				MarkdownDescription: "Explicit supported form behavior. Automation, notifications, and new-contact creation remain provider-owned and disabled.",
				Attributes: map[string]schema.Attribute{
					"language":                         requiredString("Language code used to render the form.", formLanguageValidator{}),
					"allow_link_to_reset_known_values": requiredBool("Whether visitors may reset pre-filled known values."),
					"pre_populate_known_values":        requiredBool("Whether known contact values are pre-populated."),
					"recaptcha_enabled":                requiredBool("Whether HubSpot reCAPTCHA protection is enabled."),
					"thank_you_text":                   requiredString("Text rendered after a successful submission; the action type is fixed to thank_you.", formRequiredTextValidator{kind: "thank-you text"}),
				},
			},
			"display_options": schema.SingleNestedAttribute{
				Required:            true,
				Description:         "Explicit safe-rendering presentation options; raw HTML remains disabled and the default theme is fixed.",
				MarkdownDescription: "Explicit safe-rendering presentation options. Raw HTML remains disabled and theme is fixed to `default_style`.",
				Attributes: map[string]schema.Attribute{
					"submit_button_text": requiredString("Visible submit button text.", formRequiredTextValidator{kind: "submit button text"}),
					"style": schema.SingleNestedAttribute{
						Required:            true,
						Description:         "Explicit visual style values for the safely rendered form.",
						MarkdownDescription: "Explicit visual style values for the safely rendered form.",
						Attributes: map[string]schema.Attribute{
							"label_text_size":          requiredString("CSS size for field labels.", formPixelSizeValidator{}),
							"label_text_color":         requiredString("CSS color for field labels.", formColorValidator{}),
							"legal_consent_text_size":  requiredString("CSS size for legal consent text, retained even though consent is disabled.", formPixelSizeValidator{}),
							"legal_consent_text_color": requiredString("CSS color for legal consent text, retained even though consent is disabled.", formColorValidator{}),
							"help_text_size":           requiredString("CSS size for help text.", formPixelSizeValidator{}),
							"help_text_color":          requiredString("CSS color for help text.", formColorValidator{}),
							"font_family":              requiredString("CSS font family for form text.", formFontFamilyValidator{}),
							"background_width":         requiredString("CSS width for the form background.", formPercentageSizeValidator{}),
							"submit_font_color":        requiredString("CSS color for submit button text.", formColorValidator{}),
							"submit_alignment":         requiredString("Alignment for the submit button.", formAlignmentValidator{}),
							"submit_size":              requiredString("CSS padding for the submit button.", formSubmitSizeValidator{}),
							"submit_color":             requiredString("CSS color for the submit button.", formColorValidator{}),
						},
					},
				},
			},
		},
	}
}

func requiredString(description string, validators ...validator.String) schema.StringAttribute {
	return schema.StringAttribute{Required: true, Description: description, MarkdownDescription: description, Validators: validators}
}

func requiredBool(description string) schema.BoolAttribute {
	return schema.BoolAttribute{Required: true, Description: description, MarkdownDescription: description}
}

func requiredStringList(description string, validators ...validator.List) schema.ListAttribute {
	return schema.ListAttribute{Required: true, ElementType: types.StringType, Description: description, MarkdownDescription: description, Validators: validators}
}

type exactlyOneListValidator struct{ subject string }

func (v exactlyOneListValidator) Description(context.Context) string {
	return v.subject + " must contain exactly one element"
}

func (v exactlyOneListValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v exactlyOneListValidator) ValidateList(_ context.Context, request validator.ListRequest, response *validator.ListResponse) {
	if request.ConfigValue.IsNull() || request.ConfigValue.IsUnknown() {
		return
	}
	if len(request.ConfigValue.Elements()) != 1 {
		response.Diagnostics.AddAttributeError(request.Path, "Unsupported form shape", v.subject+" must contain exactly one element.")
	}
}

func (r *FormDefinitionResource) Configure(_ context.Context, request resource.ConfigureRequest, response *resource.ConfigureResponse) {
	if request.ProviderData == nil {
		return
	}
	clients, ok := request.ProviderData.(*hubspot.ClientSet)
	if !ok || clients == nil || clients.Forms == nil {
		response.Diagnostics.AddError("Provider is not configured", "The HubSpot Forms v3 client was not available to hubspot_form_definition.")
		return
	}
	r.client = clients.Forms
}

func (r *FormDefinitionResource) Create(ctx context.Context, request resource.CreateRequest, response *resource.CreateResponse) {
	var plan formDefinitionResourceModel
	response.Diagnostics.Append(request.Plan.Get(ctx, &plan)...)
	if response.Diagnostics.HasError() {
		return
	}
	input, diagnostics := formWriteFromModel(ctx, plan)
	response.Diagnostics.Append(diagnostics...)
	if response.Diagnostics.HasError() {
		return
	}
	created, err := r.client.Create(ctx, input)
	if created.ID == "" {
		if err != nil && !isAmbiguous(err) {
			response.Diagnostics.AddError("Unable to create Form definition", err.Error())
		} else if err != nil {
			response.Diagnostics.AddError("Form definition creation outcome is unknown", "HubSpot did not return a generated form ID. Inspect active forms in HubSpot and import the exact generated ID if creation succeeded; the provider did not search by name. Original error: "+err.Error())
		} else {
			response.Diagnostics.AddError("Form definition creation outcome is unknown", "HubSpot omitted the generated form ID. Inspect active forms in HubSpot and import the exact generated ID if creation succeeded; the provider did not search by name.")
		}
		return
	}
	if !validFormImportID(created.ID) {
		response.Diagnostics.AddError("Form definition creation identity is invalid", "HubSpot returned a non-canonical generated ID. No state was written and the provider did not search by name.")
		return
	}
	recovery := plan
	recovery.ID = types.StringValue(created.ID)
	response.Diagnostics.Append(response.State.Set(ctx, &recovery)...)
	if response.Diagnostics.HasError() {
		return
	}
	verified, verifyErr := r.client.Get(ctx, created.ID)
	if verifyErr != nil {
		if err != nil {
			response.Diagnostics.AddError("Form definition creation outcome requires recovery", "HubSpot returned generated ID "+created.ID+" with an ambiguous create response, and exact-ID read-back failed. The generated ID was retained in state; retry refresh or import that exact ID. Create error: "+err.Error()+". Verification error: "+verifyErr.Error())
		} else {
			response.Diagnostics.AddError("Form definition creation outcome requires recovery", "HubSpot returned generated ID "+created.ID+", but exact-ID read-back failed. The generated ID was retained in state; retry refresh or import that exact ID. Verification error: "+verifyErr.Error())
		}
		return
	}
	if verified.ID != created.ID || verified.Archived {
		response.Diagnostics.AddError("Form definition creation was not verified", "HubSpot did not return the same active generated form ID; the generated ID was retained in state for exact recovery.")
		return
	}
	model, diagnostics := formModelFromDefinition(verified)
	response.Diagnostics.Append(diagnostics...)
	if response.Diagnostics.HasError() {
		return
	}
	if !formModelsEqualManaged(model, plan) {
		response.Diagnostics.AddError("Form definition creation was not verified", "HubSpot did not return every planned managed value; the generated ID was retained in state for exact recovery.")
		return
	}
	if err != nil {
		response.Diagnostics.AddWarning("Create response was ambiguous", "HubSpot returned a create error, but exact read-back of the generated ID matched every planned managed value.")
	}
	response.Diagnostics.Append(response.State.Set(ctx, &model)...)
}

func (r *FormDefinitionResource) Read(ctx context.Context, request resource.ReadRequest, response *resource.ReadResponse) {
	var state formDefinitionResourceModel
	response.Diagnostics.Append(request.State.Get(ctx, &state)...)
	if response.Diagnostics.HasError() {
		return
	}
	form, err := r.client.Get(ctx, state.ID.ValueString())
	if err != nil {
		if isNotFound(err) {
			archived, archivedErr := r.client.GetArchived(ctx, state.ID.ValueString())
			if archivedErr == nil {
				if archived.ID != state.ID.ValueString() || !archived.Archived {
					response.Diagnostics.AddError("Form definition absence was not verified", "The archived view returned a different or active identity; prior state was retained.")
					return
				}
				response.State.RemoveResource(ctx)
				return
			}
			if isNotFound(archivedErr) {
				response.State.RemoveResource(ctx)
				return
			}
			appendHubSpotDiagnostic(&response.Diagnostics, "Form definition archived-view refresh failed", archivedErr)
			return
		}
		appendHubSpotDiagnostic(&response.Diagnostics, "Form definition refresh failed", err)
		return
	}
	if form.ID != state.ID.ValueString() || form.Archived {
		response.Diagnostics.AddError("Form definition refresh was not verified", "HubSpot did not return the same active generated form ID; state was retained.")
		return
	}
	model, diagnostics := formModelFromDefinition(form)
	response.Diagnostics.Append(diagnostics...)
	if response.Diagnostics.HasError() {
		return
	}
	response.Diagnostics.Append(response.State.Set(ctx, &model)...)
}

func (r *FormDefinitionResource) Update(ctx context.Context, request resource.UpdateRequest, response *resource.UpdateResponse) {
	var plan, state formDefinitionResourceModel
	response.Diagnostics.Append(request.Plan.Get(ctx, &plan)...)
	response.Diagnostics.Append(request.State.Get(ctx, &state)...)
	if response.Diagnostics.HasError() {
		return
	}
	if state.ID.IsNull() || state.ID.IsUnknown() || state.ID.ValueString() == "" {
		response.Diagnostics.AddError("Form definition identity missing", "The generated form ID was absent from state; no update was sent.")
		return
	}
	patch, changed, diagnostics := formPatchFromModels(ctx, state, plan)
	response.Diagnostics.Append(diagnostics...)
	if response.Diagnostics.HasError() {
		return
	}
	if !changed {
		plan.ID = state.ID
		response.Diagnostics.Append(response.State.Set(ctx, &plan)...)
		return
	}
	_, updateErr := r.client.Update(ctx, state.ID.ValueString(), patch)
	verified, verifyErr := r.client.Get(ctx, state.ID.ValueString())
	if verifyErr != nil {
		if updateErr != nil {
			appendHubSpotDiagnostic(&response.Diagnostics, "Form definition update failed", updateErr)
		} else {
			appendHubSpotDiagnostic(&response.Diagnostics, "Form definition update verification failed", verifyErr)
		}
		return
	}
	model, diagnostics := formModelFromDefinition(verified)
	response.Diagnostics.Append(diagnostics...)
	if response.Diagnostics.HasError() {
		return
	}
	if verified.ID != state.ID.ValueString() || verified.Archived || !formModelsEqualManaged(model, plan) {
		if updateErr != nil {
			appendHubSpotDiagnostic(&response.Diagnostics, "Form definition update failed", updateErr)
		} else {
			response.Diagnostics.AddError("Form definition update was not verified", "HubSpot did not return the same active ID with every planned managed value; prior state was retained.")
		}
		return
	}
	if updateErr != nil {
		response.Diagnostics.AddWarning("Update response was ambiguous", "HubSpot returned an update error, but exact read-back matched every planned managed value.")
	}
	response.Diagnostics.Append(response.State.Set(ctx, &model)...)
}

func (r *FormDefinitionResource) Delete(ctx context.Context, request resource.DeleteRequest, response *resource.DeleteResponse) {
	var state formDefinitionResourceModel
	response.Diagnostics.Append(request.State.Get(ctx, &state)...)
	if response.Diagnostics.HasError() {
		return
	}
	id := state.ID.ValueString()
	presence, err := r.formPresence(ctx, id)
	if err != nil {
		appendHubSpotDiagnostic(&response.Diagnostics, "Form definition archival preflight failed", err)
		return
	}
	if presence == formPresenceArchived || presence == formPresenceAbsent {
		response.State.RemoveResource(ctx)
		return
	}
	archiveErr := r.client.Archive(ctx, id)
	presence, verifyErr := r.waitForFormTerminal(ctx, id)
	if verifyErr != nil {
		if archiveErr != nil {
			appendHubSpotDiagnostic(&response.Diagnostics, "Form definition archival failed", archiveErr)
		} else {
			appendHubSpotDiagnostic(&response.Diagnostics, "Form definition archival verification failed", verifyErr)
		}
		return
	}
	if presence != formPresenceArchived && presence != formPresenceAbsent {
		response.Diagnostics.AddError("Form definition archival was not verified", "HubSpot did not prove exact active absence or terminal archived state; prior state was retained.")
		return
	}
	if archiveErr != nil {
		response.Diagnostics.AddWarning("Archive response was ambiguous", "HubSpot returned an archive error, but exact active absence and the same archived form ID were verified.")
	}
	response.State.RemoveResource(ctx)
}

type formPresence int

const (
	formPresenceActive formPresence = iota
	formPresenceArchived
	formPresenceAbsent
)

func (r *FormDefinitionResource) formPresence(ctx context.Context, id string) (formPresence, error) {
	active, err := r.client.Get(ctx, id)
	if err == nil {
		if active.ID != id || active.Archived {
			return formPresenceActive, errors.New("active form identity was not exact")
		}
		return formPresenceActive, nil
	}
	if !isNotFound(err) {
		return formPresenceActive, err
	}
	archived, archivedErr := r.client.GetArchived(ctx, id)
	if archivedErr == nil {
		if archived.ID != id || !archived.Archived {
			return formPresenceArchived, errors.New("archived form identity was not exact")
		}
		return formPresenceArchived, nil
	}
	if isNotFound(archivedErr) {
		return formPresenceAbsent, nil
	}
	return formPresenceAbsent, archivedErr
}

func (r *FormDefinitionResource) waitForFormTerminal(ctx context.Context, id string) (formPresence, error) {
	const attempts = 5
	for attempt := 0; attempt < attempts; attempt++ {
		presence, err := r.formPresence(ctx, id)
		if err != nil {
			return presence, err
		}
		if presence != formPresenceActive {
			return presence, nil
		}
		if attempt+1 < attempts {
			timer := time.NewTimer(100 * time.Millisecond)
			select {
			case <-ctx.Done():
				timer.Stop()
				return formPresenceActive, ctx.Err()
			case <-timer.C:
			}
		}
	}
	return formPresenceActive, errors.New("form remained active after bounded archive verification")
}

func (r *FormDefinitionResource) ImportState(ctx context.Context, request resource.ImportStateRequest, response *resource.ImportStateResponse) {
	if !validFormImportID(request.ID) {
		response.Diagnostics.AddAttributeError(path.Root("id"), "Invalid form definition import ID", "Use one exact lowercase generated form UUID. Names, URLs, and composite identifiers are not accepted.")
		return
	}
	form, err := r.client.Get(ctx, request.ID)
	if err != nil {
		if !isNotFound(err) {
			appendHubSpotDiagnostic(&response.Diagnostics, "Form definition import failed", err)
			return
		}
		archived, archivedErr := r.client.GetArchived(ctx, request.ID)
		if archivedErr == nil {
			if archived.ID == request.ID && archived.Archived {
				response.Diagnostics.AddAttributeError(path.Root("id"), "Archived form definition cannot be imported", "Import requires the exact generated ID of an active supported Form definition.")
				return
			}
			response.Diagnostics.AddAttributeError(path.Root("id"), "Form definition import identity mismatch", "HubSpot did not return the exact archived generated ID; no state was written.")
			return
		}
		if isNotFound(archivedErr) {
			response.Diagnostics.AddAttributeError(path.Root("id"), "Form definition was not found", "Neither the active nor archived view contained the exact generated form ID; no state was written.")
			return
		}
		appendHubSpotDiagnostic(&response.Diagnostics, "Form definition archived-view import check failed", archivedErr)
		return
	}
	if form.ID != request.ID || form.Archived {
		response.Diagnostics.AddAttributeError(path.Root("id"), "Form definition import identity mismatch", "HubSpot did not return the same active generated form ID; no state was written.")
		return
	}
	model, diagnostics := formModelFromDefinition(form)
	response.Diagnostics.Append(diagnostics...)
	if response.Diagnostics.HasError() {
		return
	}
	response.Diagnostics.Append(response.State.Set(ctx, &model)...)
}

func formWriteFromModel(ctx context.Context, model formDefinitionResourceModel) (hubspot.FormDefinitionWrite, diag.Diagnostics) {
	var diagnostics diag.Diagnostics
	var groups []formFieldGroupModel
	diagnostics.Append(model.FieldGroups.ElementsAs(ctx, &groups, false)...)
	if diagnostics.HasError() || len(groups) != 1 {
		if !diagnostics.HasError() {
			diagnostics.AddError("Unsupported form shape", "field_groups must contain exactly one element.")
		}
		return hubspot.FormDefinitionWrite{}, diagnostics
	}
	var fields []formFieldModel
	diagnostics.Append(groups[0].Fields.ElementsAs(ctx, &fields, false)...)
	if diagnostics.HasError() || len(fields) != 1 {
		if !diagnostics.HasError() {
			diagnostics.AddError("Unsupported form shape", "fields must contain exactly one element.")
		}
		return hubspot.FormDefinitionWrite{}, diagnostics
	}
	var blocked []string
	diagnostics.Append(fields[0].BlockedEmailDomains.ElementsAs(ctx, &blocked, false)...)
	var configuration formConfigurationModel
	diagnostics.Append(model.Configuration.As(ctx, &configuration, basetypes.ObjectAsOptions{})...)
	var display formDisplayOptionsModel
	diagnostics.Append(model.DisplayOptions.As(ctx, &display, basetypes.ObjectAsOptions{})...)
	var style formStyleModel
	if !diagnostics.HasError() {
		diagnostics.Append(display.Style.As(ctx, &style, basetypes.ObjectAsOptions{})...)
	}
	if diagnostics.HasError() {
		return hubspot.FormDefinitionWrite{}, diagnostics
	}
	return hubspot.FormDefinitionWrite{
		FormType: "hubspot",
		Name:     model.Name.ValueString(),
		FieldGroups: []hubspot.FormFieldGroup{{
			GroupType: "default_group", RichTextType: "text",
			Fields: []hubspot.FormField{{
				ObjectTypeID: "0-1", Hidden: false, Name: "email", DependentFields: []hubspot.FormDependentField{},
				Label: fields[0].Label.ValueString(), Description: fields[0].Description.ValueString(), Placeholder: fields[0].Placeholder.ValueString(),
				FieldType: "email", Required: fields[0].Required.ValueBool(),
				Validation: hubspot.FormFieldValidation{BlockedEmailDomains: blocked, UseDefaultBlockList: fields[0].UseDefaultBlockList.ValueBool()},
			}},
		}},
		Configuration: hubspot.FormConfiguration{
			CreateNewContactForNewEmail: false, Editable: true, AllowLinkToResetKnownValues: configuration.AllowLinkToResetKnownValues.ValueBool(),
			PostSubmitAction: hubspot.FormPostSubmitAction{Type: "thank_you", Value: configuration.ThankYouText.ValueString()},
			Language:         configuration.Language.ValueString(), PrePopulateKnownValues: configuration.PrePopulateKnownValues.ValueBool(),
			Cloneable: true, NotifyContactOwner: false, RecaptchaEnabled: configuration.RecaptchaEnabled.ValueBool(), Archivable: true,
			NotifyRecipients: []string{},
		},
		DisplayOptions: hubspot.FormDisplayOptions{
			RenderRawHTML: false, Theme: "default_style", SubmitButtonText: display.SubmitButtonText.ValueString(),
			Style: hubspot.FormStyle{
				LabelTextSize: style.LabelTextSize.ValueString(), LabelTextColor: style.LabelTextColor.ValueString(),
				LegalConsentTextSize: style.LegalConsentTextSize.ValueString(), LegalConsentTextColor: style.LegalConsentTextColor.ValueString(),
				HelpTextSize: style.HelpTextSize.ValueString(), HelpTextColor: style.HelpTextColor.ValueString(),
				FontFamily: style.FontFamily.ValueString(), BackgroundWidth: style.BackgroundWidth.ValueString(),
				SubmitFontColor: style.SubmitFontColor.ValueString(), SubmitAlignment: style.SubmitAlignment.ValueString(),
				SubmitSize: style.SubmitSize.ValueString(), SubmitColor: style.SubmitColor.ValueString(),
			},
		},
		LegalConsentOptions: hubspot.FormLegalConsentOptions{Type: "none"},
	}, diagnostics
}

func formPatchFromModels(ctx context.Context, state, plan formDefinitionResourceModel) (hubspot.FormDefinitionPatch, bool, diag.Diagnostics) {
	planned, diagnostics := formWriteFromModel(ctx, plan)
	if diagnostics.HasError() {
		return hubspot.FormDefinitionPatch{}, false, diagnostics
	}
	patch := hubspot.FormDefinitionPatch{}
	if !state.Name.Equal(plan.Name) {
		patch.Name = &planned.Name
	}
	if !state.FieldGroups.Equal(plan.FieldGroups) {
		patch.FieldGroups = &planned.FieldGroups
	}
	if !state.Configuration.Equal(plan.Configuration) {
		patch.Configuration = &planned.Configuration
	}
	if !state.DisplayOptions.Equal(plan.DisplayOptions) {
		patch.DisplayOptions = &planned.DisplayOptions
	}
	changed := patch.Name != nil || patch.FieldGroups != nil || patch.Configuration != nil || patch.DisplayOptions != nil
	return patch, changed, diagnostics
}

func formModelsEqualManaged(left, right formDefinitionResourceModel) bool {
	return left.Name.Equal(right.Name) && left.FieldGroups.Equal(right.FieldGroups) &&
		left.Configuration.Equal(right.Configuration) && left.DisplayOptions.Equal(right.DisplayOptions)
}

func formModelFromDefinition(form hubspot.FormDefinition) (formDefinitionResourceModel, diag.Diagnostics) {
	diagnostics := validateSupportedFormDefinition(form)
	if diagnostics.HasError() {
		return formDefinitionResourceModel{}, diagnostics
	}
	field := form.FieldGroups[0].Fields[0]
	return formDefinitionResourceModel{
		ID:   types.StringValue(form.ID),
		Name: types.StringValue(form.Name),
		FieldGroups: formFieldGroupsValue([]formFieldGroupModel{{Fields: formFieldsValue([]formFieldModel{{
			Label: types.StringValue(field.Label), Description: types.StringValue(field.Description), Placeholder: types.StringValue(field.Placeholder),
			Required: types.BoolValue(field.Required), BlockedEmailDomains: stringListValue(field.Validation.BlockedEmailDomains),
			UseDefaultBlockList: types.BoolValue(field.Validation.UseDefaultBlockList),
		}})}}),
		Configuration: formConfigurationValue(formConfigurationModel{
			Language: types.StringValue(form.Configuration.Language), AllowLinkToResetKnownValues: types.BoolValue(form.Configuration.AllowLinkToResetKnownValues),
			PrePopulateKnownValues: types.BoolValue(form.Configuration.PrePopulateKnownValues), RecaptchaEnabled: types.BoolValue(form.Configuration.RecaptchaEnabled),
			ThankYouText: types.StringValue(form.Configuration.PostSubmitAction.Value),
		}),
		DisplayOptions: formDisplayOptionsValue(formDisplayOptionsModel{
			SubmitButtonText: types.StringValue(form.DisplayOptions.SubmitButtonText),
			Style: formStyleValue(formStyleModel{
				LabelTextSize: types.StringValue(form.DisplayOptions.Style.LabelTextSize), LabelTextColor: types.StringValue(form.DisplayOptions.Style.LabelTextColor),
				LegalConsentTextSize: types.StringValue(form.DisplayOptions.Style.LegalConsentTextSize), LegalConsentTextColor: types.StringValue(form.DisplayOptions.Style.LegalConsentTextColor),
				HelpTextSize: types.StringValue(form.DisplayOptions.Style.HelpTextSize), HelpTextColor: types.StringValue(form.DisplayOptions.Style.HelpTextColor),
				FontFamily: types.StringValue(form.DisplayOptions.Style.FontFamily), BackgroundWidth: types.StringValue(form.DisplayOptions.Style.BackgroundWidth),
				SubmitFontColor: types.StringValue(form.DisplayOptions.Style.SubmitFontColor), SubmitAlignment: types.StringValue(form.DisplayOptions.Style.SubmitAlignment),
				SubmitSize: types.StringValue(form.DisplayOptions.Style.SubmitSize), SubmitColor: types.StringValue(form.DisplayOptions.Style.SubmitColor),
			}),
		}),
	}, diagnostics
}

func validateSupportedFormDefinition(form hubspot.FormDefinition) diag.Diagnostics {
	var diagnostics diag.Diagnostics
	addUnsupported := func(category, detail string) diag.Diagnostics {
		diagnostics.AddError("Unsupported HubSpot form definition", "Unsupported "+category+": "+detail+" State was retained and no mutation was sent.")
		return diagnostics
	}
	if form.FormType != "hubspot" {
		return addUnsupported("form type", "only the live-proven hubspot form type is managed.")
	}
	if len(form.FieldGroups) != 1 {
		return addUnsupported("field group structure", "exactly one default field group is required.")
	}
	group := form.FieldGroups[0]
	if group.GroupType != "default_group" || group.RichTextType != "text" || group.RichText != "" {
		return addUnsupported("field group structure", "the default non-rich-text group shape changed.")
	}
	if len(group.Fields) != 1 {
		return addUnsupported("email field structure", "exactly one contact email field is required.")
	}
	field := group.Fields[0]
	if field.ObjectTypeID != "0-1" || field.Name != "email" || field.FieldType != "email" || field.Hidden {
		return addUnsupported("email field structure", "the contact email identity or visibility changed.")
	}
	if len(field.DependentFields) != 0 {
		return addUnsupported("dependent field structure", "dependent fields are outside the managed contract.")
	}
	if field.DefaultValue != "" {
		return addUnsupported("default value", "runtime field defaults are outside the managed contract.")
	}
	if form.LegalConsentOptions.Type != "none" {
		return addUnsupported("consent model", "only the no-consent model is supported.")
	}
	configuration := form.Configuration
	if len(configuration.NotifyRecipients) != 0 || configuration.NotifyContactOwner {
		return addUnsupported("notification configuration", "form notifications are outside the managed contract.")
	}
	if len(configuration.LifecycleStages) != 0 {
		return addUnsupported("lifecycle automation", "lifecycle-stage mutation is outside the managed contract.")
	}
	if configuration.CreateNewContactForNewEmail {
		return addUnsupported("contact creation behavior", "new-contact creation is outside the managed contract.")
	}
	if configuration.PostSubmitAction.Type != "thank_you" {
		return addUnsupported("post-submit action", "only the thank-you display action is supported.")
	}
	if !configuration.Editable || !configuration.Cloneable || !configuration.Archivable {
		return addUnsupported("capability flags", "the required editable, cloneable, or archivable capability changed.")
	}
	if form.DisplayOptions.RenderRawHTML {
		return addUnsupported("rendering mode", "raw HTML is outside the managed contract.")
	}
	if form.DisplayOptions.Theme != "default_style" {
		return addUnsupported("theme", "only default_style is supported.")
	}
	if form.DisplayOptions.CSSClass != "" {
		return addUnsupported("CSS configuration", "custom CSS classes are outside the managed contract.")
	}
	if !validRemotePresentation(form) {
		return addUnsupported("presentation values", "one or more returned managed values are outside the narrow typed contract.")
	}
	return diagnostics
}

func validRemotePresentation(form hubspot.FormDefinition) bool {
	field := form.FieldGroups[0].Fields[0]
	configuration := form.Configuration
	display := form.DisplayOptions
	style := display.Style
	for _, value := range []string{form.Name, field.Label, field.Description, field.Placeholder, configuration.PostSubmitAction.Value, display.SubmitButtonText} {
		if value == "" || value != strings.TrimSpace(value) {
			return false
		}
	}
	if configuration.Language != "en" || (style.SubmitAlignment != "left" && style.SubmitAlignment != "center") {
		return false
	}
	for _, color := range []string{style.LabelTextColor, style.LegalConsentTextColor, style.HelpTextColor, style.SubmitFontColor, style.SubmitColor} {
		if !formColorPattern.MatchString(color) {
			return false
		}
	}
	for _, size := range []string{style.LabelTextSize, style.LegalConsentTextSize, style.HelpTextSize} {
		if !formPixelSizePattern.MatchString(size) {
			return false
		}
	}
	if !formFontFamilyPattern.MatchString(style.FontFamily) || !formPercentPattern.MatchString(style.BackgroundWidth) || !formSubmitSizePattern.MatchString(style.SubmitSize) {
		return false
	}
	for _, domain := range field.Validation.BlockedEmailDomains {
		if len(domain) > 253 || !formDomainPattern.MatchString(domain) {
			return false
		}
	}
	return true
}

func formFieldGroupsValue(groups []formFieldGroupModel) types.List {
	values := make([]attr.Value, 0, len(groups))
	for _, group := range groups {
		values = append(values, types.ObjectValueMust(formFieldGroupAttrTypes(), map[string]attr.Value{"fields": group.Fields}))
	}
	return types.ListValueMust(types.ObjectType{AttrTypes: formFieldGroupAttrTypes()}, values)
}

func formFieldsValue(fields []formFieldModel) types.List {
	values := make([]attr.Value, 0, len(fields))
	for _, field := range fields {
		values = append(values, types.ObjectValueMust(formFieldAttrTypes(), map[string]attr.Value{
			"label": field.Label, "description": field.Description, "placeholder": field.Placeholder, "required": field.Required,
			"blocked_email_domains": field.BlockedEmailDomains, "use_default_block_list": field.UseDefaultBlockList,
		}))
	}
	return types.ListValueMust(types.ObjectType{AttrTypes: formFieldAttrTypes()}, values)
}

func formConfigurationValue(value formConfigurationModel) types.Object {
	return types.ObjectValueMust(formConfigurationAttrTypes(), map[string]attr.Value{
		"language": value.Language, "allow_link_to_reset_known_values": value.AllowLinkToResetKnownValues,
		"pre_populate_known_values": value.PrePopulateKnownValues, "recaptcha_enabled": value.RecaptchaEnabled, "thank_you_text": value.ThankYouText,
	})
}

func formDisplayOptionsValue(value formDisplayOptionsModel) types.Object {
	return types.ObjectValueMust(formDisplayOptionsAttrTypes(), map[string]attr.Value{"submit_button_text": value.SubmitButtonText, "style": value.Style})
}

func formStyleValue(value formStyleModel) types.Object {
	return types.ObjectValueMust(formStyleAttrTypes(), map[string]attr.Value{
		"label_text_size": value.LabelTextSize, "label_text_color": value.LabelTextColor,
		"legal_consent_text_size": value.LegalConsentTextSize, "legal_consent_text_color": value.LegalConsentTextColor,
		"help_text_size": value.HelpTextSize, "help_text_color": value.HelpTextColor, "font_family": value.FontFamily,
		"background_width": value.BackgroundWidth, "submit_font_color": value.SubmitFontColor, "submit_alignment": value.SubmitAlignment,
		"submit_size": value.SubmitSize, "submit_color": value.SubmitColor,
	})
}

func stringListValue(values []string) types.List {
	elements := make([]attr.Value, len(values))
	for index, value := range values {
		elements[index] = types.StringValue(value)
	}
	return types.ListValueMust(types.StringType, elements)
}

func formFieldGroupAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{"fields": types.ListType{ElemType: types.ObjectType{AttrTypes: formFieldAttrTypes()}}}
}

func formFieldAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"label": types.StringType, "description": types.StringType, "placeholder": types.StringType, "required": types.BoolType,
		"blocked_email_domains": types.ListType{ElemType: types.StringType}, "use_default_block_list": types.BoolType,
	}
}

func formConfigurationAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"language": types.StringType, "allow_link_to_reset_known_values": types.BoolType, "pre_populate_known_values": types.BoolType,
		"recaptcha_enabled": types.BoolType, "thank_you_text": types.StringType,
	}
}

func formDisplayOptionsAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{"submit_button_text": types.StringType, "style": types.ObjectType{AttrTypes: formStyleAttrTypes()}}
}

func formStyleAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"label_text_size": types.StringType, "label_text_color": types.StringType,
		"legal_consent_text_size": types.StringType, "legal_consent_text_color": types.StringType,
		"help_text_size": types.StringType, "help_text_color": types.StringType, "font_family": types.StringType,
		"background_width": types.StringType, "submit_font_color": types.StringType, "submit_alignment": types.StringType,
		"submit_size": types.StringType, "submit_color": types.StringType,
	}
}
