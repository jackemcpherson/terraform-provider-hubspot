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

const freePropertyOverallHeadroom = 10

var freePropertyObjectHeadroom = map[string]int64{
	"0-1": 3,
	"0-2": 1,
	"0-3": 1,
	"0-5": 1,
}

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
		ByObjectType []struct {
			ObjectTypeID string `json:"objectTypeId"`
			Limit        int64  `json:"limit"`
			Usage        int64  `json:"usage"`
		} `json:"byObjectType"`
	}
	if err := transport.Do(ctx, hubspot.Operation{
		Name: "custom-property-limit-read", Method: http.MethodGet,
		Path: "/crm/limits/2026-03/custom-properties", Replay: hubspot.ReplaySafe,
	}, nil, &limits); err != nil {
		t.Fatalf("custom-property quota preflight failed: %s", acceptance.SanitizedHubSpotError(err))
	}
	if limits.OverallLimit-limits.OverallUsage < freePropertyOverallHeadroom {
		t.Fatal("custom-property quota preflight found insufficient overall headroom")
	}
	seen := make(map[string]bool, len(freePropertyObjectHeadroom))
	for _, object := range limits.ByObjectType {
		headroom, required := freePropertyObjectHeadroom[object.ObjectTypeID]
		if !required {
			continue
		}
		if object.Limit-object.Usage < headroom {
			t.Fatalf("custom-property quota preflight found insufficient headroom for object type %s", object.ObjectTypeID)
		}
		seen[object.ObjectTypeID] = true
	}
	for objectTypeID := range freePropertyObjectHeadroom {
		if !seen[objectTypeID] {
			t.Fatalf("custom-property quota preflight did not return object type %s limits", objectTypeID)
		}
	}
}
