// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

type crmUserAvailabilityValidator struct{}

func (crmUserAvailabilityValidator) Description(context.Context) string {
	return "availability_status must be available or away"
}

func (v crmUserAvailabilityValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v crmUserAvailabilityValidator) ValidateString(_ context.Context, request validator.StringRequest, response *validator.StringResponse) {
	if request.ConfigValue.IsNull() || request.ConfigValue.IsUnknown() {
		return
	}
	if value := request.ConfigValue.ValueString(); value != "available" && value != "away" {
		response.Diagnostics.AddAttributeError(request.Path, "Invalid CRM user availability status", v.Description(context.Background())+".")
	}
}

type crmUserTimeZoneValidator struct{}

func (crmUserTimeZoneValidator) Description(context.Context) string {
	return "time_zone must be a nonblank identifier without surrounding whitespace"
}

func (v crmUserTimeZoneValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v crmUserTimeZoneValidator) ValidateString(_ context.Context, request validator.StringRequest, response *validator.StringResponse) {
	if request.ConfigValue.IsNull() || request.ConfigValue.IsUnknown() {
		return
	}
	value := request.ConfigValue.ValueString()
	if strings.TrimSpace(value) == "" || strings.TrimSpace(value) != value {
		response.Diagnostics.AddAttributeError(request.Path, "Invalid CRM user time zone", v.Description(context.Background())+".")
	}
}

type crmUserWorkingDaysValidator struct{}

func (crmUserWorkingDaysValidator) Description(context.Context) string {
	return "days must be one documented CRM user working-hours day value"
}

func (v crmUserWorkingDaysValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v crmUserWorkingDaysValidator) ValidateString(_ context.Context, request validator.StringRequest, response *validator.StringResponse) {
	if request.ConfigValue.IsNull() || request.ConfigValue.IsUnknown() {
		return
	}
	if _, ok := crmUserExpandedDays[request.ConfigValue.ValueString()]; !ok {
		response.Diagnostics.AddAttributeError(request.Path, "Invalid CRM user working-hours days", v.Description(context.Background())+".")
	}
}

type crmUserMinuteValidator struct{}

func (crmUserMinuteValidator) Description(context.Context) string {
	return "working-hours minute values must be from 0 through 1440"
}

func (v crmUserMinuteValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v crmUserMinuteValidator) ValidateInt64(_ context.Context, request validator.Int64Request, response *validator.Int64Response) {
	if request.ConfigValue.IsNull() || request.ConfigValue.IsUnknown() {
		return
	}
	if value := request.ConfigValue.ValueInt64(); value < 0 || value > 1440 {
		response.Diagnostics.AddAttributeError(request.Path, "Invalid CRM user working-hours minute", v.Description(context.Background())+".")
	}
}

var crmUserExpandedDays = map[string][]string{
	"MONDAY":           {"MONDAY"},
	"TUESDAY":          {"TUESDAY"},
	"WEDNESDAY":        {"WEDNESDAY"},
	"THURSDAY":         {"THURSDAY"},
	"FRIDAY":           {"FRIDAY"},
	"SATURDAY":         {"SATURDAY"},
	"SUNDAY":           {"SUNDAY"},
	"MONDAY_TO_FRIDAY": {"MONDAY", "TUESDAY", "WEDNESDAY", "THURSDAY", "FRIDAY"},
	"SATURDAY_SUNDAY":  {"SATURDAY", "SUNDAY"},
	"EVERY_DAY":        {"MONDAY", "TUESDAY", "WEDNESDAY", "THURSDAY", "FRIDAY", "SATURDAY", "SUNDAY"},
}

func validateCRMUserProfileModel(ctx context.Context, model crmUserProfileResourceModel) diag.Diagnostics {
	diagnostics := diag.Diagnostics{}
	managedString := func(value interface {
		IsNull() bool
		IsUnknown() bool
	}) bool {
		return !value.IsNull() && !value.IsUnknown()
	}
	workingHoursManaged := !model.WorkingHours.IsNull() && !model.WorkingHours.IsUnknown()
	hasUnknownManagedCandidate := model.JobTitle.IsUnknown() || model.AvailabilityStatus.IsUnknown() ||
		model.TimeZone.IsUnknown() || model.WorkingHours.IsUnknown()
	if !managedString(model.JobTitle) && !managedString(model.AvailabilityStatus) &&
		!managedString(model.TimeZone) && !workingHoursManaged && !hasUnknownManagedCandidate {
		diagnostics.AddAttributeError(path.Root("job_title"), "CRM user profile manages no properties", "Configure at least one of job_title, availability_status, time_zone, or working_hours. Null properties are unmanaged.")
		return diagnostics
	}
	if !workingHoursManaged {
		return diagnostics
	}
	if model.TimeZone.IsNull() {
		diagnostics.AddAttributeError(path.Root("working_hours"), "CRM user working hours require a time zone", "Configure time_zone before managing working_hours.")
		return diagnostics
	}
	if model.TimeZone.IsUnknown() {
		return diagnostics
	}

	var hours []crmUserWorkingHoursModel
	diagnostics.Append(model.WorkingHours.ElementsAs(ctx, &hours, true)...)
	if diagnostics.HasError() {
		return diagnostics
	}
	type interval struct{ start, end int64 }
	byDay := make(map[string][]interval)
	for _, item := range hours {
		if item.Days.IsNull() || item.Days.IsUnknown() || item.StartMinute.IsNull() || item.StartMinute.IsUnknown() || item.EndMinute.IsNull() || item.EndMinute.IsUnknown() {
			continue
		}
		start, end := item.StartMinute.ValueInt64(), item.EndMinute.ValueInt64()
		if end <= start {
			diagnostics.AddAttributeError(path.Root("working_hours"), "Invalid CRM user working-hours ordering", "Each end_minute must be later than its start_minute.")
			return diagnostics
		}
		for _, day := range crmUserExpandedDays[item.Days.ValueString()] {
			byDay[day] = append(byDay[day], interval{start: start, end: end})
		}
	}
	for _, intervals := range byDay {
		sort.Slice(intervals, func(i, j int) bool {
			if intervals[i].start != intervals[j].start {
				return intervals[i].start < intervals[j].start
			}
			return intervals[i].end < intervals[j].end
		})
		for index := 1; index < len(intervals); index++ {
			if intervals[index].start < intervals[index-1].end {
				diagnostics.AddAttributeError(path.Root("working_hours"), "Overlapping CRM user working hours", "Working-hours intervals must not overlap after grouped day values are expanded.")
				return diagnostics
			}
		}
	}
	return diagnostics
}
