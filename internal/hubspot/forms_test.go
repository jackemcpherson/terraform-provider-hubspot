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
)

func TestFormClientUsesGeneratedIDLifecycleRoutes(t *testing.T) {
	wantWrite := canonicalFormWriteForTest()
	requests := make([]string, 0, 4)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests = append(requests, request.Method+" "+request.URL.RequestURI())
		writer.Header().Set("Content-Type", "application/json")
		switch request.Method {
		case http.MethodPost:
			var got FormDefinitionWrite
			if err := json.NewDecoder(request.Body).Decode(&got); err != nil {
				t.Fatalf("decode create payload: %v", err)
			}
			if !reflect.DeepEqual(got, wantWrite) {
				t.Fatalf("create payload = %#v, want %#v", got, wantWrite)
			}
			writer.WriteHeader(http.StatusCreated)
			io.WriteString(writer, `{"id":"generated-form-7","name":"Managed form","formType":"hubspot"}`)
		case http.MethodGet:
			archived := request.URL.Query().Get("archived") == "true"
			io.WriteString(writer, `{"id":"generated-form-7","name":"Managed form","formType":"hubspot","archived":`+boolJSON(archived)+`}`)
		case http.MethodDelete:
			writer.WriteHeader(http.StatusNoContent)
		}
	}))
	defer server.Close()

	client := &FormClient{transport: newTestTransport(t, server.URL)}
	created, err := client.Create(context.Background(), wantWrite)
	if err != nil {
		t.Fatal(err)
	}
	if created.ID != "generated-form-7" {
		t.Fatalf("generated ID = %q", created.ID)
	}
	active, err := client.Get(context.Background(), created.ID)
	if err != nil || active.ID != created.ID || active.Archived {
		t.Fatalf("active read = %#v, %v", active, err)
	}
	archived, err := client.GetArchived(context.Background(), created.ID)
	if err != nil || archived.ID != created.ID || !archived.Archived {
		t.Fatalf("archived read = %#v, %v", archived, err)
	}
	if err := client.Archive(context.Background(), created.ID); err != nil {
		t.Fatal(err)
	}

	wantRequests := []string{
		"POST /marketing/v3/forms",
		"GET /marketing/v3/forms/generated-form-7",
		"GET /marketing/v3/forms/generated-form-7?archived=true",
		"DELETE /marketing/v3/forms/generated-form-7",
	}
	if !reflect.DeepEqual(requests, wantRequests) {
		t.Fatalf("requests = %#v, want %#v", requests, wantRequests)
	}
}

func TestFormClientPatchContainsOnlySelectedManagedSubtrees(t *testing.T) {
	var payload []byte
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPatch || request.URL.RequestURI() != "/marketing/v3/forms/generated-form-7" {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.RequestURI())
		}
		payload, _ = io.ReadAll(request.Body)
		writer.Header().Set("Content-Type", "application/json")
		io.WriteString(writer, `{"id":"generated-form-7","name":"Updated"}`)
	}))
	defer server.Close()

	name := "Updated"
	display := canonicalFormWriteForTest().DisplayOptions
	display.SubmitButtonText = "Send"
	client := &FormClient{transport: newTestTransport(t, server.URL)}
	updated, err := client.Update(context.Background(), "generated-form-7", FormDefinitionPatch{
		Name: &name, DisplayOptions: &display,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.ID != "generated-form-7" {
		t.Fatalf("updated ID = %q", updated.ID)
	}

	var document map[string]json.RawMessage
	if err := json.Unmarshal(payload, &document); err != nil {
		t.Fatal(err)
	}
	if len(document) != 2 || !bytes.Equal(document["name"], []byte(`"Updated"`)) || len(document["displayOptions"]) == 0 {
		t.Fatalf("bounded patch = %s", payload)
	}
	for _, excluded := range []string{"id", "formType", "createdAt", "updatedAt", "archived", "configuration", "fieldGroups", "legalConsentOptions"} {
		if strings.Contains(string(payload), `"`+excluded+`"`) {
			t.Fatalf("patch included excluded field %q: %s", excluded, payload)
		}
	}
}

func TestFormClientRejectsResponsesWithoutGeneratedID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusCreated)
		io.WriteString(writer, `{"name":"Managed form"}`)
	}))
	defer server.Close()

	client := &FormClient{transport: newTestTransport(t, server.URL)}
	if _, err := client.Create(context.Background(), canonicalFormWriteForTest()); err == nil {
		t.Fatal("create accepted a response without the generated form ID")
	}
}

func canonicalFormWriteForTest() FormDefinitionWrite {
	return FormDefinitionWrite{
		FormType: "hubspot",
		Name:     "Managed form",
		FieldGroups: []FormFieldGroup{{
			GroupType: "default_group", RichTextType: "text",
			Fields: []FormField{{
				ObjectTypeID: "0-1", Hidden: false, Name: "email", DependentFields: []FormDependentField{},
				Label: "Email address", FieldType: "email", Required: true,
				Validation:  FormFieldValidation{BlockedEmailDomains: []string{}, UseDefaultBlockList: true},
				Description: "Contact email", Placeholder: "name@example.com",
			}},
		}},
		Configuration: FormConfiguration{
			CreateNewContactForNewEmail: false, Editable: true, AllowLinkToResetKnownValues: false,
			PostSubmitAction: FormPostSubmitAction{Type: "thank_you", Value: "Thank you"},
			Language:         "en", PrePopulateKnownValues: false, Cloneable: true, NotifyContactOwner: false,
			RecaptchaEnabled: true, Archivable: true, NotifyRecipients: []string{},
		},
		DisplayOptions: FormDisplayOptions{
			RenderRawHTML: false, Theme: "default_style", SubmitButtonText: "Submit",
			Style: FormStyle{
				LabelTextSize: "13px", LegalConsentTextColor: "#33475b", FontFamily: "Arial, sans-serif",
				LegalConsentTextSize: "12px", BackgroundWidth: "100%", HelpTextSize: "11px",
				SubmitFontColor: "#ffffff", LabelTextColor: "#33475b", SubmitAlignment: "left",
				SubmitSize: "12px 24px", HelpTextColor: "#516f90", SubmitColor: "#ff7a59",
			},
		},
		LegalConsentOptions: FormLegalConsentOptions{Type: "none"},
	}
}

func boolJSON(value bool) string {
	if value {
		return "true"
	}
	return "false"
}
