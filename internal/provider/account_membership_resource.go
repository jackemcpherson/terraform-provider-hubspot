// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/jackemcpherson/terraform-provider-hubspot/internal/hubspot"
)

var canonicalSettingsUserID = regexp.MustCompile(`^[1-9][0-9]*$`)

// AccountMembershipResource manages one account-specific admission of a global
// HubSpot user. CRM user profile configuration is a separate surface.
type AccountMembershipResource struct {
	client *hubspot.AccountMembershipClient
}

var _ resource.ResourceWithImportState = (*AccountMembershipResource)(nil)

type accountMembershipResourceModel struct {
	ID               types.String `tfsdk:"id"`
	Email            types.String `tfsdk:"email"`
	FirstName        types.String `tfsdk:"first_name"`
	LastName         types.String `tfsdk:"last_name"`
	SendWelcomeEmail types.Bool   `tfsdk:"send_welcome_email"`
	AllowRemoval     types.Bool   `tfsdk:"allow_removal"`
	SuperAdmin       types.Bool   `tfsdk:"super_admin"`
}

func NewAccountMembershipResource() resource.Resource { return &AccountMembershipResource{} }

func (r *AccountMembershipResource) Metadata(_ context.Context, _ resource.MetadataRequest, response *resource.MetadataResponse) {
	response.TypeName = "hubspot_account_membership"
}

func (r *AccountMembershipResource) Schema(_ context.Context, _ resource.SchemaRequest, response *resource.SchemaResponse) {
	response.Schema = schema.Schema{
		Version:             0,
		Description:         "Manages one HubSpot account membership by its canonical Settings user ID.",
		MarkdownDescription: "Manages one HubSpot account membership by its canonical Settings user `id`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true, Description: "Canonical HubSpot Settings user ID used for state and import.",
				MarkdownDescription: "Canonical HubSpot Settings user `id` used for state and import.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"email": schema.StringAttribute{
				Required: true, Description: "Global HubSpot identity email; changes replace the account membership.",
				MarkdownDescription: "Global HubSpot identity email. Changes replace the account membership.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"first_name": schema.StringAttribute{
				Optional: true, Computed: true,
				Description:         "Observed or configured global first name; configured changes require an activated user and can affect the identity across accounts.",
				MarkdownDescription: "Observed or configured global first name. Configured changes require an activated user and can affect the identity across accounts.",
				Validators:          []validator.String{accountMembershipNameValidator{}},
			},
			"last_name": schema.StringAttribute{
				Optional: true, Computed: true,
				Description:         "Observed or configured global last name; configured changes require an activated user and can affect the identity across accounts.",
				MarkdownDescription: "Observed or configured global last name. Configured changes require an activated user and can affect the identity across accounts.",
				Validators:          []validator.String{accountMembershipNameValidator{}},
			},
			"send_welcome_email": schema.BoolAttribute{
				Required: true, Description: "Creation-only choice to ask HubSpot to send its welcome email.",
				MarkdownDescription: "Creation-only choice to ask HubSpot to send its welcome email. Changes replace the membership.",
				PlanModifiers:       []planmodifier.Bool{boolRequiresReplace{}},
			},
			"allow_removal": schema.BoolAttribute{
				Optional: true, Computed: true, Default: booldefault.StaticBool(false),
				Description:         "Local opt-in guard for account membership removal; defaults to false.",
				MarkdownDescription: "Local opt-in guard for account membership removal. Defaults to `false`.",
			},
			"super_admin": schema.BoolAttribute{
				Computed: true, Description: "Whether HubSpot currently reports this member as a Super Admin.",
				MarkdownDescription: "Whether HubSpot currently reports this member as a Super Admin.",
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *AccountMembershipResource) Configure(_ context.Context, request resource.ConfigureRequest, response *resource.ConfigureResponse) {
	if request.ProviderData == nil {
		return
	}
	clients, ok := request.ProviderData.(*hubspot.ClientSet)
	if !ok || clients == nil || clients.AccountMemberships == nil {
		response.Diagnostics.AddError("Provider is not configured", "The HubSpot account membership client was not available to hubspot_account_membership.")
		return
	}
	r.client = clients.AccountMemberships
}

func (r *AccountMembershipResource) Create(ctx context.Context, request resource.CreateRequest, response *resource.CreateResponse) {
	var plan, config accountMembershipResourceModel
	response.Diagnostics.Append(request.Plan.Get(ctx, &plan)...)
	response.Diagnostics.Append(request.Config.Get(ctx, &config)...)
	if response.Diagnostics.HasError() {
		return
	}
	if nameIsConfigured(config.FirstName) || nameIsConfigured(config.LastName) {
		response.Diagnostics.AddWarning("Account membership names affect global identity", "HubSpot treats first_name and last_name as global user identity fields, so this create can affect the same user across accounts.")
	}
	created, createErr := r.client.Create(ctx, hubspot.AccountMembershipCreate{
		Email: plan.Email.ValueString(), FirstName: configuredNameValue(config.FirstName, plan.FirstName),
		LastName: configuredNameValue(config.LastName, plan.LastName), SendWelcomeEmail: plan.SendWelcomeEmail.ValueBool(),
	})
	if canonicalSettingsUserID.MatchString(created.ID) {
		recovery := plan
		recovery.ID = types.StringValue(created.ID)
		response.Diagnostics.Append(response.State.Set(ctx, &recovery)...)
		if response.Diagnostics.HasError() {
			return
		}
	}
	if created.ID == "" {
		if createErr != nil && !isAmbiguous(createErr) {
			appendHubSpotDiagnostic(&response.Diagnostics, "Account membership creation failed", createErr)
		} else {
			response.Diagnostics.AddError("Account membership creation outcome is unknown", "HubSpot did not return a canonical Settings user ID. Inspect the exact email safely and import it only with the explicit email: form; the provider did not adopt an existing membership.")
		}
		return
	}
	if !canonicalSettingsUserID.MatchString(created.ID) {
		response.Diagnostics.AddError("Account membership creation identity is invalid", "HubSpot returned a non-canonical Settings user ID. No state was written and the provider did not search by email.")
		return
	}
	verified, verifyErr := r.client.GetByID(ctx, created.ID)
	if verifyErr != nil {
		appendHubSpotDiagnostic(&response.Diagnostics, "Account membership creation verification failed", verifyErr)
		return
	}
	if !membershipMatchesCreate(verified, created.ID, plan.Email.ValueString(), config, plan) {
		response.Diagnostics.AddError("Account membership creation was not verified", "HubSpot did not return the same Settings user ID, email, and configured names. The generated ID was retained in state for exact recovery.")
		return
	}
	if createErr != nil {
		response.Diagnostics.AddWarning("Create response was ambiguous", "HubSpot returned a create error with a generated ID, but exact Settings ID read-back matched the planned membership.")
	}
	model := accountMembershipModelFromRemote(verified, plan.SendWelcomeEmail, plan.AllowRemoval)
	response.Diagnostics.Append(response.State.Set(ctx, &model)...)
}

func (r *AccountMembershipResource) Read(ctx context.Context, request resource.ReadRequest, response *resource.ReadResponse) {
	var state accountMembershipResourceModel
	response.Diagnostics.Append(request.State.Get(ctx, &state)...)
	if response.Diagnostics.HasError() {
		return
	}
	membership, err := r.client.GetByID(ctx, state.ID.ValueString())
	if err != nil {
		if isNotFound(err) {
			response.State.RemoveResource(ctx)
			return
		}
		appendHubSpotDiagnostic(&response.Diagnostics, "Account membership refresh failed", err)
		return
	}
	if membership.ID != state.ID.ValueString() {
		response.Diagnostics.AddError("Account membership refresh identity mismatch", "HubSpot returned a different Settings user ID. Prior state was retained.")
		return
	}
	model := accountMembershipModelFromRemote(membership, state.SendWelcomeEmail, state.AllowRemoval)
	response.Diagnostics.Append(response.State.Set(ctx, &model)...)
}

func (r *AccountMembershipResource) Update(ctx context.Context, request resource.UpdateRequest, response *resource.UpdateResponse) {
	var state, plan, config accountMembershipResourceModel
	response.Diagnostics.Append(request.State.Get(ctx, &state)...)
	response.Diagnostics.Append(request.Plan.Get(ctx, &plan)...)
	response.Diagnostics.Append(request.Config.Get(ctx, &config)...)
	if response.Diagnostics.HasError() {
		return
	}
	firstChanged := configuredNameChanged(config.FirstName, state.FirstName, plan.FirstName)
	lastChanged := configuredNameChanged(config.LastName, state.LastName, plan.LastName)
	if !firstChanged && !lastChanged {
		plan.ID = state.ID
		plan.SuperAdmin = state.SuperAdmin
		response.Diagnostics.Append(response.State.Set(ctx, &plan)...)
		return
	}
	current, err := r.client.GetByID(ctx, state.ID.ValueString())
	if err != nil {
		appendHubSpotDiagnostic(&response.Diagnostics, "Account membership name-update preflight failed", err)
		return
	}
	if current.ID != state.ID.ValueString() || current.Email != state.Email.ValueString() {
		response.Diagnostics.AddError("Account membership name-update identity mismatch", "The fresh Settings read did not match the exact state ID and email. No update was sent.")
		return
	}
	byEmail, err := r.client.GetByEmail(ctx, state.Email.ValueString())
	if err != nil {
		appendHubSpotDiagnostic(&response.Diagnostics, "Account membership name-update email preflight failed", err)
		return
	}
	if byEmail.ID != state.ID.ValueString() || byEmail.Email != state.Email.ValueString() {
		response.Diagnostics.AddError("Account membership name-update email identity mismatch", "The fresh Settings email lookup did not match the exact state ID and email. No update was sent.")
		return
	}
	if current.HasRoleOrTeamAssignments() || byEmail.HasRoleOrTeamAssignments() {
		response.Diagnostics.AddError("Account membership name update is unsafe", "HubSpot reports a current role or team assignment, and the Settings API does not document PUT omission semantics. No update was sent.")
		return
	}
	desiredFirst := current.FirstName
	if nameIsConfigured(config.FirstName) {
		desiredFirst = plan.FirstName.ValueString()
	}
	desiredLast := current.LastName
	if nameIsConfigured(config.LastName) {
		desiredLast = plan.LastName.ValueString()
	}
	if current.FirstName == desiredFirst && current.LastName == desiredLast {
		model := accountMembershipModelFromRemote(current, plan.SendWelcomeEmail, plan.AllowRemoval)
		response.Diagnostics.Append(response.State.Set(ctx, &model)...)
		return
	}
	response.Diagnostics.AddWarning("Account membership names affect global identity", "HubSpot treats first_name and last_name as global user identity fields, so this update can affect the same user across accounts.")
	_, updateErr := r.client.UpdateNames(ctx, state.ID.ValueString(), hubspot.AccountMembershipNameUpdate{
		FirstName: desiredFirst, LastName: desiredLast,
	})
	if updateErr != nil && !isAmbiguous(updateErr) {
		appendHubSpotDiagnostic(&response.Diagnostics, "Account membership name update failed", updateErr)
		return
	}
	verified, verifyErr := r.client.GetByID(ctx, state.ID.ValueString())
	if verifyErr != nil {
		appendHubSpotDiagnostic(&response.Diagnostics, "Account membership name-update verification failed", verifyErr)
		return
	}
	if verified.ID != state.ID.ValueString() || verified.Email != state.Email.ValueString() || verified.FirstName != desiredFirst || verified.LastName != desiredLast {
		response.Diagnostics.AddError("Account membership name update was not verified", "HubSpot did not return the exact Settings identity and desired names. Prior state was retained.")
		return
	}
	if updateErr != nil {
		response.Diagnostics.AddWarning("Update response was ambiguous", "HubSpot returned an update error, but exact Settings ID read-back matched the desired names.")
	}
	model := accountMembershipModelFromRemote(verified, plan.SendWelcomeEmail, plan.AllowRemoval)
	response.Diagnostics.Append(response.State.Set(ctx, &model)...)
}

func (r *AccountMembershipResource) Delete(ctx context.Context, request resource.DeleteRequest, response *resource.DeleteResponse) {
	var state accountMembershipResourceModel
	response.Diagnostics.Append(request.State.Get(ctx, &state)...)
	if response.Diagnostics.HasError() {
		return
	}
	if !state.AllowRemoval.ValueBool() {
		response.Diagnostics.AddError("Account membership removal is disabled", "Set allow_removal to true and apply that reviewed opt-in before destroying this membership. Removing state is the non-destructive alternative.")
		return
	}
	current, err := r.client.GetByID(ctx, state.ID.ValueString())
	if err != nil {
		if isNotFound(err) {
			return
		}
		appendHubSpotDiagnostic(&response.Diagnostics, "Account membership removal preflight failed", err)
		return
	}
	if current.ID != state.ID.ValueString() || current.Email != state.Email.ValueString() {
		response.Diagnostics.AddError("Account membership removal identity mismatch", "The fresh Settings read did not match the exact state ID and email. No deletion was sent.")
		return
	}
	byEmail, err := r.client.GetByEmail(ctx, state.Email.ValueString())
	if err != nil {
		appendHubSpotDiagnostic(&response.Diagnostics, "Account membership removal email preflight failed", err)
		return
	}
	if byEmail.ID != state.ID.ValueString() || byEmail.Email != state.Email.ValueString() {
		response.Diagnostics.AddError("Account membership removal email identity mismatch", "The fresh Settings email lookup did not match the exact state ID and email. No deletion was sent.")
		return
	}
	if current.SuperAdmin || byEmail.SuperAdmin {
		response.Diagnostics.AddError("Super Admin account membership cannot be removed", "HubSpot reports this membership as a Super Admin. Remove Super Admin access outside this resource or remove the resource from state.")
		return
	}
	deleteErr := r.client.Delete(ctx, state.ID.ValueString())
	if deleteErr != nil && !isNotFound(deleteErr) && !isAmbiguous(deleteErr) {
		appendHubSpotDiagnostic(&response.Diagnostics, "Account membership removal failed", deleteErr)
		return
	}
	if err := r.client.WaitForAbsence(ctx, state.ID.ValueString(), state.Email.ValueString()); err != nil {
		appendHubSpotDiagnostic(&response.Diagnostics, "Account membership removal was not verified", err)
		return
	}
	if deleteErr != nil {
		response.Diagnostics.AddWarning("Delete response was ambiguous", "HubSpot returned a delete error, but exact ID, email, and collection reads verified absence.")
	}
}

func (r *AccountMembershipResource) ImportState(ctx context.Context, request resource.ImportStateRequest, response *resource.ImportStateResponse) {
	var membership hubspot.AccountMembership
	var err error
	requestedEmail := ""
	if strings.HasPrefix(request.ID, "email:") {
		email := strings.TrimPrefix(request.ID, "email:")
		if email == "" || strings.TrimSpace(email) != email {
			response.Diagnostics.AddAttributeError(path.Root("email"), "Invalid account membership import email", "Use email: followed by one exact nonblank email address.")
			return
		}
		requestedEmail = email
		membership, err = r.client.GetByEmail(ctx, email)
	} else {
		if !canonicalSettingsUserID.MatchString(request.ID) {
			response.Diagnostics.AddAttributeError(path.Root("id"), "Invalid account membership import ID", "Use one canonical numeric Settings user ID or email: followed by an exact email address.")
			return
		}
		membership, err = r.client.GetByID(ctx, request.ID)
	}
	if err != nil {
		appendHubSpotDiagnostic(&response.Diagnostics, "Account membership import failed", err)
		return
	}
	if !canonicalSettingsUserID.MatchString(membership.ID) {
		response.Diagnostics.AddAttributeError(path.Root("id"), "Account membership import identity is invalid", "HubSpot did not return a canonical Settings user ID. No state was written.")
		return
	}
	if requestedEmail != "" && membership.Email != requestedEmail {
		response.Diagnostics.AddAttributeError(path.Root("email"), "Account membership import email mismatch", "HubSpot did not return the exact requested email address. No state was written.")
		return
	}
	if requestedEmail == "" && membership.ID != request.ID {
		response.Diagnostics.AddAttributeError(path.Root("id"), "Account membership import identity mismatch", "HubSpot did not return the exact requested Settings user ID. No state was written.")
		return
	}
	model := accountMembershipModelFromRemote(membership, types.BoolValue(false), types.BoolValue(false))
	response.Diagnostics.Append(response.State.Set(ctx, &model)...)
}

func accountMembershipModelFromRemote(membership hubspot.AccountMembership, welcome, allowRemoval types.Bool) accountMembershipResourceModel {
	return accountMembershipResourceModel{
		ID: types.StringValue(membership.ID), Email: types.StringValue(membership.Email),
		FirstName: types.StringValue(membership.FirstName), LastName: types.StringValue(membership.LastName),
		SendWelcomeEmail: welcome, AllowRemoval: allowRemoval, SuperAdmin: types.BoolValue(membership.SuperAdmin),
	}
}

func membershipMatchesCreate(membership hubspot.AccountMembership, id, email string, config, plan accountMembershipResourceModel) bool {
	if membership.ID != id || membership.Email != email {
		return false
	}
	if nameIsConfigured(config.FirstName) && membership.FirstName != plan.FirstName.ValueString() {
		return false
	}
	return !nameIsConfigured(config.LastName) || membership.LastName == plan.LastName.ValueString()
}

func configuredNameValue(config, plan types.String) string {
	if nameIsConfigured(config) {
		return plan.ValueString()
	}
	return ""
}

func configuredNameChanged(config, state, plan types.String) bool {
	return nameIsConfigured(config) && !plan.IsUnknown() && (state.IsNull() || state.IsUnknown() || state.ValueString() != plan.ValueString())
}

func nameIsConfigured(value types.String) bool {
	return !value.IsNull() && !value.IsUnknown()
}
