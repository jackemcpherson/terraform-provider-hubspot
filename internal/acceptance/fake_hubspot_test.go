// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

//go:build acceptance

package acceptance_test

import (
	"context"
	"errors"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/jackemcpherson/terraform-provider-hubspot/internal/acceptance"
	"github.com/jackemcpherson/terraform-provider-hubspot/internal/hubspot"
)

func newFakeHubSpotClients(t *testing.T, fake *acceptance.FakeHubSpot, token string) *hubspot.ClientSet {
	t.Helper()
	server := httptest.NewServer(fake)
	t.Cleanup(server.Close)
	origin, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	clients, err := hubspot.NewClientSet(hubspot.TransportConfig{BaseURL: origin, AccessToken: token, UserAgent: "fake-hubspot-test"})
	if err != nil {
		t.Fatal(err)
	}
	return clients
}

func TestFakeHubSpotRejectsWrongBearerToken(t *testing.T) {
	fake := acceptance.NewFakeHubSpot("correct-token", 42)
	clients := newFakeHubSpotClients(t, fake, "wrong-token")
	_, err := clients.AccountInfo.Get(context.Background())
	var apiError *hubspot.Error
	if !errors.As(err, &apiError) || apiError.Status != 401 {
		t.Fatalf("error = %#v, want HTTP 401", err)
	}
}

func TestFakeHubSpotAccountInfoReportsConfiguredPortal(t *testing.T) {
	fake := acceptance.NewFakeHubSpot("sentinel", 987654)
	clients := newFakeHubSpotClients(t, fake, "sentinel")
	info, err := clients.AccountInfo.Get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if info.PortalID != 987654 {
		t.Fatalf("portal id = %d, want 987654", info.PortalID)
	}
}

func TestFakeHubSpotPropertyGroupArchivalIsNotDeletionButBecomesUnreadable(t *testing.T) {
	fake := acceptance.NewFakeHubSpot("sentinel", 1)
	clients := newFakeHubSpotClients(t, fake, "sentinel")
	ctx := context.Background()

	if _, err := clients.PropertyGroups.Create(ctx, "contacts", hubspot.PropertyGroupCreate{Name: "marketing", Label: "Marketing", DisplayOrder: -1}); err != nil {
		t.Fatal(err)
	}
	if err := clients.PropertyGroups.Archive(ctx, "contacts", "marketing"); err != nil {
		t.Fatal(err)
	}
	if _, err := clients.PropertyGroups.Get(ctx, "contacts", "marketing"); err == nil {
		t.Fatal("expected archived group to be unreadable, matching the harness's absence probes")
	} else {
		var apiError *hubspot.Error
		if !errors.As(err, &apiError) || apiError.Status != 404 {
			t.Fatalf("error = %#v, want HTTP 404", err)
		}
	}
	// Archived groups are not name-reserved: recreation must succeed.
	if _, err := clients.PropertyGroups.Create(ctx, "contacts", hubspot.PropertyGroupCreate{Name: "marketing", Label: "Marketing again", DisplayOrder: -1}); err != nil {
		t.Fatalf("recreate archived group name: %v", err)
	}
}

func TestFakeHubSpotPropertyGroupDuplicateCreateConflicts(t *testing.T) {
	fake := acceptance.NewFakeHubSpot("sentinel", 1)
	clients := newFakeHubSpotClients(t, fake, "sentinel")
	ctx := context.Background()

	if _, err := clients.PropertyGroups.Create(ctx, "contacts", hubspot.PropertyGroupCreate{Name: "marketing", Label: "Marketing", DisplayOrder: -1}); err != nil {
		t.Fatal(err)
	}
	_, err := clients.PropertyGroups.Create(ctx, "contacts", hubspot.PropertyGroupCreate{Name: "marketing", Label: "Marketing", DisplayOrder: -1})
	var apiError *hubspot.Error
	if !errors.As(err, &apiError) || apiError.Status != 409 {
		t.Fatalf("error = %#v, want HTTP 409", err)
	}
}

func TestFakeHubSpotGroupDeletionFailsWithActiveProperties(t *testing.T) {
	fake := acceptance.NewFakeHubSpot("sentinel", 1)
	clients := newFakeHubSpotClients(t, fake, "sentinel")
	ctx := context.Background()

	if _, err := clients.PropertyGroups.Create(ctx, "contacts", hubspot.PropertyGroupCreate{Name: "marketing", Label: "Marketing", DisplayOrder: -1}); err != nil {
		t.Fatal(err)
	}
	groupName := "marketing"
	if _, err := clients.Properties.Create(ctx, "contacts", hubspot.PropertyWrite{Name: "tier", Label: "Tier", GroupName: groupName, Type: "string", FieldType: "text"}); err != nil {
		t.Fatal(err)
	}

	err := clients.PropertyGroups.Archive(ctx, "contacts", "marketing")
	if acceptance.SanitizedHubSpotError(err) != string(acceptance.PropertyGroupHasActiveProperties) {
		t.Fatalf("SanitizedHubSpotError(err) = %q, want %q", acceptance.SanitizedHubSpotError(err), acceptance.PropertyGroupHasActiveProperties)
	}

	// Archiving the owning property clears the way for group archival.
	if err := clients.Properties.Archive(ctx, "contacts", "tier"); err != nil {
		t.Fatal(err)
	}
	if err := clients.PropertyGroups.Archive(ctx, "contacts", "marketing"); err != nil {
		t.Fatalf("archive group after its only property was archived: %v", err)
	}
}

func TestFakeHubSpotPropertyArchivedNameIsImmediatelyReusable(t *testing.T) {
	fake := acceptance.NewFakeHubSpot("sentinel", 1)
	clients := newFakeHubSpotClients(t, fake, "sentinel")
	ctx := context.Background()

	if _, err := clients.PropertyGroups.Create(ctx, "contacts", hubspot.PropertyGroupCreate{Name: "marketing", Label: "Marketing", DisplayOrder: -1}); err != nil {
		t.Fatal(err)
	}
	if _, err := clients.Properties.Create(ctx, "contacts", hubspot.PropertyWrite{Name: "tier", Label: "Tier", GroupName: "marketing", Type: "string", FieldType: "text"}); err != nil {
		t.Fatal(err)
	}
	if err := clients.Properties.Archive(ctx, "contacts", "tier"); err != nil {
		t.Fatal(err)
	}

	// Current /2026-03 behavior permits immediate reuse while preserving the
	// archived definition as a historical tombstone.
	active, err := clients.Properties.Create(ctx, "contacts", hubspot.PropertyWrite{Name: "tier", Label: "Tier again", GroupName: "marketing", Type: "string", FieldType: "text"})
	if err != nil {
		t.Fatalf("recreate archived property name: %v", err)
	}
	if active.Label != "Tier again" || boolPointerValue(active.Archived) {
		t.Fatalf("active property = %#v, want recreated active definition", active)
	}

	// The archived definition itself is still readable through the
	// archived-visibility query.
	archived, err := clients.Properties.Get(ctx, "contacts", "tier", true, "non_sensitive", "")
	if err != nil {
		t.Fatalf("read archived property definition: %v", err)
	}
	if archived.Archived == nil || !*archived.Archived {
		t.Fatal("archived property definition did not report archived=true")
	}
	if archived.Label != "Tier" {
		t.Fatalf("archived label = %q, want original tombstone", archived.Label)
	}
	current, err := clients.Properties.Get(ctx, "contacts", "tier", false, "non_sensitive", "")
	if err != nil {
		t.Fatalf("read recreated active property: %v", err)
	}
	if current.Label != "Tier again" {
		t.Fatalf("active label = %q, want recreated definition", current.Label)
	}
}

func TestFakeHubSpotGeneratesAppendOrdersWithoutMutatingClientOptions(t *testing.T) {
	fake := acceptance.NewFakeHubSpot("sentinel", 1)
	clients := newFakeHubSpotClients(t, fake, "sentinel")
	ctx := context.Background()

	group, err := clients.PropertyGroups.Create(ctx, "contacts", hubspot.PropertyGroupCreate{Name: "marketing", Label: "Marketing", DisplayOrder: -1})
	if err != nil {
		t.Fatal(err)
	}
	if group.DisplayOrder < 0 {
		t.Fatalf("group display order = %d, want generated nonnegative order", group.DisplayOrder)
	}
	appendOrder := int64(-1)
	options := []hubspot.PropertyOption{
		{Value: "alpha", Label: "Alpha", DisplayOrder: &appendOrder},
		{Value: "beta", Label: "Beta", DisplayOrder: &appendOrder},
	}
	property, err := clients.Properties.Create(ctx, "contacts", hubspot.PropertyWrite{
		Name: "tier", Label: "Tier", GroupName: "marketing", Type: "enumeration", FieldType: "select",
		DisplayOrder: &appendOrder, Options: options,
	})
	if err != nil {
		t.Fatal(err)
	}
	if property.DisplayOrder == nil || *property.DisplayOrder < 0 {
		t.Fatalf("property display order = %#v, want generated nonnegative order", property.DisplayOrder)
	}
	for _, option := range property.Options {
		if option.DisplayOrder == nil || *option.DisplayOrder < 0 {
			t.Fatalf("option display order = %#v, want generated nonnegative order", option.DisplayOrder)
		}
	}
	if *options[0].DisplayOrder != -1 || *options[1].DisplayOrder != -1 {
		t.Fatalf("client options mutated: %#v", options)
	}
}

func boolPointerValue(value *bool) bool {
	return value != nil && *value
}

func TestFakeHubSpotPipelineStageNormalizationCopiesClientMetadata(t *testing.T) {
	fake := acceptance.NewFakeHubSpot("sentinel", 1)
	clients := newFakeHubSpotClients(t, fake, "sentinel")
	ctx := context.Background()

	clientMetadata := map[string]string{"probability": "0.25"}
	created, err := clients.Pipelines.Create(ctx, "deals", hubspot.PipelineWrite{
		Label: "Sales", DisplayOrder: -1,
		Stages: []hubspot.PipelineStageWrite{
			{Label: "Open", DisplayOrder: 10, Metadata: clientMetadata},
			{Label: "Won", DisplayOrder: 20, Metadata: nil},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	// The client's own map must be untouched by the fake's normalization.
	if len(clientMetadata) != 1 || clientMetadata["probability"] != "0.25" {
		t.Fatalf("client metadata mutated: %#v", clientMetadata)
	}
	if len(created.Stages) != 2 {
		t.Fatalf("stages = %#v", created.Stages)
	}
	for _, stage := range created.Stages {
		if stage.ID == "" {
			t.Fatal("expected the fake to assign a server-side stage identity")
		}
		if stage.Metadata["probability"] == "" {
			t.Fatalf("expected a deal stage to carry an injected probability default: %#v", stage.Metadata)
		}
	}
	if created.Stages[0].Metadata["probability"] != "0.25" {
		t.Fatalf("client-supplied probability was not preserved: %#v", created.Stages[0].Metadata)
	}
	if created.Stages[1].Metadata["probability"] != "0.0" {
		t.Fatalf("missing probability was not defaulted: %#v", created.Stages[1].Metadata)
	}

	ticket, err := clients.Pipelines.Create(ctx, "tickets", hubspot.PipelineWrite{
		Label: "Support", DisplayOrder: -1,
		Stages: []hubspot.PipelineStageWrite{{Label: "Open", DisplayOrder: 10}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if ticket.Stages[0].Metadata["ticketState"] != "OPEN" {
		t.Fatalf("missing ticketState was not defaulted: %#v", ticket.Stages[0].Metadata)
	}
}

func TestFakeHubSpotPipelineArchivalUsesArchivedVisibilityQuery(t *testing.T) {
	fake := acceptance.NewFakeHubSpot("sentinel", 1)
	clients := newFakeHubSpotClients(t, fake, "sentinel")
	ctx := context.Background()

	created, err := clients.Pipelines.Create(ctx, "deals", hubspot.PipelineWrite{
		Label: "Sales", DisplayOrder: -1,
		Stages: []hubspot.PipelineStageWrite{{Label: "Open", DisplayOrder: 10}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := clients.Pipelines.Archive(ctx, "deals", created.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := clients.Pipelines.Get(ctx, "deals", created.ID); err == nil {
		t.Fatal("expected the active view to omit an archived pipeline")
	}
	archived, err := clients.Pipelines.GetArchived(ctx, "deals", created.ID)
	if err != nil {
		t.Fatalf("read archived pipeline: %v", err)
	}
	if !archived.Archived || archived.ID != created.ID {
		t.Fatalf("archived pipeline = %#v", archived)
	}
}
