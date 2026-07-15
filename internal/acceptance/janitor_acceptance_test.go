// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

//go:build acceptance

package acceptance_test

import (
	"context"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/jackemcpherson/terraform-provider-hubspot/internal/acceptance"
	"github.com/jackemcpherson/terraform-provider-hubspot/internal/hubspot"
)

func TestAcc_JanitorReport(t *testing.T) {
	clients := freeJanitorClients(t)
	prefix := requiredEnvironment(t, "HUBSPOT_ACCEPTANCE_PREFIX")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	propertyCount, groupCount := countFreeOwnedConfiguration(t, ctx, clients, prefix)
	t.Logf("stale owned CRM configuration: property_definitions=%d property_groups=%d", propertyCount, groupCount)
}

func TestAcc_ManualPrefixCleanup(t *testing.T) {
	clients := freeJanitorClients(t)
	prefix := requiredEnvironment(t, "HUBSPOT_ACCEPTANCE_PREFIX")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	properties, err := clients.Properties.List(ctx, "contacts", false, "non_sensitive", "")
	if err != nil {
		t.Fatalf("list property definitions for manual cleanup: %s", acceptance.SanitizedHubSpotError(err))
	}
	for _, property := range properties {
		if strings.HasPrefix(property.Name, prefix) {
			if err := clients.Properties.Archive(ctx, "contacts", property.Name); err != nil {
				t.Fatalf("archive owned property definition during manual cleanup: %s", acceptance.SanitizedHubSpotError(err))
			}
		}
	}

	groups, err := clients.PropertyGroups.List(ctx, "contacts")
	if err != nil {
		t.Fatalf("list property groups for manual cleanup: %s", acceptance.SanitizedHubSpotError(err))
	}
	for _, group := range groups {
		if strings.HasPrefix(group.Name, prefix) {
			if err := clients.PropertyGroups.Archive(ctx, "contacts", group.Name); err != nil {
				t.Fatalf("archive owned property group during manual cleanup: %s", acceptance.SanitizedHubSpotError(err))
			}
		}
	}

	propertyCount, groupCount := countFreeOwnedConfiguration(t, ctx, clients, prefix)
	if propertyCount != 0 || groupCount != 0 {
		t.Fatal("manual cleanup could not verify absence of all prefixed CRM configuration")
	}
}

func freeJanitorClients(t *testing.T) *hubspot.ClientSet {
	t.Helper()
	if requiredEnvironment(t, "CAPABILITY_SHARD") != "free_properties" {
		t.Fatal("janitor implementation is unavailable for the selected capability shard")
	}
	token := requiredEnvironment(t, "HUBSPOT_ACCESS_TOKEN")
	origin, err := url.Parse("https://api.hubapi.com")
	if err != nil {
		t.Fatal("parse HubSpot API origin")
	}
	clients, err := hubspot.NewClientSet(hubspot.TransportConfig{
		BaseURL:     origin,
		AccessToken: token,
		UserAgent:   "terraform-provider-hubspot/acceptance-janitor",
	})
	if err != nil {
		t.Fatal("configure HubSpot acceptance janitor")
	}
	return clients
}

func countFreeOwnedConfiguration(t *testing.T, ctx context.Context, clients *hubspot.ClientSet, prefix string) (int, int) {
	t.Helper()
	properties, err := clients.Properties.List(ctx, "contacts", false, "non_sensitive", "")
	if err != nil {
		t.Fatalf("list property definitions for janitor verification: %s", acceptance.SanitizedHubSpotError(err))
	}
	groups, err := clients.PropertyGroups.List(ctx, "contacts")
	if err != nil {
		t.Fatalf("list property groups for janitor verification: %s", acceptance.SanitizedHubSpotError(err))
	}
	propertyCount := 0
	for _, property := range properties {
		if strings.HasPrefix(property.Name, prefix) {
			propertyCount++
		}
	}
	groupCount := 0
	for _, group := range groups {
		if strings.HasPrefix(group.Name, prefix) {
			groupCount++
		}
	}
	return propertyCount, groupCount
}

func requireFreeOwnedConfigurationAbsent(t *testing.T, prefix string) {
	t.Helper()
	clients := freeJanitorClients(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	properties, groups := countFreeOwnedConfiguration(t, ctx, clients, prefix)
	if properties != 0 || groups != 0 {
		t.Fatal("independent cleanup verification found active owned CRM configuration")
	}
}
