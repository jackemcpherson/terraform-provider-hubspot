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

func TestAcc_files_configuration_CapabilityPreflight(t *testing.T) {
	requireAcceptanceEnabled(t)
	token := requiredEnvironment(t, "HUBSPOT_ACCESS_TOKEN")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	clients, err := acceptance.NewRealPortalClientSet(ctx, token, "terraform-provider-hubspot/files-acceptance-preflight")
	if err != nil {
		t.Fatalf("Files capability preflight failed: %s", acceptance.SanitizedHubSpotError(err))
	}
	if _, err := clients.FileFolders.Search(ctx, nil, ""); err != nil {
		t.Fatalf("File folder read preflight failed: %s", acceptance.SanitizedHubSpotError(err))
	}
	if _, err := clients.Files.Search(ctx, nil, ""); err != nil {
		t.Fatalf("Managed file read preflight failed: %s", acceptance.SanitizedHubSpotError(err))
	}
}
