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

func TestExecuteAuthorsDriftAndArchivesRefreshTarget(t *testing.T) {
	server := httptest.NewServer(acceptance.NewFakeHubSpot("token", 123))
	defer server.Close()
	origin, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	clients, err := hubspot.NewClientSet(hubspot.TransportConfig{BaseURL: origin, AccessToken: "token"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	order := int64(1)
	sensitivity := "non_sensitive"
	if _, err := clients.PropertyGroups.Create(ctx, "contacts", hubspot.PropertyGroupCreate{Name: "ns_customer_context", Label: "Northstar", DisplayOrder: order}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"ns_buyer_role", "ns_last_success_review"} {
		if _, err := clients.Properties.Create(ctx, "contacts", hubspot.PropertyWrite{
			Name: name, Label: name, GroupName: "ns_customer_context", Type: "string", FieldType: "text", DataSensitivity: &sensitivity,
		}); err != nil {
			t.Fatal(err)
		}
	}
	form, err := clients.Forms.Create(ctx, northstarFormFixture())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := execute(ctx, "drift", form.ID, clients); err != nil {
		t.Fatal(err)
	}
	drifted, err := clients.Properties.Get(ctx, "contacts", "ns_buyer_role", false, "non_sensitive", "")
	if err != nil || drifted.Label != driftLabel {
		t.Fatalf("drifted property = %#v, %v", drifted, err)
	}
	driftedForm, err := clients.Forms.Get(ctx, form.ID)
	if err != nil || driftedForm.Name != "ns_contact_us_drift" || driftedForm.FieldGroups[0].Fields[0].Label != "Out-of-band work email" {
		t.Fatalf("drifted form = %#v, %v", driftedForm, err)
	}
	if _, err := execute(ctx, "archive-for-refresh", "", clients); err != nil {
		t.Fatal(err)
	}
	if err := clients.Forms.Archive(ctx, form.ID); err != nil {
		t.Fatal(err)
	}
	record, err := execute(ctx, "verify-form-terminal", form.ID, clients)
	if err != nil || record == "" || strings.Contains(record, form.ID) || !strings.Contains(record, `"terminal":"archived"`) {
		t.Fatalf("terminal record = %q, %v", record, err)
	}
}

func TestExecuteRejectsUnknownAction(t *testing.T) {
	if _, err := execute(context.Background(), "unknown", "", &hubspot.ClientSet{}); err == nil {
		t.Fatal("unknown action accepted")
	}
}

func northstarFormFixture() hubspot.FormDefinitionWrite {
	return hubspot.FormDefinitionWrite{
		FormType: "hubspot", Name: "ns_contact_us",
		FieldGroups: []hubspot.FormFieldGroup{{GroupType: "default_group", RichTextType: "text", Fields: []hubspot.FormField{{
			ObjectTypeID: "0-1", Name: "email", DependentFields: []hubspot.FormDependentField{}, Label: "Work email",
			FieldType: "email", Required: true, Description: "Contact email", Placeholder: "you@company.example",
			Validation: hubspot.FormFieldValidation{BlockedEmailDomains: []string{}, UseDefaultBlockList: true},
		}}}},
		Configuration: hubspot.FormConfiguration{Editable: true, Language: "en", Cloneable: true, RecaptchaEnabled: true, Archivable: true,
			PostSubmitAction: hubspot.FormPostSubmitAction{Type: "thank_you", Value: "Thank you"}, NotifyRecipients: []string{}},
		DisplayOptions: hubspot.FormDisplayOptions{Theme: "default_style", SubmitButtonText: "Contact Northstar", Style: hubspot.FormStyle{
			LabelTextSize: "13px", LabelTextColor: "#33475b", LegalConsentTextSize: "12px", LegalConsentTextColor: "#33475b",
			HelpTextSize: "11px", HelpTextColor: "#516f90", FontFamily: "Arial, sans-serif", BackgroundWidth: "100%",
			SubmitFontColor: "#ffffff", SubmitAlignment: "center", SubmitSize: "12px 24px", SubmitColor: "#00a4bd",
		}},
		LegalConsentOptions: hubspot.FormLegalConsentOptions{Type: "none"},
	}
}
