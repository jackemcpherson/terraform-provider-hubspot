// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

package hubspot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestCRMUserWorkingHoursUseCanonicalJSON(t *testing.T) {
	hours := []CRMUserWorkingHours{
		{Days: "SATURDAY_SUNDAY", StartMinute: 600, EndMinute: 900},
		{Days: "MONDAY_TO_FRIDAY", StartMinute: 540, EndMinute: 1020},
	}

	encoded, err := SerializeCRMUserWorkingHours(hours)
	if err != nil {
		t.Fatal(err)
	}
	const want = `[{"days":"MONDAY_TO_FRIDAY","startMinute":540,"endMinute":1020},{"days":"SATURDAY_SUNDAY","startMinute":600,"endMinute":900}]`
	if encoded != want {
		t.Fatalf("working hours JSON = %s, want %s", encoded, want)
	}

	decoded, err := ParseCRMUserWorkingHours(` [ { "endMinute": 900, "days": "SATURDAY_SUNDAY", "startMinute": 600 }, { "days": "MONDAY_TO_FRIDAY", "endMinute": 1020, "startMinute": 540 } ] `)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded, []CRMUserWorkingHours{
		{Days: "MONDAY_TO_FRIDAY", StartMinute: 540, EndMinute: 1020},
		{Days: "SATURDAY_SUNDAY", StartMinute: 600, EndMinute: 900},
	}) {
		t.Fatalf("parsed working hours = %#v", decoded)
	}
}

func TestCRMUserProfileManagedReadIgnoresUnmanagedWorkingHours(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if properties := request.URL.Query().Get("properties"); properties != "hs_internal_user_id,hs_job_title" {
			t.Fatalf("managed read properties = %q", properties)
		}
		writer.Header().Set("Content-Type", "application/json")
		io.WriteString(writer, `{"id":"301","properties":{"hs_internal_user_id":"101","hs_job_title":"Engineer","hs_working_hours":"future-format"}}`)
	}))
	defer server.Close()
	client := &CRMUserProfileClient{transport: newTestTransport(t, server.URL)}
	profile, err := client.GetManaged(context.Background(), "301", CRMUserProfileFields{JobTitle: true})
	if err != nil || profile.JobTitle != "Engineer" || len(profile.WorkingHours) != 0 {
		t.Fatalf("managed profile = %#v, %v", profile, err)
	}
}

func TestCRMUserProfilePatchTreatsMalformedSuccessAsAmbiguous(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		io.WriteString(writer, `{"id":"301","properties":{"hs_working_hours":"future-format"}}`)
	}))
	defer server.Close()
	client := &CRMUserProfileClient{transport: newTestTransport(t, server.URL)}
	_, err := client.PatchProperties(context.Background(), "301", map[string]string{"hs_job_title": "Engineer"})
	var operationError *Error
	if !errors.As(err, &operationError) || !operationError.Ambiguous {
		t.Fatalf("PATCH error = %v, want ambiguous success-body failure", err)
	}
}

func TestCRMUserWorkingHoursRejectInvalidAndOverlappingIntervals(t *testing.T) {
	tests := map[string][]CRMUserWorkingHours{
		"unknown days":      {{Days: "WEEKDAYS", StartMinute: 540, EndMinute: 1020}},
		"negative minute":   {{Days: "MONDAY", StartMinute: -1, EndMinute: 60}},
		"minute above 1440": {{Days: "MONDAY", StartMinute: 60, EndMinute: 1441}},
		"equal range":       {{Days: "MONDAY", StartMinute: 60, EndMinute: 60}},
		"backwards range":   {{Days: "MONDAY", StartMinute: 120, EndMinute: 60}},
		"expanded overlap": {
			{Days: "MONDAY_TO_FRIDAY", StartMinute: 540, EndMinute: 1020},
			{Days: "MONDAY", StartMinute: 600, EndMinute: 900},
		},
	}
	for name, hours := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := SerializeCRMUserWorkingHours(hours); err == nil {
				t.Fatal("invalid working hours were serialized")
			}
		})
	}
	if _, err := SerializeCRMUserWorkingHours([]CRMUserWorkingHours{
		{Days: "EVERY_DAY", StartMinute: 0, EndMinute: 720},
		{Days: "EVERY_DAY", StartMinute: 720, EndMinute: 1440},
	}); err != nil {
		t.Fatalf("adjacent inclusive-bound intervals were rejected: %v", err)
	}
}

func TestCRMUserProfileReadinessDistinguishesMissingAndAmbiguousJoins(t *testing.T) {
	t.Run("missing join times out with activation guidance", func(t *testing.T) {
		requests := 0
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			requests++
			writer.Header().Set("Content-Type", "application/json")
			io.WriteString(writer, `{"results":[]}`)
		}))
		defer server.Close()
		transport := newTestTransport(t, server.URL)
		transport.sleep = func(context.Context, time.Duration) error { return nil }
		client := &CRMUserProfileClient{transport: transport}
		_, err := client.WaitForSettingsID(context.Background(), "101")
		if err == nil || !strings.Contains(err.Error(), "20 attempts") || !strings.Contains(err.Error(), "activated and materialized") {
			t.Fatalf("readiness error = %v", err)
		}
		if requests != 20 {
			t.Fatalf("readiness requests = %d, want 20", requests)
		}
	})

	t.Run("ambiguous join fails without retry", func(t *testing.T) {
		requests := 0
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			requests++
			writer.Header().Set("Content-Type", "application/json")
			io.WriteString(writer, `{"results":[{"id":"301","properties":{"hs_internal_user_id":"101"}},{"id":"302","properties":{"hs_internal_user_id":"101"}}]}`)
		}))
		defer server.Close()
		transport := newTestTransport(t, server.URL)
		transport.sleep = func(context.Context, time.Duration) error {
			t.Fatal("ambiguous join was retried")
			return nil
		}
		client := &CRMUserProfileClient{transport: transport}
		_, err := client.WaitForSettingsID(context.Background(), "101")
		if err == nil || !strings.Contains(err.Error(), "2 CRM user profiles") {
			t.Fatalf("ambiguous join error = %v", err)
		}
		if requests != 1 {
			t.Fatalf("ambiguous join requests = %d, want 1", requests)
		}
	})
}

func TestCRMUserProfileClientRejectsMalformedResponsesAndAPIRejection(t *testing.T) {
	tests := map[string]struct {
		status int
		body   string
		match  string
	}{
		"missing id":              {status: http.StatusOK, body: `{"properties":{"hs_internal_user_id":"101"}}`, match: "omitted id"},
		"different id":            {status: http.StatusOK, body: `{"id":"999","properties":{"hs_internal_user_id":"101"}}`, match: "different id"},
		"malformed working hours": {status: http.StatusOK, body: `{"id":"301","properties":{"hs_internal_user_id":"101","hs_working_hours":"not-json"}}`, match: "decode HubSpot CRM user working hours"},
		"API rejection":           {status: http.StatusBadRequest, body: `{"status":"error","category":"VALIDATION_ERROR","message":"rejected profile value"}`, match: "rejected profile value"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writer.Header().Set("Content-Type", "application/json")
				writer.WriteHeader(test.status)
				io.WriteString(writer, test.body)
			}))
			defer server.Close()
			client := &CRMUserProfileClient{transport: newTestTransport(t, server.URL)}
			_, err := client.Get(context.Background(), "301")
			if err == nil || !strings.Contains(err.Error(), test.match) {
				t.Fatalf("error = %v, want match %q", err, test.match)
			}
		})
	}
}

func TestCRMUserProfileClientDiscoversExactIdentityAndPatchesProperties(t *testing.T) {
	requests := make([]string, 0, 5)
	firstPageReads := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests = append(requests, request.Method+" "+request.URL.RequestURI())
		writer.Header().Set("Content-Type", "application/json")
		switch request.Method + " " + request.URL.Path {
		case "GET /crm/objects/2026-03/users":
			if request.URL.Query().Get("after") == "next-page" {
				io.WriteString(writer, `{"results":[{"id":"302","properties":{"hs_internal_user_id":"202","hs_working_hours":"future-format"},"futureField":true}]}`)
				return
			}
			firstPageReads++
			if firstPageReads == 1 {
				io.WriteString(writer, `{"results":[],"paging":{"next":{"after":"next-page"}}}`)
				return
			}
			io.WriteString(writer, `{"results":[{"id":"301","properties":{"hs_internal_user_id":"101","hs_job_title":"Engineer","hs_availability_status":"available","hs_standard_time_zone":"Australia/Melbourne","hs_working_hours":"[{\"days\":\"MONDAY_TO_FRIDAY\",\"startMinute\":540,\"endMinute\":1020}]"}}]}`)
		case "GET /crm/objects/2026-03/users/301":
			io.WriteString(writer, `{"id":"301","properties":{"hs_internal_user_id":"101","hs_job_title":"Engineer","hs_availability_status":"available","hs_standard_time_zone":"Australia/Melbourne","hs_working_hours":"[]"}}`)
		case "PATCH /crm/objects/2026-03/users/301":
			var body struct {
				Properties map[string]string `json:"properties"`
			}
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(body.Properties, map[string]string{"hs_job_title": "Lead Engineer"}) {
				t.Fatalf("patch properties = %#v", body.Properties)
			}
			io.WriteString(writer, `{"id":"301","properties":{"hs_internal_user_id":"101","hs_job_title":"Lead Engineer"}}`)
		default:
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.RequestURI())
		}
	}))
	defer server.Close()

	transport := newTestTransport(t, server.URL)
	transport.sleep = func(context.Context, time.Duration) error { return nil }
	client := &CRMUserProfileClient{transport: transport}
	profile, err := client.WaitForSettingsID(context.Background(), "101")
	if err != nil {
		t.Fatal(err)
	}
	if profile.ID != "301" || profile.SettingsID != "101" {
		t.Fatalf("discovered profile = %#v", profile)
	}
	read, err := client.Get(context.Background(), "301")
	if err != nil || read.ID != "301" || read.SettingsID != "101" {
		t.Fatalf("exact read = %#v, %v", read, err)
	}
	patched, err := client.PatchProperties(context.Background(), "301", map[string]string{"hs_job_title": "Lead Engineer"})
	if err != nil || patched.JobTitle != "Lead Engineer" {
		t.Fatalf("patch = %#v, %v", patched, err)
	}

	properties := "hs_availability_status,hs_internal_user_id,hs_job_title,hs_standard_time_zone,hs_working_hours"
	want := []string{
		"GET /crm/objects/2026-03/users?limit=100&properties=hs_internal_user_id",
		"GET /crm/objects/2026-03/users?after=next-page&limit=100&properties=hs_internal_user_id",
		"GET /crm/objects/2026-03/users?limit=100&properties=hs_internal_user_id",
		"GET /crm/objects/2026-03/users/301?properties=" + properties,
		"PATCH /crm/objects/2026-03/users/301",
	}
	for index := range want {
		want[index] = string(bytes.ReplaceAll([]byte(want[index]), []byte(","), []byte("%2C")))
	}
	if !reflect.DeepEqual(requests, want) {
		t.Fatalf("requests = %#v, want %#v", requests, want)
	}
}
