// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

//go:build acceptance

package main

import (
	"context"
	"net/http/httptest"
	"net/url"
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
	if err := execute(ctx, "drift", clients); err != nil {
		t.Fatal(err)
	}
	drifted, err := clients.Properties.Get(ctx, "contacts", "ns_buyer_role", false, "non_sensitive", "")
	if err != nil || drifted.Label != driftLabel {
		t.Fatalf("drifted property = %#v, %v", drifted, err)
	}
	if err := execute(ctx, "archive-for-refresh", clients); err != nil {
		t.Fatal(err)
	}
}

func TestExecuteRejectsUnknownAction(t *testing.T) {
	if err := execute(context.Background(), "unknown", &hubspot.ClientSet{}); err == nil {
		t.Fatal("unknown action accepted")
	}
}
