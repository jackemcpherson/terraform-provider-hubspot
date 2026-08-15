// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

//go:build acceptance

package acceptance_test

import (
	"context"
	"errors"
	"math/big"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jackemcpherson/terraform-provider-hubspot/internal/acceptance"
	"github.com/jackemcpherson/terraform-provider-hubspot/internal/hubspot"
)

func TestAcc_product_definitions_ContractPreflight(t *testing.T) {
	requireAcceptanceEnabled(t)
	token := requiredEnvironment(t, "HUBSPOT_ACCESS_TOKEN")
	prefix := requiredEnvironment(t, "HUBSPOT_ACCEPTANCE_PREFIX")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	clients, err := acceptance.NewRealPortalClientSet(ctx, token, "terraform-provider-hubspot/products-contract-preflight")
	if err != nil {
		t.Fatalf("Products capability preflight failed: %s", acceptance.SanitizedHubSpotError(err))
	}

	properties, err := clients.Products.PropertySchema(ctx)
	if err != nil {
		t.Fatalf("Product runtime property schema failed: %s", acceptance.SanitizedHubSpotError(err))
	}
	createdIDs := make([]string, 0, 2)
	defer func() {
		for _, id := range createdIDs {
			if err := clients.Products.Archive(context.Background(), id); err != nil && !productPreflightNotFound(err) {
				t.Errorf("Product contract probe cleanup failed: %s", acceptance.SanitizedHubSpotError(err))
			}
		}
	}()
	if err := acceptance.ValidateProductPropertySchema(properties); err != nil {
		t.Fatal(err)
	}

	suffix := strings.ReplaceAll(time.Now().UTC().Format("20060102T150405.000000000"), ".", "")
	sku := prefix + "contract_probe_" + suffix
	cost := "300.00"
	recurrence := "P12M"
	created, createErr := clients.Products.Create(ctx, hubspot.ProductWrite{
		Name: "Provider contract probe", SKU: sku,
		Description: "Disposable Product contract probe", Price: "1200.00",
		Cost: &cost, RecurringBillingPeriod: &recurrence,
	})
	if created.ID != "" {
		createdIDs = append(createdIDs, created.ID)
	}
	if createErr != nil || created.ID == "" {
		t.Fatalf("Product root create probe failed: %s", acceptance.SanitizedHubSpotError(createErr))
	}
	read, err := clients.Products.Get(ctx, created.ID)
	if err != nil || read.ID != created.ID || read.Archived || read.Folder != "" ||
		!productPreflightDecimalsEqual(read.Price, "1200.00") ||
		!productPreflightDecimalsEqual(read.Cost, "300.00") || read.RecurringBillingPeriod != recurrence {
		t.Fatal("Product contract probe did not round-trip the exact generated identity and supported values")
	}
	duplicate, duplicateErr := clients.Products.Create(ctx, hubspot.ProductWrite{
		Name: "Duplicate Product contract probe", SKU: sku,
		Description: "Duplicate SKU probe", Price: "1",
	})
	if duplicate.ID != "" {
		createdIDs = append(createdIDs, duplicate.ID)
	}
	if duplicateErr == nil || duplicate.ID != "" {
		t.Fatal("Product contract probe did not reject a duplicate active SKU")
	}
	var apiError *hubspot.Error
	if !errors.As(duplicateErr, &apiError) || (apiError.Status != http.StatusBadRequest && apiError.Status != http.StatusConflict) {
		t.Fatalf("duplicate SKU returned an unexpected safe status: %s", acceptance.SanitizedHubSpotError(duplicateErr))
	}

	if _, err := clients.Products.Patch(ctx, created.ID, map[string]string{
		"hs_cost_of_goods_sold": "", "hs_recurring_billing_period": "",
	}); err != nil {
		t.Fatalf("Product optional clear probe failed: %s", acceptance.SanitizedHubSpotError(err))
	}
	cleared, err := clients.Products.Get(ctx, created.ID)
	if err != nil || cleared.Cost != "" || cleared.RecurringBillingPeriod != "" {
		t.Fatal("Product optional clear probe did not reach empty remote values")
	}
	if err := clients.Products.Archive(ctx, created.ID); err != nil {
		t.Fatalf("Product archive probe failed: %s", acceptance.SanitizedHubSpotError(err))
	}
	if _, err := clients.Products.Get(ctx, created.ID); !productPreflightNotFound(err) {
		t.Fatal("Product archive probe did not verify active absence")
	}
	tombstone, err := clients.Products.GetArchived(ctx, created.ID)
	if err != nil || tombstone.ID != created.ID || !tombstone.Archived {
		t.Fatal("Product archive probe did not verify the same archived identity")
	}
	if err := clients.Products.Archive(ctx, created.ID); !acceptance.ProductArchiveReplayAccepted(err) {
		t.Fatal("Product duplicate archive probe returned an unexpected result")
	}
}

func productPreflightDecimalsEqual(first, second string) bool {
	firstValue, firstOK := new(big.Rat).SetString(first)
	secondValue, secondOK := new(big.Rat).SetString(second)
	return firstOK && secondOK && firstValue.Cmp(secondValue) == 0
}

func productPreflightNotFound(err error) bool {
	var apiError *hubspot.Error
	return errors.As(err, &apiError) && apiError.Status == http.StatusNotFound
}
