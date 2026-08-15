// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

//go:build acceptance

package acceptance

import (
	"net/http"
	"sort"
	"strconv"

	"github.com/jackemcpherson/terraform-provider-hubspot/internal/hubspot"
)

type fakeCRMUserProfile struct {
	profile      hubspot.CRMUserProfile
	patchCount   int
	patchHistory [][]string
}

func (f *FakeHubSpot) handleCRMUserProfiles(response http.ResponseWriter, request *http.Request, rest []string) {
	switch len(rest) {
	case 0:
		f.handleCRMUserProfileCollection(response, request)
	case 1:
		f.handleCRMUserProfileItem(response, request, rest[0])
	default:
		writeFakeError(response, http.StatusNotFound, "OBJECT_NOT_FOUND", "", "No CRM user profile route matched this request.")
	}
}

func (f *FakeHubSpot) handleCRMUserProfileCollection(response http.ResponseWriter, request *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if request.Method != http.MethodGet {
		writeFakeError(response, http.StatusMethodNotAllowed, "VALIDATION_ERROR", "", "Unsupported method.")
		return
	}
	f.crmProfileListReads++
	ids := make([]string, 0, len(f.crmUserProfiles))
	for id := range f.crmUserProfiles {
		if f.crmProfileReadiness[id] > 0 {
			f.crmProfileReadiness[id]--
			continue
		}
		ids = append(ids, id)
	}
	sort.Strings(ids)
	results := make([]map[string]any, 0, len(ids))
	for _, id := range ids {
		results = append(results, fakeCRMUserProfileDocument(f.crmUserProfiles[id].profile))
	}
	writeFakeJSON(response, http.StatusOK, map[string]any{"results": results})
}

func (f *FakeHubSpot) handleCRMUserProfileItem(response http.ResponseWriter, request *http.Request, id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	profile := f.crmUserProfiles[id]
	if profile == nil {
		writeFakeError(response, http.StatusNotFound, "OBJECT_NOT_FOUND", "", "No CRM user profile matched this identity.")
		return
	}
	switch request.Method {
	case http.MethodGet:
		writeFakeJSON(response, http.StatusOK, fakeCRMUserProfileDocument(profile.profile))
	case http.MethodPatch:
		if f.rejectNextCRMPatch {
			f.rejectNextCRMPatch = false
			writeFakeError(response, http.StatusBadRequest, "VALIDATION_ERROR", "", "The CRM user profile patch was rejected.")
			return
		}
		var body struct {
			Properties map[string]string `json:"properties"`
		}
		if !decodeFakeBody(response, request, &body) {
			return
		}
		if len(body.Properties) == 0 {
			writeFakeError(response, http.StatusBadRequest, "VALIDATION_ERROR", "", "A CRM user profile patch requires one property.")
			return
		}
		updated := profile.profile
		for name, value := range body.Properties {
			switch name {
			case "hs_job_title":
				updated.JobTitle = value
			case "hs_availability_status":
				if value != "available" && value != "away" {
					writeFakeError(response, http.StatusBadRequest, "VALIDATION_ERROR", "", "Unsupported availability status.")
					return
				}
				updated.AvailabilityStatus = value
			case "hs_standard_time_zone":
				if value == "" {
					writeFakeError(response, http.StatusBadRequest, "VALIDATION_ERROR", "", "A timezone is required.")
					return
				}
				updated.TimeZone = value
			case "hs_working_hours":
				if profile.profile.TimeZone == "" {
					writeFakeError(response, http.StatusBadRequest, "VALIDATION_ERROR", "", "Set timezone before working hours.")
					return
				}
				hours, err := hubspot.ParseCRMUserWorkingHours(value)
				if err != nil {
					writeFakeError(response, http.StatusBadRequest, "VALIDATION_ERROR", "", "Working hours were invalid.")
					return
				}
				updated.WorkingHours = hours
			default:
				writeFakeError(response, http.StatusBadRequest, "VALIDATION_ERROR", "", "The CRM user property is not writable.")
				return
			}
		}
		propertyNames := make([]string, 0, len(body.Properties))
		for name := range body.Properties {
			propertyNames = append(propertyNames, name)
		}
		sort.Strings(propertyNames)
		profile.profile = updated
		profile.patchCount++
		profile.patchHistory = append(profile.patchHistory, propertyNames)
		if f.malformedNextCRMPatchSuccess {
			f.malformedNextCRMPatchSuccess = false
			writeFakeJSON(response, http.StatusOK, map[string]any{
				"id":         profile.profile.ID,
				"properties": map[string]string{"hs_working_hours": "future-format"},
			})
			return
		}
		writeFakeJSON(response, http.StatusOK, fakeCRMUserProfileDocument(profile.profile))
	default:
		writeFakeError(response, http.StatusMethodNotAllowed, "VALIDATION_ERROR", "", "Unsupported method.")
	}
}

func fakeCRMUserProfileDocument(profile hubspot.CRMUserProfile) map[string]any {
	hours, err := hubspot.SerializeCRMUserWorkingHours(profile.WorkingHours)
	if err != nil {
		panic(err)
	}
	return map[string]any{
		"id": profile.ID,
		"properties": map[string]string{
			"hs_internal_user_id":    profile.SettingsID,
			"hs_job_title":           profile.JobTitle,
			"hs_availability_status": profile.AvailabilityStatus,
			"hs_standard_time_zone":  profile.TimeZone,
			"hs_working_hours":       hours,
		},
		"futureField": map[string]any{"ignored": true},
	}
}

// SeedCRMUserProfile creates one activated Settings membership and its
// distinct account-scoped CRM projection for hermetic profile tests.
func (f *FakeHubSpot) SeedCRMUserProfile(settingsID string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.seedAccountMembership(settingsID)
	return f.seedCRMUserProfile(settingsID)
}

func (f *FakeHubSpot) seedAccountMembership(settingsID string) {
	email := "seed-" + settingsID + "@example.invalid"
	f.accountMemberships[settingsID] = &fakeAccountMembership{
		membership: hubspot.AccountMembership{ID: settingsID, Email: email, RoleIDs: []string{}, SecondaryTeamIDs: []string{}},
		active:     true, activated: true,
	}
	f.accountMembershipIDsByEmail[email] = settingsID
}

func (f *FakeHubSpot) seedCRMUserProfile(settingsID string) string {
	f.nextCRMUserProfileID++
	id := strconv.Itoa(30_000 + f.nextCRMUserProfileID)
	f.crmUserProfiles[id] = &fakeCRMUserProfile{profile: hubspot.CRMUserProfile{ID: id, SettingsID: settingsID, WorkingHours: []hubspot.CRMUserWorkingHours{}}}
	return id
}

func (f *FakeHubSpot) SeedAccountMembership(settingsID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.seedAccountMembership(settingsID)
}

func (f *FakeHubSpot) DuplicateCRMUserProfile(settingsID string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.seedCRMUserProfile(settingsID)
}

func (f *FakeHubSpot) DelayCRMUserProfileMaterialization(id string, listReads int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.crmProfileReadiness[id] = listReads
}

func (f *FakeHubSpot) CRMUserProfileListReads() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.crmProfileListReads
}

func (f *FakeHubSpot) RejectNextCRMUserProfilePatch() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rejectNextCRMPatch = true
}

func (f *FakeHubSpot) MalformNextCRMUserProfilePatchSuccess() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.malformedNextCRMPatchSuccess = true
}

func (f *FakeHubSpot) CRMUserProfilePatchCount(id string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	if profile := f.crmUserProfiles[id]; profile != nil {
		return profile.patchCount
	}
	return 0
}

func (f *FakeHubSpot) CRMUserProfilePatchHistory(id string) [][]string {
	f.mu.Lock()
	defer f.mu.Unlock()
	profile := f.crmUserProfiles[id]
	if profile == nil {
		return nil
	}
	history := make([][]string, len(profile.patchHistory))
	for index := range profile.patchHistory {
		history[index] = append([]string(nil), profile.patchHistory[index]...)
	}
	return history
}

func (f *FakeHubSpot) DriftCRMUserProfile(id, jobTitle, availability string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	profile := f.crmUserProfiles[id]
	if profile == nil {
		return false
	}
	profile.profile.JobTitle = jobTitle
	profile.profile.AvailabilityStatus = availability
	return true
}

func (f *FakeHubSpot) CRMUserProfileSnapshot(id string) (hubspot.CRMUserProfile, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	profile := f.crmUserProfiles[id]
	if profile == nil {
		return hubspot.CRMUserProfile{}, false
	}
	return profile.profile, true
}
