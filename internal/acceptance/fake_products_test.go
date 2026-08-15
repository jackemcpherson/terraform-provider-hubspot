// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

//go:build acceptance

package acceptance

import (
	"context"
	"errors"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/jackemcpherson/terraform-provider-hubspot/internal/hubspot"
)

func TestFakeProductsEnforceSKUIdentityAndArchiveLifecycle(t *testing.T) {
	fake := NewFakeHubSpot("token", 123)
	server := httptest.NewServer(fake)
	defer server.Close()
	origin, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	clients, err := hubspot.NewClientSet(hubspot.TransportConfig{BaseURL: origin, AccessToken: "token"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	cost := "300.00"
	recurrence := "P12M"
	created, err := clients.Products.Create(ctx, hubspot.ProductWrite{
		Name: "Support", SKU: "tf_acc_product_1", Description: "Annual support",
		Price: "1200.00", Cost: &cost, RecurringBillingPeriod: &recurrence,
	})
	if err != nil || created.ID == "" {
		t.Fatalf("create = %#v, %v", created, err)
	}
	read, err := clients.Products.Get(ctx, created.ID)
	if err != nil || read.Price != "1200" || read.Cost != "300" {
		t.Fatalf("read = %#v, %v", read, err)
	}
	if _, err := clients.Products.Create(ctx, hubspot.ProductWrite{
		Name: "Duplicate", SKU: "tf_acc_product_1", Description: "Duplicate", Price: "1",
	}); err == nil {
		t.Fatal("fake accepted a duplicate active SKU")
	}
	if _, err := clients.Products.Patch(ctx, created.ID, map[string]string{"description": "Priority support"}); err != nil {
		t.Fatal(err)
	}
	if err := clients.Products.Archive(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := clients.Products.Get(ctx, created.ID); !fakeProductNotFound(err) {
		t.Fatalf("active read after archive = %v", err)
	}
	archived, err := clients.Products.GetArchived(ctx, created.ID)
	if err != nil || archived.ID != created.ID || !archived.Archived {
		t.Fatalf("archived = %#v, %v", archived, err)
	}
	if err := clients.Products.Archive(ctx, created.ID); !fakeProductNotFound(err) {
		t.Fatalf("duplicate archive error = %v", err)
	}
}

func fakeProductNotFound(err error) bool {
	var apiError *hubspot.Error
	return errors.As(err, &apiError) && apiError.Status == 404
}
