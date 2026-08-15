// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/jackemcpherson/terraform-provider-hubspot/internal/hubspot"
)

// CRMUserProfileResource manages selected properties on the CRM projection of
// an account membership. Destroy deliberately performs no remote operation.
type CRMUserProfileResource struct {
	profiles    *hubspot.CRMUserProfileClient
	memberships *hubspot.AccountMembershipClient
}

var _ resource.ResourceWithImportState = (*CRMUserProfileResource)(nil)
var _ resource.ResourceWithValidateConfig = (*CRMUserProfileResource)(nil)

var crmUserWorkingHoursObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"days":         types.StringType,
	"start_minute": types.Int64Type,
	"end_minute":   types.Int64Type,
}}

type crmUserProfileResourceModel struct {
	ID                  types.String `tfsdk:"id"`
	AccountMembershipID types.String `tfsdk:"account_membership_id"`
	JobTitle            types.String `tfsdk:"job_title"`
	AvailabilityStatus  types.String `tfsdk:"availability_status"`
	TimeZone            types.String `tfsdk:"time_zone"`
	WorkingHours        types.Set    `tfsdk:"working_hours"`
}

type crmUserWorkingHoursModel struct {
	Days        types.String `tfsdk:"days"`
	StartMinute types.Int64  `tfsdk:"start_minute"`
	EndMinute   types.Int64  `tfsdk:"end_minute"`
}

func NewCRMUserProfileResource() resource.Resource { return &CRMUserProfileResource{} }

func (r *CRMUserProfileResource) Metadata(_ context.Context, _ resource.MetadataRequest, response *resource.MetadataResponse) {
	response.TypeName = "hubspot_crm_user_profile"
}

func (r *CRMUserProfileResource) Schema(_ context.Context, _ resource.SchemaRequest, response *resource.SchemaResponse) {
	response.Schema = schema.Schema{
		Version:             0,
		Description:         "Manages selected CRM user profile properties for one HubSpot account membership.",
		MarkdownDescription: "Manages selected CRM user profile properties for one HubSpot account membership without managing that membership.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				Description:         "Canonical account-specific CRM user ID used for state and import.",
				MarkdownDescription: "Canonical account-specific CRM user `id` used for state and import.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"account_membership_id": schema.StringAttribute{
				Required:            true,
				Description:         "Settings user ID joined through hs_internal_user_id; changes replace this management relationship.",
				MarkdownDescription: "Settings user `id` joined through `hs_internal_user_id`. Changes replace this management relationship.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators:          []validator.String{crmUserSettingsIDValidator{}},
			},
			"job_title": schema.StringAttribute{
				Optional:            true,
				Description:         "Managed CRM user job title. Null leaves the remote property unmanaged; an empty string clears it.",
				MarkdownDescription: "Managed CRM user job title. `null` leaves `hs_job_title` unmanaged; an empty string clears it.",
			},
			"availability_status": schema.StringAttribute{
				Optional:            true,
				Description:         "Managed CRM availability status: available or away. Null leaves it unmanaged.",
				MarkdownDescription: "Managed CRM availability status: `available` or `away`. `null` leaves it unmanaged.",
				Validators:          []validator.String{crmUserAvailabilityValidator{}},
			},
			"time_zone": schema.StringAttribute{
				Optional:            true,
				Description:         "Managed standard timezone identifier. Null leaves it unmanaged.",
				MarkdownDescription: "Managed `hs_standard_time_zone` identifier. `null` leaves it unmanaged.",
				Validators:          []validator.String{crmUserTimeZoneValidator{}},
			},
			"working_hours": schema.SetNestedAttribute{
				Optional:            true,
				Description:         "Managed non-overlapping working-hours intervals. Null leaves the remote property unmanaged.",
				MarkdownDescription: "Managed non-overlapping working-hours intervals. `null` leaves `hs_working_hours` unmanaged.",
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"days": schema.StringAttribute{
						Required: true, Description: "Documented day or grouped-day value.",
						MarkdownDescription: "Documented day or grouped-day value.",
						Validators:          []validator.String{crmUserWorkingDaysValidator{}},
					},
					"start_minute": schema.Int64Attribute{
						Required: true, Description: "Inclusive minute offset from midnight, from 0 through 1440.",
						MarkdownDescription: "Inclusive minute offset from midnight, from `0` through `1440`.",
						Validators:          []validator.Int64{crmUserMinuteValidator{}},
					},
					"end_minute": schema.Int64Attribute{
						Required: true, Description: "Exclusive interval end minute, from 0 through 1440 and later than start_minute.",
						MarkdownDescription: "Exclusive interval end minute, from `0` through `1440` and later than `start_minute`.",
						Validators:          []validator.Int64{crmUserMinuteValidator{}},
					},
				}},
			},
		},
	}
}

func (r *CRMUserProfileResource) Configure(_ context.Context, request resource.ConfigureRequest, response *resource.ConfigureResponse) {
	if request.ProviderData == nil {
		return
	}
	clients, ok := request.ProviderData.(*hubspot.ClientSet)
	if !ok || clients == nil || clients.CRMUserProfiles == nil || clients.AccountMemberships == nil {
		response.Diagnostics.AddError("Provider is not configured", "The CRM user profile and account membership clients were not available to hubspot_crm_user_profile.")
		return
	}
	r.profiles = clients.CRMUserProfiles
	r.memberships = clients.AccountMemberships
}

func (r *CRMUserProfileResource) ValidateConfig(ctx context.Context, request resource.ValidateConfigRequest, response *resource.ValidateConfigResponse) {
	var config crmUserProfileResourceModel
	response.Diagnostics.Append(request.Config.Get(ctx, &config)...)
	if response.Diagnostics.HasError() {
		return
	}
	response.Diagnostics.Append(validateCRMUserProfileModel(ctx, config)...)
}

func (r *CRMUserProfileResource) Create(ctx context.Context, request resource.CreateRequest, response *resource.CreateResponse) {
	var plan crmUserProfileResourceModel
	response.Diagnostics.Append(request.Plan.Get(ctx, &plan)...)
	if response.Diagnostics.HasError() {
		return
	}
	profile, err := r.verifiedProfile(ctx, "", plan.AccountMembershipID.ValueString(), true, crmUserProfileFieldsFromModel(plan))
	if err != nil {
		appendHubSpotDiagnostic(&response.Diagnostics, "CRM user profile readiness failed", err)
		return
	}
	profile, ambiguous, err := r.reconcile(ctx, profile, plan)
	if err != nil {
		appendHubSpotDiagnostic(&response.Diagnostics, "CRM user profile update failed", err)
		return
	}
	if ambiguous {
		response.Diagnostics.AddWarning("CRM user profile update response was ambiguous", "Exact CRM and Settings identity read-back verified every managed property.")
	}
	model, diagnostics := crmUserProfileModelFromRemote(ctx, profile, plan)
	response.Diagnostics.Append(diagnostics...)
	if response.Diagnostics.HasError() {
		return
	}
	response.Diagnostics.Append(response.State.Set(ctx, &model)...)
}

func (r *CRMUserProfileResource) Read(ctx context.Context, request resource.ReadRequest, response *resource.ReadResponse) {
	var state crmUserProfileResourceModel
	response.Diagnostics.Append(request.State.Get(ctx, &state)...)
	if response.Diagnostics.HasError() {
		return
	}
	profile, err := r.verifiedProfile(ctx, state.ID.ValueString(), state.AccountMembershipID.ValueString(), false, crmUserProfileFieldsFromModel(state))
	if err != nil {
		if isNotFound(err) {
			response.State.RemoveResource(ctx)
			return
		}
		appendHubSpotDiagnostic(&response.Diagnostics, "CRM user profile refresh failed", err)
		return
	}
	model, diagnostics := crmUserProfileModelFromRemote(ctx, profile, state)
	response.Diagnostics.Append(diagnostics...)
	if response.Diagnostics.HasError() {
		return
	}
	response.Diagnostics.Append(response.State.Set(ctx, &model)...)
}

func (r *CRMUserProfileResource) Update(ctx context.Context, request resource.UpdateRequest, response *resource.UpdateResponse) {
	var state, plan crmUserProfileResourceModel
	response.Diagnostics.Append(request.State.Get(ctx, &state)...)
	response.Diagnostics.Append(request.Plan.Get(ctx, &plan)...)
	if response.Diagnostics.HasError() {
		return
	}
	if state.ID.IsNull() || !canonicalCRMUserID.MatchString(state.ID.ValueString()) ||
		state.AccountMembershipID.ValueString() != plan.AccountMembershipID.ValueString() {
		response.Diagnostics.AddError("CRM user profile update identity is invalid", "The prior CRM ID and planned Settings user ID did not preserve the exact management relationship. No update was sent.")
		return
	}
	profile, err := r.verifiedProfile(ctx, state.ID.ValueString(), state.AccountMembershipID.ValueString(), false, crmUserProfileFieldsFromModel(plan))
	if err != nil {
		appendHubSpotDiagnostic(&response.Diagnostics, "CRM user profile update preflight failed", err)
		return
	}
	profile, ambiguous, err := r.reconcile(ctx, profile, plan)
	if err != nil {
		appendHubSpotDiagnostic(&response.Diagnostics, "CRM user profile update failed", err)
		return
	}
	if ambiguous {
		response.Diagnostics.AddWarning("CRM user profile update response was ambiguous", "Exact CRM and Settings identity read-back verified every managed property.")
	}
	model, diagnostics := crmUserProfileModelFromRemote(ctx, profile, plan)
	response.Diagnostics.Append(diagnostics...)
	if response.Diagnostics.HasError() {
		return
	}
	response.Diagnostics.Append(response.State.Set(ctx, &model)...)
}

func (r *CRMUserProfileResource) Delete(context.Context, resource.DeleteRequest, *resource.DeleteResponse) {
	// HubSpot exposes no dedicated CRM profile delete. Destroy stops management
	// and deliberately retains every remote profile value.
}

func (r *CRMUserProfileResource) ImportState(ctx context.Context, request resource.ImportStateRequest, response *resource.ImportStateResponse) {
	var profile hubspot.CRMUserProfile
	var err error
	if strings.HasPrefix(request.ID, "membership:") {
		membershipID := strings.TrimPrefix(request.ID, "membership:")
		if !canonicalSettingsUserID.MatchString(membershipID) {
			response.Diagnostics.AddAttributeError(path.Root("account_membership_id"), "Invalid CRM user profile membership import ID", "Use membership: followed by one canonical numeric Settings user ID.")
			return
		}
		profile, err = r.verifiedProfile(ctx, "", membershipID, true, crmUserProfileAllFields())
	} else {
		if !canonicalCRMUserID.MatchString(request.ID) {
			response.Diagnostics.AddAttributeError(path.Root("id"), "Invalid CRM user profile import ID", "Use one canonical numeric CRM user ID or membership: followed by one canonical numeric Settings user ID.")
			return
		}
		profile, err = r.profiles.Get(ctx, request.ID)
		if err == nil {
			if !canonicalSettingsUserID.MatchString(profile.SettingsID) {
				err = errors.New("HubSpot CRM user profile returned a non-canonical Settings user ID")
			} else {
				profile, err = r.verifiedProfile(ctx, request.ID, profile.SettingsID, false, crmUserProfileAllFields())
			}
		}
	}
	if err != nil {
		appendHubSpotDiagnostic(&response.Diagnostics, "CRM user profile import failed", err)
		return
	}
	managed := crmUserProfileImportMask(profile)
	model, diagnostics := crmUserProfileModelFromRemote(ctx, profile, managed)
	response.Diagnostics.Append(diagnostics...)
	if response.Diagnostics.HasError() {
		return
	}
	response.Diagnostics.Append(response.State.Set(ctx, &model)...)
}

func (r *CRMUserProfileResource) verifiedProfile(ctx context.Context, crmID, membershipID string, wait bool, fields hubspot.CRMUserProfileFields) (hubspot.CRMUserProfile, error) {
	if !canonicalSettingsUserID.MatchString(membershipID) {
		return hubspot.CRMUserProfile{}, errors.New("account membership id must be one canonical numeric Settings user ID")
	}
	membership, err := r.memberships.GetByID(ctx, membershipID)
	if err != nil {
		return hubspot.CRMUserProfile{}, err
	}
	if membership.ID != membershipID {
		return hubspot.CRMUserProfile{}, errors.New("HubSpot account membership read returned a different Settings user ID")
	}
	profileID := crmID
	if wait {
		discovered, waitErr := r.profiles.WaitForSettingsID(ctx, membershipID)
		if waitErr != nil {
			return hubspot.CRMUserProfile{}, waitErr
		}
		if !canonicalCRMUserID.MatchString(discovered.ID) {
			return hubspot.CRMUserProfile{}, errors.New("HubSpot returned a non-canonical CRM user ID")
		}
		profileID = discovered.ID
	}
	profile, err := r.profiles.GetManaged(ctx, profileID, fields)
	if err != nil {
		return hubspot.CRMUserProfile{}, err
	}
	if !canonicalCRMUserID.MatchString(profile.ID) {
		return hubspot.CRMUserProfile{}, errors.New("HubSpot returned a non-canonical CRM user ID")
	}
	if crmID != "" && profile.ID != crmID {
		return hubspot.CRMUserProfile{}, errors.New("HubSpot CRM user profile read returned a different CRM user ID")
	}
	if profile.SettingsID != membershipID {
		return hubspot.CRMUserProfile{}, errors.New("HubSpot CRM user profile did not join to the exact Settings user ID")
	}
	return profile, nil
}

func (r *CRMUserProfileResource) reconcile(ctx context.Context, current hubspot.CRMUserProfile, desired crmUserProfileResourceModel) (hubspot.CRMUserProfile, bool, error) {
	properties := make(map[string]string)
	if crmUserStringManaged(desired.JobTitle) && current.JobTitle != desired.JobTitle.ValueString() {
		properties["hs_job_title"] = desired.JobTitle.ValueString()
	}
	if crmUserStringManaged(desired.AvailabilityStatus) && current.AvailabilityStatus != desired.AvailabilityStatus.ValueString() {
		properties["hs_availability_status"] = desired.AvailabilityStatus.ValueString()
	}
	if crmUserStringManaged(desired.TimeZone) && current.TimeZone != desired.TimeZone.ValueString() {
		properties["hs_standard_time_zone"] = desired.TimeZone.ValueString()
	}

	ambiguous := false
	var err error
	if len(properties) != 0 {
		current, ambiguous, err = r.patchAndVerify(ctx, current, properties)
		if err != nil {
			return hubspot.CRMUserProfile{}, ambiguous, err
		}
	}
	if desired.WorkingHours.IsNull() || desired.WorkingHours.IsUnknown() {
		return current, ambiguous, nil
	}
	hours, diagnostics := crmUserWorkingHoursFromSet(ctx, desired.WorkingHours)
	if diagnostics.HasError() {
		return hubspot.CRMUserProfile{}, ambiguous, fmt.Errorf("decode planned CRM user working hours: %s", diagnostics.Errors()[0].Detail())
	}
	desiredJSON, err := hubspot.SerializeCRMUserWorkingHours(hours)
	if err != nil {
		return hubspot.CRMUserProfile{}, ambiguous, err
	}
	currentJSON, err := hubspot.SerializeCRMUserWorkingHours(current.WorkingHours)
	if err != nil {
		return hubspot.CRMUserProfile{}, ambiguous, err
	}
	if desiredJSON == currentJSON {
		return current, ambiguous, nil
	}
	current, hoursAmbiguous, err := r.patchAndVerify(ctx, current, map[string]string{"hs_working_hours": desiredJSON})
	return current, ambiguous || hoursAmbiguous, err
}

func (r *CRMUserProfileResource) patchAndVerify(ctx context.Context, current hubspot.CRMUserProfile, properties map[string]string) (hubspot.CRMUserProfile, bool, error) {
	_, patchErr := r.profiles.PatchProperties(ctx, current.ID, properties)
	if patchErr != nil && !isAmbiguous(patchErr) {
		return hubspot.CRMUserProfile{}, false, patchErr
	}
	verified, verifyErr := r.verifiedProfile(ctx, current.ID, current.SettingsID, false, crmUserProfileFieldsFromProperties(properties))
	if verifyErr != nil {
		if patchErr != nil {
			return hubspot.CRMUserProfile{}, true, fmt.Errorf("ambiguous PATCH was not verified: %w", verifyErr)
		}
		return hubspot.CRMUserProfile{}, false, verifyErr
	}
	for name, value := range properties {
		if !crmUserProfilePropertyMatches(verified, name, value) {
			return hubspot.CRMUserProfile{}, patchErr != nil, fmt.Errorf("HubSpot CRM user profile PATCH did not apply %s", name)
		}
	}
	return mergeCRMUserProfileProperties(current, verified, properties), patchErr != nil, nil
}

func mergeCRMUserProfileProperties(current, verified hubspot.CRMUserProfile, properties map[string]string) hubspot.CRMUserProfile {
	current.ID = verified.ID
	current.SettingsID = verified.SettingsID
	if mapHasKey(properties, "hs_job_title") {
		current.JobTitle = verified.JobTitle
	}
	if mapHasKey(properties, "hs_availability_status") {
		current.AvailabilityStatus = verified.AvailabilityStatus
	}
	if mapHasKey(properties, "hs_standard_time_zone") {
		current.TimeZone = verified.TimeZone
	}
	if mapHasKey(properties, "hs_working_hours") {
		current.WorkingHours = verified.WorkingHours
	}
	return current
}

func crmUserProfilePropertyMatches(profile hubspot.CRMUserProfile, name, value string) bool {
	switch name {
	case "hs_job_title":
		return profile.JobTitle == value
	case "hs_availability_status":
		return profile.AvailabilityStatus == value
	case "hs_standard_time_zone":
		return profile.TimeZone == value
	case "hs_working_hours":
		encoded, err := hubspot.SerializeCRMUserWorkingHours(profile.WorkingHours)
		return err == nil && encoded == value
	default:
		return false
	}
}

func crmUserProfileModelFromRemote(ctx context.Context, profile hubspot.CRMUserProfile, managed crmUserProfileResourceModel) (crmUserProfileResourceModel, diag.Diagnostics) {
	model := crmUserProfileResourceModel{
		ID:                  types.StringValue(profile.ID),
		AccountMembershipID: types.StringValue(profile.SettingsID),
		JobTitle:            types.StringNull(),
		AvailabilityStatus:  types.StringNull(),
		TimeZone:            types.StringNull(),
		WorkingHours:        types.SetNull(crmUserWorkingHoursObjectType),
	}
	if crmUserStringManaged(managed.JobTitle) {
		model.JobTitle = types.StringValue(profile.JobTitle)
	}
	if crmUserStringManaged(managed.AvailabilityStatus) {
		model.AvailabilityStatus = types.StringValue(profile.AvailabilityStatus)
	}
	if crmUserStringManaged(managed.TimeZone) {
		model.TimeZone = types.StringValue(profile.TimeZone)
	}
	diagnostics := diag.Diagnostics{}
	if !managed.WorkingHours.IsNull() && !managed.WorkingHours.IsUnknown() {
		models := make([]crmUserWorkingHoursModel, 0, len(profile.WorkingHours))
		for _, item := range profile.WorkingHours {
			models = append(models, crmUserWorkingHoursModel{
				Days: types.StringValue(item.Days), StartMinute: types.Int64Value(item.StartMinute), EndMinute: types.Int64Value(item.EndMinute),
			})
		}
		var setDiagnostics diag.Diagnostics
		model.WorkingHours, setDiagnostics = types.SetValueFrom(ctx, crmUserWorkingHoursObjectType, models)
		diagnostics.Append(setDiagnostics...)
	}
	return model, diagnostics
}

func crmUserWorkingHoursFromSet(ctx context.Context, set types.Set) ([]hubspot.CRMUserWorkingHours, diag.Diagnostics) {
	var models []crmUserWorkingHoursModel
	diagnostics := set.ElementsAs(ctx, &models, false)
	if diagnostics.HasError() {
		return nil, diagnostics
	}
	hours := make([]hubspot.CRMUserWorkingHours, 0, len(models))
	for _, item := range models {
		hours = append(hours, hubspot.CRMUserWorkingHours{
			Days: item.Days.ValueString(), StartMinute: item.StartMinute.ValueInt64(), EndMinute: item.EndMinute.ValueInt64(),
		})
	}
	return hours, diagnostics
}

func crmUserStringManaged(value types.String) bool {
	return !value.IsNull() && !value.IsUnknown()
}

func crmUserProfileFieldsFromModel(model crmUserProfileResourceModel) hubspot.CRMUserProfileFields {
	return hubspot.CRMUserProfileFields{
		JobTitle:           crmUserStringManaged(model.JobTitle),
		AvailabilityStatus: crmUserStringManaged(model.AvailabilityStatus),
		TimeZone:           crmUserStringManaged(model.TimeZone),
		WorkingHours:       !model.WorkingHours.IsNull() && !model.WorkingHours.IsUnknown(),
	}
}

func crmUserProfileAllFields() hubspot.CRMUserProfileFields {
	return hubspot.CRMUserProfileFields{JobTitle: true, AvailabilityStatus: true, TimeZone: true, WorkingHours: true}
}

func crmUserProfileFieldsFromProperties(properties map[string]string) hubspot.CRMUserProfileFields {
	return hubspot.CRMUserProfileFields{
		JobTitle:           mapHasKey(properties, "hs_job_title"),
		AvailabilityStatus: mapHasKey(properties, "hs_availability_status"),
		TimeZone:           mapHasKey(properties, "hs_standard_time_zone"),
		WorkingHours:       mapHasKey(properties, "hs_working_hours"),
	}
}

func mapHasKey(values map[string]string, key string) bool {
	_, ok := values[key]
	return ok
}

func crmUserProfileImportMask(profile hubspot.CRMUserProfile) crmUserProfileResourceModel {
	mask := crmUserProfileResourceModel{
		JobTitle:           types.StringValue(profile.JobTitle),
		AvailabilityStatus: types.StringNull(),
		TimeZone:           types.StringNull(),
		WorkingHours:       types.SetNull(crmUserWorkingHoursObjectType),
	}
	if profile.AvailabilityStatus != "" {
		mask.AvailabilityStatus = types.StringValue(profile.AvailabilityStatus)
	}
	if profile.TimeZone != "" {
		mask.TimeZone = types.StringValue(profile.TimeZone)
	}
	if len(profile.WorkingHours) != 0 {
		// A known non-null placeholder marks the field managed. Conversion from
		// the remote profile replaces it before state is written.
		mask.WorkingHours = types.SetValueMust(crmUserWorkingHoursObjectType, []attr.Value{})
	}
	return mask
}
