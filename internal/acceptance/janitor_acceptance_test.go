// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

//go:build acceptance

package acceptance_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackemcpherson/terraform-provider-hubspot/internal/acceptance"
	"github.com/jackemcpherson/terraform-provider-hubspot/internal/hubspot"
)

func TestAcc_JanitorReport(t *testing.T) {
	clients, shard := janitorClients(t)
	prefix := requiredEnvironment(t, "HUBSPOT_ACCEPTANCE_PREFIX")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	switch shard {
	case "free_properties":
		propertyCount, groupCount := countFreeOwnedConfiguration(t, ctx, clients, prefix)
		t.Logf("stale owned CRM configuration: property_definitions=%d property_groups=%d", propertyCount, groupCount)
	case "deal_pipelines":
		t.Logf("stale owned CRM configuration: deal_pipelines=%d", countOwnedPipelines(t, ctx, clients, "deals", prefix))
	case "ticket_pipelines":
		t.Logf("stale owned CRM configuration: ticket_pipelines=%d", countOwnedPipelines(t, ctx, clients, "tickets", prefix))
	case "form_definitions":
		active, archived, err := countOwnedForms(ctx, clients, prefix)
		if err != nil {
			t.Fatalf("report stale owned Form definitions: %s", acceptance.SanitizedHubSpotError(err))
		}
		t.Logf("stale owned HubSpot configuration: active_form_definitions=%d retained_archived_form_definitions=%d", active, archived)
	}
}

func TestAcc_ManualPrefixCleanup(t *testing.T) {
	clients, shard := janitorClients(t)
	prefix := requiredEnvironment(t, "HUBSPOT_ACCEPTANCE_PREFIX")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if shard == "form_definitions" {
		archived, err := archiveOwnedForms(ctx, clients, prefix)
		if err != nil {
			t.Fatalf("archive owned Form definitions: %s", acceptance.SanitizedHubSpotError(err))
		}
		t.Logf("terminal cleanup retained Archived form definitions: %d", archived)
		return
	}

	if shard == "deal_pipelines" || shard == "ticket_pipelines" {
		objectType := "deals"
		if shard == "ticket_pipelines" {
			objectType = "tickets"
		}
		pipelines, err := clients.Pipelines.List(ctx, objectType)
		if err != nil {
			t.Fatalf("list pipelines for manual cleanup: %s", acceptance.SanitizedHubSpotError(err))
		}
		for _, pipeline := range pipelines {
			if !pipeline.Archived && strings.HasPrefix(pipeline.Label, prefix) {
				if err := clients.Pipelines.Archive(ctx, objectType, pipeline.ID); err != nil {
					t.Fatalf("archive owned pipeline during manual cleanup: %s", acceptance.SanitizedHubSpotError(err))
				}
			}
		}
		if countOwnedPipelines(t, ctx, clients, objectType, prefix) != 0 {
			t.Fatal("manual cleanup could not verify absence of all active prefixed pipelines")
		}
		return
	}

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

func janitorClients(t *testing.T) (*hubspot.ClientSet, string) {
	t.Helper()
	shard := requiredEnvironment(t, "CAPABILITY_SHARD")
	if shard != "free_properties" && shard != "form_definitions" && shard != "deal_pipelines" && shard != "ticket_pipelines" {
		t.Fatal("janitor implementation is unavailable for the selected capability shard")
	}
	token := requiredEnvironment(t, "HUBSPOT_ACCESS_TOKEN")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	clients, err := acceptance.NewRealPortalClientSet(ctx, token, "terraform-provider-hubspot/acceptance-janitor")
	if err != nil {
		t.Fatalf("configure HubSpot acceptance janitor: %v", err)
	}
	return clients, shard
}

func countOwnedForms(ctx context.Context, clients *hubspot.ClientSet, prefix string) (int, int, error) {
	active, err := clients.Forms.List(ctx, false)
	if err != nil {
		return 0, 0, err
	}
	archived, err := clients.Forms.List(ctx, true)
	if err != nil {
		return 0, 0, err
	}
	return countFormsWithPrefix(active, prefix), countFormsWithPrefix(archived, prefix), nil
}

func countFormsWithPrefix(forms []hubspot.FormDefinition, prefix string) int {
	count := 0
	for _, form := range forms {
		if strings.HasPrefix(form.Name, prefix) {
			count++
		}
	}
	return count
}

func archiveOwnedForms(ctx context.Context, clients *hubspot.ClientSet, prefix string) (int, error) {
	active, err := clients.Forms.List(ctx, false)
	if err != nil {
		return 0, err
	}
	for _, form := range active {
		if !strings.HasPrefix(form.Name, prefix) {
			continue
		}
		if form.FormType != "hubspot" || form.ID == "" {
			return 0, errors.New("prefix-owned Form definition has an unsupported identity or type")
		}
		archiveErr := clients.Forms.Archive(ctx, form.ID)
		if _, activeErr := clients.Forms.Get(ctx, form.ID); activeErr == nil {
			return 0, errors.New("prefix-owned Form definition remained active after archival")
		} else if !formJanitorNotFound(activeErr) {
			return 0, fmt.Errorf("verify active Form definition absence: %w", activeErr)
		}
		archived, archivedErr := clients.Forms.GetArchived(ctx, form.ID)
		if archivedErr != nil {
			if archiveErr != nil {
				return 0, fmt.Errorf("archive prefix-owned Form definition: %w", archiveErr)
			}
			return 0, fmt.Errorf("verify Archived form definition: %w", archivedErr)
		}
		if archived.ID != form.ID || !archived.Archived {
			return 0, errors.New("Archived form definition identity was not exact")
		}
	}
	remaining, retained, err := countOwnedForms(ctx, clients, prefix)
	if err != nil {
		return 0, err
	}
	if remaining != 0 {
		return 0, errors.New("manual cleanup could not verify zero active prefix-owned Form definitions")
	}
	return retained, nil
}

func formJanitorNotFound(err error) bool {
	var apiError *hubspot.Error
	return errors.As(err, &apiError) && apiError.Status == 404
}

func freeJanitorClients(t *testing.T) *hubspot.ClientSet {
	t.Helper()
	clients, shard := janitorClients(t)
	if shard != "free_properties" {
		t.Fatal("free janitor client used the wrong capability shard")
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
	clients, shard := janitorClients(t)
	if shard != "free_properties" {
		t.Fatal("free owned-configuration verification used the wrong capability shard")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	properties, groups := countFreeOwnedConfiguration(t, ctx, clients, prefix)
	if properties != 0 || groups != 0 {
		t.Fatal("independent cleanup verification found active owned CRM configuration")
	}
}

func requireFreeOwnedConfigurationAbsentForStandardObjectTypes(t *testing.T, prefix string) {
	t.Helper()
	clients := freeJanitorClients(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	for _, objectType := range []string{"contacts", "companies", "deals", "tickets"} {
		properties, err := clients.Properties.List(ctx, objectType, false, "non_sensitive", "")
		if err != nil {
			t.Fatalf("list %s property definitions for cleanup verification: %s", objectType, acceptance.SanitizedHubSpotError(err))
		}
		groups, err := clients.PropertyGroups.List(ctx, objectType)
		if err != nil {
			t.Fatalf("list %s property groups for cleanup verification: %s", objectType, acceptance.SanitizedHubSpotError(err))
		}
		for _, property := range properties {
			if strings.HasPrefix(property.Name, prefix) {
				t.Fatalf("cleanup left an active %s property definition: %s", objectType, property.Name)
			}
		}
		for _, group := range groups {
			if strings.HasPrefix(group.Name, prefix) {
				t.Fatalf("cleanup left an active %s property group: %s", objectType, group.Name)
			}
		}
	}
}

func countOwnedPipelines(t *testing.T, ctx context.Context, clients *hubspot.ClientSet, objectType, prefix string) int {
	t.Helper()
	pipelines, err := clients.Pipelines.List(ctx, objectType)
	if err != nil {
		t.Fatalf("list pipelines for janitor verification: %s", acceptance.SanitizedHubSpotError(err))
	}
	count := 0
	for _, pipeline := range pipelines {
		if !pipeline.Archived && strings.HasPrefix(pipeline.Label, prefix) {
			count++
		}
	}
	return count
}
