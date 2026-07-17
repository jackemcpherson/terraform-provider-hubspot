// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

//go:build acceptance && deferred

package acceptance_test

import (
	"context"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/jackemcpherson/terraform-provider-hubspot/internal/acceptance"
	"github.com/jackemcpherson/terraform-provider-hubspot/internal/hubspot"
)

func TestAcc_ticket_pipelines_QuotaPreflight(t *testing.T) {
	requireAcceptanceEnabled(t)
	token := requiredEnvironment(t, "HUBSPOT_ACCESS_TOKEN")
	origin, err := url.Parse("https://api.hubapi.com")
	if err != nil {
		t.Fatal("parse HubSpot API origin")
	}
	transport, err := hubspot.NewTransport(hubspot.TransportConfig{BaseURL: origin, AccessToken: token, UserAgent: "terraform-provider-hubspot/ticket-pipeline-preflight"})
	if err != nil {
		t.Fatal("configure HubSpot ticket-pipeline preflight")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	var limits struct {
		HubSpotDefinedObjectTypes []struct {
			ObjectTypeID string `json:"objectTypeId"`
			Limit        int64  `json:"limit"`
			Usage        int64  `json:"usage"`
		} `json:"hubspotDefinedObjectTypes"`
	}
	if err := transport.Do(ctx, hubspot.Operation{Name: "ticket-pipeline-limit-read", Method: http.MethodGet, Path: "/crm/limits/2026-03/pipelines", Replay: hubspot.ReplaySafe}, nil, &limits); err != nil {
		t.Fatalf("ticket-pipeline quota preflight failed: %s", acceptance.SanitizedHubSpotError(err))
	}
	found := false
	for _, object := range limits.HubSpotDefinedObjectTypes {
		if object.ObjectTypeID == "0-5" {
			found = true
			if object.Limit-object.Usage < 1 {
				t.Fatal("ticket-pipeline quota preflight found insufficient headroom")
			}
		}
	}
	if !found {
		t.Fatal("ticket-pipeline quota preflight did not return ticket limits")
	}
	clients, err := hubspot.NewClientSet(hubspot.TransportConfig{BaseURL: origin, AccessToken: token, UserAgent: "terraform-provider-hubspot/ticket-pipeline-preflight"})
	if err != nil {
		t.Fatal("configure HubSpot ticket-pipeline read preflight")
	}
	if _, err := clients.Pipelines.List(ctx, "tickets"); err != nil {
		t.Fatalf("ticket-pipeline read preflight failed: %s", acceptance.SanitizedHubSpotError(err))
	}
	t.Fatal("ticket-pipeline qualification is blocked until the referenced-ticket failure and cleanup contract is measured on an eligible Service Hub account")
}
