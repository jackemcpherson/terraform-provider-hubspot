// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

package hubspot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
)

const productPropertySelection = "description,hs_cost_of_goods_sold,hs_folder,hs_recurring_billing_period,hs_sku,name,price"

const productPageLimit = "100"

// ProductClient manages standard Product definitions by generated ID.
type ProductClient struct{ transport *Transport }

// Product is the supported Product definition projection.
type Product struct {
	ID                     string
	Name                   string
	SKU                    string
	Description            string
	Price                  string
	Cost                   string
	Folder                 string
	RecurringBillingPeriod string
	Archived               bool
}

// ProductWrite contains the standard Product properties supported on create.
// Nil optional values are omitted. Non-nil empty values explicitly clear them.
type ProductWrite struct {
	Name                   string
	SKU                    string
	Description            string
	Price                  string
	Cost                   *string
	RecurringBillingPeriod *string
}

// ProductProperty describes one runtime Product property contract entry.
type ProductProperty struct {
	Name           string `json:"name"`
	Type           string `json:"type"`
	HasUniqueValue bool   `json:"hasUniqueValue"`
}

type productWire struct {
	ID         string `json:"id"`
	Properties struct {
		Name                   string `json:"name"`
		SKU                    string `json:"hs_sku"`
		Description            string `json:"description"`
		Price                  string `json:"price"`
		Cost                   string `json:"hs_cost_of_goods_sold"`
		Folder                 string `json:"hs_folder"`
		RecurringBillingPeriod string `json:"hs_recurring_billing_period"`
	} `json:"properties"`
	Archived bool `json:"archived"`
}

type productPage struct {
	Results []productWire `json:"results"`
	Paging  struct {
		Next struct {
			After string `json:"after"`
		} `json:"next"`
	} `json:"paging"`
}

func productsPath() string { return "/crm/objects/2026-03/products" }

func productPropertiesPath() string { return "/crm/properties/2026-03/products" }

// PropertySchema reads the runtime Product property contract.
func (c *ProductClient) PropertySchema(ctx context.Context) ([]ProductProperty, error) {
	var page struct {
		Results []ProductProperty `json:"results"`
	}
	if err := c.transport.Do(ctx, Operation{
		Name: "product-property-schema-read", Method: http.MethodGet,
		Path: productPropertiesPath(), Replay: ReplaySafe,
	}, nil, &page); err != nil {
		return nil, err
	}
	return page.Results, nil
}

// Create creates one standard Product without selecting an account folder.
func (c *ProductClient) Create(ctx context.Context, input ProductWrite) (Product, error) {
	properties := map[string]string{
		"name": input.Name, "hs_sku": input.SKU,
		"description": input.Description, "price": input.Price,
	}
	if input.Cost != nil {
		properties["hs_cost_of_goods_sold"] = *input.Cost
	}
	if input.RecurringBillingPeriod != nil {
		properties["hs_recurring_billing_period"] = *input.RecurringBillingPeriod
	}
	body, err := json.Marshal(struct {
		Properties map[string]string `json:"properties"`
	}{Properties: properties})
	if err != nil {
		return Product{}, err
	}
	var wire productWire
	err = c.transport.Do(ctx, Operation{
		Name: "product-create", Method: http.MethodPost,
		Path: productsPath(), Replay: ReplayNever,
	}, bytes.NewReader(body), &wire)
	if err != nil {
		var operationError *Error
		if errors.As(err, &operationError) && operationError.Status >= 200 && operationError.Status < 300 {
			operationError.Ambiguous = true
		}
		return productFromWire(wire), err
	}
	product := productFromWire(wire)
	if product.ID == "" {
		return product, &Error{
			Operation: "product-create", Status: http.StatusOK,
			Cause: errors.New("HubSpot Product create response omitted id"), Ambiguous: true,
		}
	}
	return product, nil
}

// Get reads one exact active Product identity.
func (c *ProductClient) Get(ctx context.Context, id string) (Product, error) {
	return c.get(ctx, id, false)
}

// GetArchived reads the exact identity from HubSpot's archived view.
func (c *ProductClient) GetArchived(ctx context.Context, id string) (Product, error) {
	return c.get(ctx, id, true)
}

// List returns every active Product by following HubSpot's cursor.
func (c *ProductClient) List(ctx context.Context) ([]Product, error) {
	return c.list(ctx, false)
}

// ListArchived returns every archived Product by following HubSpot's cursor.
func (c *ProductClient) ListArchived(ctx context.Context) ([]Product, error) {
	return c.list(ctx, true)
}

func (c *ProductClient) list(ctx context.Context, archived bool) ([]Product, error) {
	products := make([]Product, 0)
	after := ""
	seen := make(map[string]struct{})
	for {
		query := url.Values{
			"limit":      []string{productPageLimit},
			"properties": []string{productPropertySelection},
		}
		operation := "product-list"
		if archived {
			query.Set("archived", "true")
			operation = "product-list-archived"
		}
		if after != "" {
			query.Set("after", after)
		}
		var page productPage
		if err := c.transport.Do(ctx, Operation{
			Name: operation, Method: http.MethodGet,
			Path: productsPath() + "?" + query.Encode(), Replay: ReplaySafe,
		}, nil, &page); err != nil {
			return nil, err
		}
		for _, wire := range page.Results {
			product := productFromWire(wire)
			if product.ID == "" {
				return nil, errors.New("HubSpot Product list response omitted id")
			}
			products = append(products, product)
		}
		next := page.Paging.Next.After
		if next == "" {
			return products, nil
		}
		if _, exists := seen[next]; exists {
			return nil, errors.New("HubSpot Product list cursor repeated")
		}
		seen[next] = struct{}{}
		after = next
	}
}

func (c *ProductClient) get(ctx context.Context, id string, archived bool) (Product, error) {
	if id == "" {
		return Product{}, errors.New("product id must not be empty")
	}
	query := url.Values{"properties": []string{productPropertySelection}}
	operation := "product-read"
	if archived {
		query.Set("archived", "true")
		operation = "product-read-archived"
	}
	var wire productWire
	if err := c.transport.Do(ctx, Operation{
		Name: operation, Method: http.MethodGet,
		Path: productsPath() + "/" + url.PathEscape(id) + "?" + query.Encode(), Replay: ReplaySafe,
	}, nil, &wire); err != nil {
		return Product{}, err
	}
	product := productFromWire(wire)
	if product.ID == "" {
		return Product{}, errors.New("HubSpot Product response omitted id")
	}
	if product.ID != id {
		return Product{}, errors.New("HubSpot Product response returned a different id")
	}
	return product, nil
}

// Patch updates only the supplied managed Product properties.
func (c *ProductClient) Patch(ctx context.Context, id string, properties map[string]string) (Product, error) {
	if id == "" {
		return Product{}, errors.New("product id must not be empty")
	}
	if len(properties) == 0 {
		return Product{}, errors.New("product patch requires at least one property")
	}
	body, err := json.Marshal(struct {
		Properties map[string]string `json:"properties"`
	}{Properties: properties})
	if err != nil {
		return Product{}, err
	}
	var wire productWire
	err = c.transport.Do(ctx, Operation{
		Name: "product-update", Method: http.MethodPatch,
		Path: productsPath() + "/" + url.PathEscape(id), Replay: ReplayExplicit,
	}, bytes.NewReader(body), &wire)
	if err != nil {
		var operationError *Error
		if errors.As(err, &operationError) && operationError.Status >= 200 && operationError.Status < 300 {
			operationError.Ambiguous = true
		}
		return productFromWire(wire), err
	}
	product := productFromWire(wire)
	if product.ID == "" {
		return product, &Error{Operation: "product-update", Status: http.StatusOK, Cause: errors.New("HubSpot Product update response omitted id"), Ambiguous: true}
	}
	if product.ID != id {
		return product, &Error{Operation: "product-update", Status: http.StatusOK, Cause: errors.New("HubSpot Product update returned a different id"), Ambiguous: true}
	}
	return product, nil
}

// Archive moves one exact Product identity to HubSpot's recycling bin.
func (c *ProductClient) Archive(ctx context.Context, id string) error {
	if id == "" {
		return errors.New("product id must not be empty")
	}
	return c.transport.Do(ctx, Operation{
		Name: "product-archive", Method: http.MethodDelete,
		Path: productsPath() + "/" + url.PathEscape(id), Replay: ReplayExplicit,
	}, nil, nil)
}

func productFromWire(wire productWire) Product {
	return Product{
		ID: wire.ID, Name: wire.Properties.Name, SKU: wire.Properties.SKU,
		Description: wire.Properties.Description, Price: wire.Properties.Price,
		Cost: wire.Properties.Cost, Folder: wire.Properties.Folder,
		RecurringBillingPeriod: wire.Properties.RecurringBillingPeriod,
		Archived:               wire.Archived,
	}
}
