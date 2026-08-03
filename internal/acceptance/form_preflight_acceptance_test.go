// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

//go:build acceptance

package acceptance_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackemcpherson/terraform-provider-hubspot/internal/acceptance"
)

func TestAcc_form_definitions_CapabilityPreflight(t *testing.T) {
	requireAcceptanceEnabled(t)
	token := requiredEnvironment(t, "HUBSPOT_ACCESS_TOKEN")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	clients, err := acceptance.NewRealPortalClientSet(ctx, token, "terraform-provider-hubspot/forms-acceptance-preflight")
	if err != nil {
		t.Fatalf("Forms capability preflight failed: %s", acceptance.SanitizedHubSpotError(err))
	}
	if _, err := clients.Forms.List(ctx, false); err != nil {
		t.Fatalf("active Forms read preflight failed: %s", acceptance.SanitizedHubSpotError(err))
	}
	if _, err := clients.Forms.List(ctx, true); err != nil {
		t.Fatalf("archived Forms read preflight failed: %s", acceptance.SanitizedHubSpotError(err))
	}
}
