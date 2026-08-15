// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

package hubspot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const (
	crmUserProfilePageLimit    = "100"
	crmUserProfileWaitAttempts = 20
	crmUserProfileWaitDelay    = time.Second
	crmUserProfileIdentity     = "hs_internal_user_id"
)

// CRMUserProfileClient manages the account-scoped CRM projection of one
// Settings user. It never provisions or removes account membership.
type CRMUserProfileClient struct{ transport *Transport }

// CRMUserProfile is the narrow writable CRM profile projection used by the
// provider. SettingsID is HubSpot's hs_internal_user_id join property.
type CRMUserProfile struct {
	ID                 string
	SettingsID         string
	JobTitle           string
	AvailabilityStatus string
	TimeZone           string
	WorkingHours       []CRMUserWorkingHours
}

// CRMUserProfileFields selects profile properties that the caller actively
// manages. Identity is always requested regardless of these flags.
type CRMUserProfileFields struct {
	JobTitle           bool
	AvailabilityStatus bool
	TimeZone           bool
	WorkingHours       bool
}

type crmUserProfileWire struct {
	ID         string `json:"id"`
	Properties struct {
		SettingsID         string `json:"hs_internal_user_id"`
		JobTitle           string `json:"hs_job_title"`
		AvailabilityStatus string `json:"hs_availability_status"`
		TimeZone           string `json:"hs_standard_time_zone"`
		WorkingHours       string `json:"hs_working_hours"`
	} `json:"properties"`
}

type crmUserProfilePage struct {
	Results []crmUserProfileWire `json:"results"`
	Paging  struct {
		Next struct {
			After string `json:"after"`
		} `json:"next"`
	} `json:"paging"`
}

func crmUserProfilesPath() string { return "/crm/objects/2026-03/users" }

// Get reads one exact active CRM user identity and requests every property the
// provider can manage or uses for identity verification.
func (c *CRMUserProfileClient) Get(ctx context.Context, id string) (CRMUserProfile, error) {
	return c.GetManaged(ctx, id, CRMUserProfileFields{
		JobTitle: true, AvailabilityStatus: true, TimeZone: true, WorkingHours: true,
	})
}

// GetManaged reads exact identity plus only the properties selected by the
// caller. Unmanaged fields cannot make the typed read fail.
func (c *CRMUserProfileClient) GetManaged(ctx context.Context, id string, fields CRMUserProfileFields) (CRMUserProfile, error) {
	if id == "" {
		return CRMUserProfile{}, errors.New("CRM user profile id must not be empty")
	}
	query := url.Values{"properties": []string{crmUserProfilePropertySelection(fields)}}
	var wire crmUserProfileWire
	if err := c.transport.Do(ctx, Operation{
		Name: "crm-user-profile-read", Method: http.MethodGet,
		Path: crmUserProfilesPath() + "/" + url.PathEscape(id) + "?" + query.Encode(), Replay: ReplaySafe,
	}, nil, &wire); err != nil {
		return CRMUserProfile{}, err
	}
	profile, err := crmUserProfileFromWire(wire, fields.WorkingHours)
	if err != nil {
		return CRMUserProfile{}, err
	}
	if profile.ID != id {
		return CRMUserProfile{}, errors.New("HubSpot CRM user profile response returned a different id")
	}
	return profile, nil
}

func crmUserProfilePropertySelection(fields CRMUserProfileFields) string {
	properties := make([]string, 0, 5)
	if fields.AvailabilityStatus {
		properties = append(properties, "hs_availability_status")
	}
	properties = append(properties, crmUserProfileIdentity)
	if fields.JobTitle {
		properties = append(properties, "hs_job_title")
	}
	if fields.TimeZone {
		properties = append(properties, "hs_standard_time_zone")
	}
	if fields.WorkingHours {
		properties = append(properties, "hs_working_hours")
	}
	return strings.Join(properties, ",")
}

// List returns every active CRM user profile by following HubSpot's cursor.
func (c *CRMUserProfileClient) List(ctx context.Context) ([]CRMUserProfile, error) {
	results := make([]CRMUserProfile, 0)
	after := ""
	seen := make(map[string]struct{})
	for {
		query := url.Values{
			"limit":      []string{crmUserProfilePageLimit},
			"properties": []string{crmUserProfileIdentity},
		}
		if after != "" {
			query.Set("after", after)
		}
		var page crmUserProfilePage
		if err := c.transport.Do(ctx, Operation{
			Name: "crm-user-profile-list", Method: http.MethodGet,
			Path: crmUserProfilesPath() + "?" + query.Encode(), Replay: ReplaySafe,
		}, nil, &page); err != nil {
			return nil, err
		}
		for _, wire := range page.Results {
			profile, err := crmUserProfileFromWire(wire, false)
			if err != nil {
				return nil, err
			}
			results = append(results, profile)
		}
		next := page.Paging.Next.After
		if next == "" {
			return results, nil
		}
		if _, exists := seen[next]; exists {
			return nil, errors.New("HubSpot CRM user profile list cursor repeated")
		}
		seen[next] = struct{}{}
		after = next
	}
}

// FindBySettingsID requires exactly one CRM projection for a Settings user.
func (c *CRMUserProfileClient) FindBySettingsID(ctx context.Context, settingsID string) (CRMUserProfile, error) {
	if settingsID == "" {
		return CRMUserProfile{}, errors.New("account membership id must not be empty")
	}
	profiles, err := c.List(ctx)
	if err != nil {
		return CRMUserProfile{}, err
	}
	matches := make([]CRMUserProfile, 0, 1)
	for _, profile := range profiles {
		if profile.SettingsID == settingsID {
			matches = append(matches, profile)
		}
	}
	if len(matches) == 0 {
		return CRMUserProfile{}, &CRMUserProfileNotMaterializedError{SettingsID: settingsID}
	}
	if len(matches) != 1 {
		return CRMUserProfile{}, fmt.Errorf("HubSpot returned %d CRM user profiles for one account membership", len(matches))
	}
	return matches[0], nil
}

// WaitForSettingsID bounds the asynchronous Settings-to-CRM materialization
// join. Ambiguous joins fail immediately instead of being retried.
func (c *CRMUserProfileClient) WaitForSettingsID(ctx context.Context, settingsID string) (CRMUserProfile, error) {
	for attempt := 1; attempt <= crmUserProfileWaitAttempts; attempt++ {
		profile, err := c.FindBySettingsID(ctx, settingsID)
		if err == nil {
			return profile, nil
		}
		var notMaterialized *CRMUserProfileNotMaterializedError
		if !errors.As(err, &notMaterialized) {
			return CRMUserProfile{}, err
		}
		if attempt < crmUserProfileWaitAttempts {
			if err := c.transport.sleep(ctx, crmUserProfileWaitDelay); err != nil {
				return CRMUserProfile{}, err
			}
		}
	}
	return CRMUserProfile{}, fmt.Errorf("HubSpot CRM user profile did not materialize after %d attempts; the account membership must be activated and materialized before its profile can be managed", crmUserProfileWaitAttempts)
}

// CRMUserProfileNotMaterializedError marks a currently missing Settings-to-CRM
// join so readiness polling can distinguish it from unsafe ambiguity.
type CRMUserProfileNotMaterializedError struct{ SettingsID string }

func (e *CRMUserProfileNotMaterializedError) Error() string {
	return "HubSpot returned no CRM user profile for the account membership"
}

// PatchProperties writes only caller-selected managed properties.
func (c *CRMUserProfileClient) PatchProperties(ctx context.Context, id string, properties map[string]string) (CRMUserProfile, error) {
	if id == "" {
		return CRMUserProfile{}, errors.New("CRM user profile id must not be empty")
	}
	if len(properties) == 0 {
		return CRMUserProfile{}, errors.New("CRM user profile patch requires at least one property")
	}
	body, err := json.Marshal(struct {
		Properties map[string]string `json:"properties"`
	}{Properties: properties})
	if err != nil {
		return CRMUserProfile{}, err
	}
	var wire crmUserProfileWire
	err = c.transport.Do(ctx, Operation{
		Name: "crm-user-profile-update", Method: http.MethodPatch,
		Path: crmUserProfilesPath() + "/" + url.PathEscape(id), Replay: ReplayExplicit,
	}, bytes.NewReader(body), &wire)
	if err != nil {
		var operationError *Error
		if errors.As(err, &operationError) && operationError.Status >= 200 && operationError.Status < 300 {
			operationError.Ambiguous = true
		}
		return CRMUserProfile{}, err
	}
	profile, err := crmUserProfileFromWire(wire, true)
	if err != nil {
		return CRMUserProfile{}, &Error{Operation: "crm-user-profile-update", Status: http.StatusOK, Cause: err, Ambiguous: true}
	}
	if profile.ID != id {
		return CRMUserProfile{}, &Error{Operation: "crm-user-profile-update", Status: http.StatusOK, Cause: errors.New("HubSpot CRM user profile update returned a different id"), Ambiguous: true}
	}
	return profile, nil
}

func crmUserProfileFromWire(wire crmUserProfileWire, parseWorkingHours bool) (CRMUserProfile, error) {
	if strings.TrimSpace(wire.ID) == "" {
		return CRMUserProfile{}, errors.New("HubSpot CRM user profile response omitted id")
	}
	hours := []CRMUserWorkingHours{}
	if parseWorkingHours {
		var err error
		hours, err = ParseCRMUserWorkingHours(wire.Properties.WorkingHours)
		if err != nil {
			return CRMUserProfile{}, fmt.Errorf("decode HubSpot CRM user working hours: %w", err)
		}
	}
	return CRMUserProfile{
		ID: wire.ID, SettingsID: wire.Properties.SettingsID,
		JobTitle: wire.Properties.JobTitle, AvailabilityStatus: wire.Properties.AvailabilityStatus,
		TimeZone: wire.Properties.TimeZone, WorkingHours: hours,
	}, nil
}

// CRMUserWorkingHours is one documented CRM user working-hours interval.
type CRMUserWorkingHours struct {
	Days        string `json:"days"`
	StartMinute int64  `json:"startMinute"`
	EndMinute   int64  `json:"endMinute"`
}

// SerializeCRMUserWorkingHours produces stable API property JSON from a
// Terraform set whose incoming order is intentionally insignificant.
func SerializeCRMUserWorkingHours(hours []CRMUserWorkingHours) (string, error) {
	if err := validateCRMUserWorkingHours(hours); err != nil {
		return "", err
	}
	canonical := append([]CRMUserWorkingHours(nil), hours...)
	sortCRMUserWorkingHours(canonical)
	encoded, err := json.Marshal(canonical)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

// ParseCRMUserWorkingHours decodes HubSpot's stringified JSON property and
// returns the same canonical ordering used for state and writes.
func ParseCRMUserWorkingHours(value string) ([]CRMUserWorkingHours, error) {
	if value == "" {
		return []CRMUserWorkingHours{}, nil
	}
	var hours []CRMUserWorkingHours
	if err := json.Unmarshal([]byte(value), &hours); err != nil {
		return nil, err
	}
	if hours == nil {
		hours = []CRMUserWorkingHours{}
	}
	if err := validateCRMUserWorkingHours(hours); err != nil {
		return nil, err
	}
	sortCRMUserWorkingHours(hours)
	return hours, nil
}

func validateCRMUserWorkingHours(hours []CRMUserWorkingHours) error {
	expandedDays := map[string][]string{
		"MONDAY": {"MONDAY"}, "TUESDAY": {"TUESDAY"}, "WEDNESDAY": {"WEDNESDAY"},
		"THURSDAY": {"THURSDAY"}, "FRIDAY": {"FRIDAY"}, "SATURDAY": {"SATURDAY"}, "SUNDAY": {"SUNDAY"},
		"MONDAY_TO_FRIDAY": {"MONDAY", "TUESDAY", "WEDNESDAY", "THURSDAY", "FRIDAY"},
		"SATURDAY_SUNDAY":  {"SATURDAY", "SUNDAY"},
		"EVERY_DAY":        {"MONDAY", "TUESDAY", "WEDNESDAY", "THURSDAY", "FRIDAY", "SATURDAY", "SUNDAY"},
	}
	type interval struct{ start, end int64 }
	byDay := make(map[string][]interval)
	for _, item := range hours {
		days, ok := expandedDays[item.Days]
		if !ok {
			return fmt.Errorf("unsupported CRM user working-hours days %q", item.Days)
		}
		if item.StartMinute < 0 || item.StartMinute > 1440 || item.EndMinute < 0 || item.EndMinute > 1440 {
			return errors.New("CRM user working-hours minutes must be from 0 through 1440")
		}
		if item.EndMinute <= item.StartMinute {
			return errors.New("CRM user working-hours end minute must be later than start minute")
		}
		for _, day := range days {
			byDay[day] = append(byDay[day], interval{start: item.StartMinute, end: item.EndMinute})
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
				return errors.New("CRM user working-hours intervals overlap")
			}
		}
	}
	return nil
}

func sortCRMUserWorkingHours(hours []CRMUserWorkingHours) {
	sort.Slice(hours, func(i, j int) bool {
		if hours[i].Days != hours[j].Days {
			return crmUserDayRank(hours[i].Days) < crmUserDayRank(hours[j].Days)
		}
		if hours[i].StartMinute != hours[j].StartMinute {
			return hours[i].StartMinute < hours[j].StartMinute
		}
		return hours[i].EndMinute < hours[j].EndMinute
	})
}

func crmUserDayRank(days string) int {
	for index, candidate := range []string{
		"MONDAY", "TUESDAY", "WEDNESDAY", "THURSDAY", "FRIDAY", "SATURDAY", "SUNDAY",
		"MONDAY_TO_FRIDAY", "SATURDAY_SUNDAY", "EVERY_DAY",
	} {
		if days == candidate {
			return index
		}
	}
	return 10
}
