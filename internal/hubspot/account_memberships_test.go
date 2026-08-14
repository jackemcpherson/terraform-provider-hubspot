// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

package hubspot

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestAccountMembershipClientListsEveryPageAndIgnoresUnknownFields(t *testing.T) {
	requests := make([]string, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests = append(requests, request.URL.RequestURI())
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Query().Get("after") {
		case "":
			io.WriteString(writer, `{"results":[{"id":"101","email":"first@example.com","firstName":"First","lastName":"Member","superAdmin":false,"roleIds":[],"primaryTeamId":null,"secondaryTeamIds":[],"seatNames":["core"]}],"paging":{"next":{"after":"cursor-2"}}}`)
		case "cursor-2":
			io.WriteString(writer, `{"results":[{"id":"202","email":"second@example.com","superAdmin":true,"roleIds":["role-7"],"primaryTeamId":"team-3","secondaryTeamIds":["team-4"],"futureField":{"ignored":true}}]}`)
		default:
			t.Fatalf("unexpected cursor %q", request.URL.Query().Get("after"))
		}
	}))
	defer server.Close()

	client := &AccountMembershipClient{transport: newTestTransport(t, server.URL)}
	memberships, err := client.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(memberships) != 2 || memberships[0].ID != "101" || memberships[1].ID != "202" {
		t.Fatalf("memberships = %#v", memberships)
	}
	if memberships[0].HasRoleOrTeamAssignments() {
		t.Fatal("empty role and team fields were treated as assignments")
	}
	if !memberships[1].HasRoleOrTeamAssignments() {
		t.Fatal("role and team fields were not detected")
	}
	wantRequests := []string{
		"/settings/users/2026-03?limit=100",
		"/settings/users/2026-03?after=cursor-2&limit=100",
	}
	if !reflect.DeepEqual(requests, wantRequests) {
		t.Fatalf("requests = %#v, want %#v", requests, wantRequests)
	}
}

func TestAccountMembershipClientUsesExactIdentityLifecycleRoutes(t *testing.T) {
	requests := make([]string, 0, 7)
	readByIDCount := 0
	listAfterDeleteCount := 0
	deleted := false
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests = append(requests, request.Method+" "+request.URL.RequestURI())
		writer.Header().Set("Content-Type", "application/json")
		switch request.Method + " " + request.URL.RequestURI() {
		case "POST /settings/users/2026-03":
			var got AccountMembershipCreate
			if err := json.NewDecoder(request.Body).Decode(&got); err != nil {
				t.Fatalf("decode create: %v", err)
			}
			want := AccountMembershipCreate{Email: "member+test@example.com", FirstName: "First", LastName: "Member", SendWelcomeEmail: true}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("create = %#v, want %#v", got, want)
			}
			writer.WriteHeader(http.StatusCreated)
			io.WriteString(writer, `{"id":"404","email":"member+test@example.com","firstName":"First","lastName":"Member","superAdmin":false,"seatNames":["core"]}`)
		case "GET /settings/users/2026-03/404":
			readByIDCount++
			if readByIDCount >= 2 {
				writeMembershipTestError(writer, http.StatusNotFound)
				return
			}
			io.WriteString(writer, `{"id":"404","email":"member+test@example.com","firstName":"First","lastName":"Member","superAdmin":false}`)
		case "GET /settings/users/2026-03/member+test@example.com?idProperty=EMAIL":
			if deleted {
				writeMembershipTestError(writer, http.StatusNotFound)
				return
			}
			io.WriteString(writer, `{"id":"404","email":"member+test@example.com","firstName":"First","lastName":"Member","superAdmin":false}`)
		case "PUT /settings/users/2026-03/404":
			body, err := io.ReadAll(request.Body)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(body, []byte(`{"firstName":"Updated","lastName":"Name"}`)) {
				t.Fatalf("name update body = %s", body)
			}
			io.WriteString(writer, `{"id":"404","email":"member+test@example.com","firstName":"Updated","lastName":"Name","superAdmin":false}`)
		case "DELETE /settings/users/2026-03/404":
			deleted = true
			writer.WriteHeader(http.StatusNoContent)
		case "GET /settings/users/2026-03?limit=100":
			listAfterDeleteCount++
			if listAfterDeleteCount == 1 {
				io.WriteString(writer, `{"results":[{"id":"404","email":"member+test@example.com"}]}`)
				return
			}
			io.WriteString(writer, `{"results":[]}`)
		default:
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.RequestURI())
		}
	}))
	defer server.Close()

	transport := newTestTransport(t, server.URL)
	transport.sleep = func(context.Context, time.Duration) error { return nil }
	client := &AccountMembershipClient{transport: transport}
	created, err := client.Create(context.Background(), AccountMembershipCreate{
		Email: "member+test@example.com", FirstName: "First", LastName: "Member", SendWelcomeEmail: true,
	})
	if err != nil || created.ID != "404" {
		t.Fatalf("create = %#v, %v", created, err)
	}
	if _, err := client.GetByID(context.Background(), "404"); err != nil {
		t.Fatal(err)
	}
	byEmail, err := client.GetByEmail(context.Background(), "member+test@example.com")
	if err != nil || byEmail.ID != "404" {
		t.Fatalf("email read = %#v, %v", byEmail, err)
	}
	updated, err := client.UpdateNames(context.Background(), "404", AccountMembershipNameUpdate{FirstName: "Updated", LastName: "Name"})
	if err != nil || updated.FirstName != "Updated" || updated.LastName != "Name" {
		t.Fatalf("update = %#v, %v", updated, err)
	}
	if err := client.Delete(context.Background(), "404"); err != nil {
		t.Fatal(err)
	}
	if err := client.WaitForAbsence(context.Background(), "404", "member+test@example.com"); err != nil {
		t.Fatal(err)
	}

	want := []string{
		"POST /settings/users/2026-03",
		"GET /settings/users/2026-03/404",
		"GET /settings/users/2026-03/member+test@example.com?idProperty=EMAIL",
		"PUT /settings/users/2026-03/404",
		"DELETE /settings/users/2026-03/404",
		"GET /settings/users/2026-03/404",
		"GET /settings/users/2026-03/member+test@example.com?idProperty=EMAIL",
		"GET /settings/users/2026-03?limit=100",
		"GET /settings/users/2026-03/404",
		"GET /settings/users/2026-03/member+test@example.com?idProperty=EMAIL",
		"GET /settings/users/2026-03?limit=100",
	}
	if !reflect.DeepEqual(requests, want) {
		t.Fatalf("requests = %#v, want %#v", requests, want)
	}
}

func TestAccountMembershipClientSurfacesActivationFailureWithoutRetry(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requestCount++
		writeMembershipTestErrorWithSubcategory(writer, http.StatusBadRequest, "USER_NOT_ON_ANY_HUBS")
	}))
	defer server.Close()

	client := &AccountMembershipClient{transport: newTestTransport(t, server.URL)}
	_, err := client.UpdateNames(context.Background(), "505", AccountMembershipNameUpdate{FirstName: "Active", LastName: "Required"})
	if err == nil || !strings.Contains(err.Error(), "USER_NOT_ON_ANY_HUBS") {
		t.Fatalf("activation error = %v", err)
	}
	if requestCount != 1 {
		t.Fatalf("activation failure request count = %d, want 1", requestCount)
	}
}

func TestAccountMembershipClientRejectsMalformedPagination(t *testing.T) {
	tests := map[string]string{
		"missing id":      `{"results":[{"email":"member@example.com"}]}`,
		"repeated cursor": `{"results":[{"id":"101","email":"member@example.com"}],"paging":{"next":{"after":"same"}}}`,
	}
	for name, responseBody := range tests {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writer.Header().Set("Content-Type", "application/json")
				io.WriteString(writer, responseBody)
			}))
			defer server.Close()
			client := &AccountMembershipClient{transport: newTestTransport(t, server.URL)}
			if _, err := client.List(context.Background()); err == nil {
				t.Fatal("malformed account membership pagination was accepted")
			}
		})
	}
}

func TestAccountMembershipAbsenceRequiresHTTP404(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		requests++
		writeMembershipTestErrorWithSubcategory(writer, http.StatusForbidden, "MISSING_SCOPES")
	}))
	defer server.Close()
	transport := newTestTransport(t, server.URL)
	transport.sleep = func(context.Context, time.Duration) error {
		t.Fatal("non-404 absence check was retried")
		return nil
	}
	client := &AccountMembershipClient{transport: transport}
	if err := client.WaitForAbsence(context.Background(), "101", "member@example.com"); err == nil {
		t.Fatal("non-404 response proved account membership absence")
	}
	if requests != 1 {
		t.Fatalf("non-404 absence requests = %d, want 1", requests)
	}
}

func writeMembershipTestError(writer http.ResponseWriter, status int) {
	writeMembershipTestErrorWithSubcategory(writer, status, "OBJECT_NOT_FOUND")
}

func writeMembershipTestErrorWithSubcategory(writer http.ResponseWriter, status int, subcategory string) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	io.WriteString(writer, `{"status":"error","category":"VALIDATION_ERROR","subCategory":"`+subcategory+`","message":"`+subcategory+`"}`)
}
