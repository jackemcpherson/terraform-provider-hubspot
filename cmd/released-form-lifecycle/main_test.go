// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

//go:build acceptance

package main

import (
	"context"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/jackemcpherson/terraform-provider-hubspot/internal/acceptance"
	"github.com/jackemcpherson/terraform-provider-hubspot/internal/hubspot"
)

func TestReleasedFormActionsPreserveExactIdentityAndTerminalHash(t *testing.T) {
	fake := acceptance.NewFakeHubSpot("token", 123)
	server := httptest.NewServer(fake)
	defer server.Close()
	origin, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	clients, err := hubspot.NewClientSet(hubspot.TransportConfig{BaseURL: origin, AccessToken: "token"})
	if err != nil {
		t.Fatal(err)
	}
	const prefix = "tf_acc_released_"
	form, err := clients.Forms.Create(context.Background(), releasedFormFixture(prefix+"released_form"))
	if err != nil {
		t.Fatal(err)
	}
	for _, action := range []string{"verify-active", "drift", "verify-active", "archive"} {
		if _, err := execute(context.Background(), action, form.ID, prefix, clients); err != nil {
			t.Fatalf("%s: %v", action, err)
		}
	}
	record, err := execute(context.Background(), "verify-terminal", form.ID, prefix, clients)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(record, form.ID) || !strings.Contains(record, `"terminal":"archived"`) || !strings.Contains(record, `"active_owned_forms":0`) {
		t.Fatalf("unsafe terminal record: %s", record)
	}
	if fake.FormCreateCount() != 1 || fake.FormDeleteCount(form.ID) != 1 {
		t.Fatalf("creates=%d deletes=%d, want one each", fake.FormCreateCount(), fake.FormDeleteCount(form.ID))
	}
}

func TestReleasedFormActionsRejectUnsafeOwnership(t *testing.T) {
	clients := &hubspot.ClientSet{}
	if _, err := execute(context.Background(), "verify-active", "id", "released_", clients); err == nil {
		t.Fatal("unsafe prefix accepted")
	}
	if _, err := execute(context.Background(), "unknown", "id", "tf_acc_released_", clients); err == nil {
		t.Fatal("unknown action accepted")
	}
}

func releasedFormFixture(name string) hubspot.FormDefinitionWrite {
	return hubspot.FormDefinitionWrite{
		FormType: "hubspot", Name: name,
		FieldGroups: []hubspot.FormFieldGroup{{GroupType: "default_group", RichTextType: "text", Fields: []hubspot.FormField{{
			ObjectTypeID: "0-1", Name: "email", DependentFields: []hubspot.FormDependentField{}, Label: "Email address",
			FieldType: "email", Required: true, Description: "Contact email", Placeholder: "name@example.com",
			Validation: hubspot.FormFieldValidation{BlockedEmailDomains: []string{}, UseDefaultBlockList: true},
		}}}},
		Configuration: hubspot.FormConfiguration{Editable: true, Language: "en", Cloneable: true, RecaptchaEnabled: true, Archivable: true,
			PostSubmitAction: hubspot.FormPostSubmitAction{Type: "thank_you", Value: "Thank you"}, NotifyRecipients: []string{}},
		DisplayOptions: hubspot.FormDisplayOptions{Theme: "default_style", SubmitButtonText: "Submit", Style: hubspot.FormStyle{
			LabelTextSize: "13px", LabelTextColor: "#33475b", LegalConsentTextSize: "12px", LegalConsentTextColor: "#33475b",
			HelpTextSize: "11px", HelpTextColor: "#516f90", FontFamily: "Arial, sans-serif", BackgroundWidth: "100%",
			SubmitFontColor: "#ffffff", SubmitAlignment: "left", SubmitSize: "12px 24px", SubmitColor: "#ff7a59",
		}},
		LegalConsentOptions: hubspot.FormLegalConsentOptions{Type: "none"},
	}
}
