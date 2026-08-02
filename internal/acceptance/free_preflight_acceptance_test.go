// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

//go:build acceptance

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

func TestAcc_free_properties_QuotaPreflight(t *testing.T) {
	requireAcceptanceEnabled(t)
	token := requiredEnvironment(t, "HUBSPOT_ACCESS_TOKEN")
	origin, err := url.Parse("https://api.hubapi.com")
	if err != nil {
		t.Fatal("parse HubSpot API origin")
	}
	transport, err := hubspot.NewTransport(hubspot.TransportConfig{
		BaseURL: origin, AccessToken: token, UserAgent: "terraform-provider-hubspot/acceptance-preflight",
	})
	if err != nil {
		t.Fatal("configure HubSpot quota preflight")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	var limits struct {
		OverallLimit int64 `json:"overallLimit"`
		OverallUsage int64 `json:"overallUsage"`
	}
	if err := transport.Do(ctx, hubspot.Operation{
		Name: "custom-property-limit-read", Method: http.MethodGet,
		Path: "/crm/limits/2026-03/custom-properties", Replay: hubspot.ReplaySafe,
	}, nil, &limits); err != nil {
		t.Logf("custom-property quota telemetry unavailable; remote create remains authoritative: %s", acceptance.SanitizedHubSpotError(err))
		return
	}
	if limits.OverallLimit <= 0 || limits.OverallUsage < 0 || limits.OverallUsage > limits.OverallLimit {
		t.Log("custom-property quota telemetry was omitted or inconsistent; remote create remains authoritative")
		return
	}
	t.Logf("custom-property quota telemetry: overall usage %d of %d; advisory only", limits.OverallUsage, limits.OverallLimit)
}
