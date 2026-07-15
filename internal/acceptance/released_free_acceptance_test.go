// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

//go:build acceptance

package acceptance_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackemcpherson/terraform-provider-hubspot/internal/acceptance"
	"github.com/jackemcpherson/terraform-provider-hubspot/internal/hubspot"
)

func TestReleasedFreePropertiesDrift(t *testing.T) {
	clients := freeJanitorClients(t)
	prefix := requiredEnvironment(t, "HUBSPOT_ACCEPTANCE_PREFIX")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	name := prefix + "released_scalar"
	current, err := clients.Properties.Get(ctx, "contacts", name, false, "non_sensitive", "")
	if err != nil {
		t.Fatalf("read released property for drift probe: %s", acceptance.SanitizedHubSpotError(err))
	}
	_, err = clients.Properties.Update(ctx, "contacts", name, definitionWrite(current, "Out-of-band released property label"))
	if err != nil {
		t.Fatalf("mutate released property for drift probe: %s", acceptance.SanitizedHubSpotError(err))
	}
	verified, err := clients.Properties.Get(ctx, "contacts", name, false, "non_sensitive", "")
	if err != nil {
		t.Fatalf("verify released property drift probe: %s", acceptance.SanitizedHubSpotError(err))
	}
	if verified.Label != "Out-of-band released property label" {
		t.Fatal("released property drift probe did not reach the requested safe configuration")
	}
}

func TestReleasedFreePropertiesAbsence(t *testing.T) {
	clients := freeJanitorClients(t)
	prefix := requiredEnvironment(t, "HUBSPOT_ACCEPTANCE_PREFIX")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	properties, groups := countFreeOwnedConfiguration(t, ctx, clients, prefix)
	if properties != 0 || groups != 0 {
		t.Fatal("released-provider destroy did not verify remote absence")
	}
}

func definitionWrite(definition hubspot.PropertyDefinition, label string) hubspot.PropertyWrite {
	return hubspot.PropertyWrite{
		Name: definition.Name, Label: label, GroupName: definition.GroupName,
		Type: definition.Type, FieldType: definition.FieldType,
		Description: definition.Description, DisplayOrder: definition.DisplayOrder,
		FormField: definition.FormField, Hidden: definition.Hidden,
		HasUniqueValue: definition.HasUniqueValue, DataSensitivity: definition.DataSensitivity,
		ExternalOptions: definition.ExternalOptions, ShowCurrencySymbol: definition.ShowCurrencySymbol,
		CalculationFormula: definition.CalculationFormula, CurrencyPropertyName: definition.CurrencyPropertyName,
		NumberDisplayHint: definition.NumberDisplayHint, TextDisplayHint: definition.TextDisplayHint,
		ReferencedObjectType: definition.ReferencedObjectType, Options: definition.Options,
	}
}
