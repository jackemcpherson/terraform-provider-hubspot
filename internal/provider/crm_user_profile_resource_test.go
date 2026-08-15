// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/jackemcpherson/terraform-provider-hubspot/internal/hubspot"
)

func TestCRMUserProfileSchemaMatchesFrozenContract(t *testing.T) {
	var response resource.SchemaResponse
	NewCRMUserProfileResource().Schema(context.Background(), resource.SchemaRequest{}, &response)
	if response.Diagnostics.HasError() {
		t.Fatalf("schema diagnostics = %v", response.Diagnostics)
	}
	attributes := response.Schema.Attributes
	if response.Schema.Version != 0 || len(attributes) != 6 {
		t.Fatalf("CRM user profile schema version/shape = %d/%d", response.Schema.Version, len(attributes))
	}
	if !attributes["id"].IsComputed() || !attributes["account_membership_id"].IsRequired() {
		t.Fatal("CRM and Settings identity boundary is incorrect")
	}
	for _, name := range []string{"job_title", "availability_status", "time_zone", "working_hours"} {
		if !attributes[name].IsOptional() || attributes[name].IsComputed() {
			t.Fatalf("%s must be optional-only so null remains unmanaged", name)
		}
	}
	hours, ok := attributes["working_hours"].(schema.SetNestedAttribute)
	if !ok {
		t.Fatalf("working_hours type = %T, want schema.SetNestedAttribute", attributes["working_hours"])
	}
	if len(hours.NestedObject.Attributes) != 3 {
		t.Fatalf("working_hours nested shape = %d", len(hours.NestedObject.Attributes))
	}
	for _, name := range []string{"days", "start_minute", "end_minute"} {
		if !hours.NestedObject.Attributes[name].IsRequired() {
			t.Fatalf("working_hours.%s must be required", name)
		}
	}
}

func TestCRMUserProfileValidatesManagedPropertiesAndWorkingHours(t *testing.T) {
	validHours := crmUserWorkingHoursSet(t, []crmUserWorkingHoursModel{
		{Days: types.StringValue("MONDAY_TO_FRIDAY"), StartMinute: types.Int64Value(540), EndMinute: types.Int64Value(720)},
		{Days: types.StringValue("MONDAY_TO_FRIDAY"), StartMinute: types.Int64Value(720), EndMinute: types.Int64Value(1020)},
	})
	overlappedHours := crmUserWorkingHoursSet(t, []crmUserWorkingHoursModel{
		{Days: types.StringValue("EVERY_DAY"), StartMinute: types.Int64Value(540), EndMinute: types.Int64Value(1020)},
		{Days: types.StringValue("MONDAY"), StartMinute: types.Int64Value(600), EndMinute: types.Int64Value(900)},
	})
	backwardsHours := crmUserWorkingHoursSet(t, []crmUserWorkingHoursModel{
		{Days: types.StringValue("SUNDAY"), StartMinute: types.Int64Value(900), EndMinute: types.Int64Value(600)},
	})

	tests := map[string]struct {
		model crmUserProfileResourceModel
		valid bool
	}{
		"requires one managed property": {model: crmUserProfileResourceModel{}, valid: false},
		"job title alone is managed": {
			model: crmUserProfileResourceModel{JobTitle: types.StringValue("")}, valid: true,
		},
		"working hours require timezone": {
			model: crmUserProfileResourceModel{WorkingHours: validHours}, valid: false,
		},
		"adjacent ranges are valid": {
			model: crmUserProfileResourceModel{TimeZone: types.StringValue("Australia/Melbourne"), WorkingHours: validHours}, valid: true,
		},
		"expanded day ranges cannot overlap": {
			model: crmUserProfileResourceModel{TimeZone: types.StringValue("Australia/Melbourne"), WorkingHours: overlappedHours}, valid: false,
		},
		"end must follow start": {
			model: crmUserProfileResourceModel{TimeZone: types.StringValue("Australia/Melbourne"), WorkingHours: backwardsHours}, valid: false,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			diagnostics := validateCRMUserProfileModel(context.Background(), test.model)
			if got := !diagnostics.HasError(); got != test.valid {
				t.Fatalf("valid = %v, want %v; diagnostics = %v", got, test.valid, diagnostics)
			}
		})
	}
}

func TestCRMUserProfileScalarValidators(t *testing.T) {
	for _, test := range []struct {
		value string
		valid bool
	}{{"101", true}, {"0", false}, {"01", false}, {"member", false}} {
		response := validator.StringResponse{}
		crmUserSettingsIDValidator{}.ValidateString(context.Background(), validator.StringRequest{
			Path: path.Root("account_membership_id"), ConfigValue: types.StringValue(test.value),
		}, &response)
		if got := !response.Diagnostics.HasError(); got != test.valid {
			t.Errorf("Settings ID %q valid = %v, want %v", test.value, got, test.valid)
		}
	}
	for _, test := range []struct {
		value string
		valid bool
	}{{"available", true}, {"away", true}, {"offline", false}, {"", false}} {
		response := validator.StringResponse{}
		crmUserAvailabilityValidator{}.ValidateString(context.Background(), validator.StringRequest{
			Path: path.Root("availability_status"), ConfigValue: types.StringValue(test.value),
		}, &response)
		if got := !response.Diagnostics.HasError(); got != test.valid {
			t.Errorf("availability %q valid = %v, want %v", test.value, got, test.valid)
		}
	}
	for _, test := range []struct {
		value int64
		valid bool
	}{{0, true}, {1440, true}, {-1, false}, {1441, false}} {
		response := validator.Int64Response{}
		crmUserMinuteValidator{}.ValidateInt64(context.Background(), validator.Int64Request{
			Path: path.Root("minute"), ConfigValue: types.Int64Value(test.value),
		}, &response)
		if got := !response.Diagnostics.HasError(); got != test.valid {
			t.Errorf("minute %d valid = %v, want %v", test.value, got, test.valid)
		}
	}
}

func TestCRMUserProfileModelLeavesNullPropertiesUnmanaged(t *testing.T) {
	remote := hubspot.CRMUserProfile{
		ID: "301", SettingsID: "101", JobTitle: "Engineer", AvailabilityStatus: "away",
		TimeZone: "Australia/Melbourne", WorkingHours: []hubspot.CRMUserWorkingHours{{Days: "MONDAY", StartMinute: 540, EndMinute: 1020}},
	}
	managed := crmUserProfileResourceModel{
		JobTitle:           types.StringValue("Engineer"),
		AvailabilityStatus: types.StringNull(),
		TimeZone:           types.StringNull(),
		WorkingHours:       types.SetNull(crmUserWorkingHoursObjectType),
	}
	model, diagnostics := crmUserProfileModelFromRemote(context.Background(), remote, managed)
	if diagnostics.HasError() {
		t.Fatalf("model conversion diagnostics = %v", diagnostics)
	}
	if model.JobTitle.ValueString() != "Engineer" || !model.AvailabilityStatus.IsNull() || !model.TimeZone.IsNull() || !model.WorkingHours.IsNull() {
		t.Fatalf("null-as-unmanaged model = %#v", model)
	}
}

func crmUserWorkingHoursSet(t *testing.T, hours []crmUserWorkingHoursModel) types.Set {
	t.Helper()
	set, diagnostics := types.SetValueFrom(context.Background(), crmUserWorkingHoursObjectType, hours)
	if diagnostics.HasError() {
		t.Fatalf("build working hours set: %v", diagnostics)
	}
	return set
}
