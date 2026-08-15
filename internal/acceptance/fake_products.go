// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

//go:build acceptance

package acceptance

import (
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/jackemcpherson/terraform-provider-hubspot/internal/hubspot"
)

type fakeProduct struct {
	product     hubspot.Product
	patchCount  int
	deleteCount int
}

type ProductFault string

const (
	ProductFaultCreateKnown       ProductFault = "create_known"
	ProductFaultCreateUnknown     ProductFault = "create_unknown"
	ProductFaultReadRejected      ProductFault = "read_rejected"
	ProductFaultArchiveRejected   ProductFault = "archive_rejected"
	ProductFaultArchiveAmbiguous  ProductFault = "archive_ambiguous"
	ProductFaultArchiveDisappears ProductFault = "archive_disappears"
)

func (f *FakeHubSpot) handleProducts(response http.ResponseWriter, request *http.Request, rest []string) {
	switch len(rest) {
	case 0:
		f.handleProductCollection(response, request)
	case 1:
		f.handleProductItem(response, request, rest[0])
	default:
		writeFakeError(response, http.StatusNotFound, "OBJECT_NOT_FOUND", "", "No route matched this request.")
	}
}

func (f *FakeHubSpot) handleProductCollection(response http.ResponseWriter, request *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if request.Method == http.MethodGet {
		archived := request.URL.Query().Get("archived") == "true"
		ids := make([]string, 0, len(f.products))
		for id, entry := range f.products {
			if entry.product.Archived == archived {
				ids = append(ids, id)
			}
		}
		sort.Strings(ids)
		results := make([]map[string]any, 0, len(ids))
		for _, id := range ids {
			results = append(results, fakeProductDocument(f.products[id].product))
		}
		writeFakeJSON(response, http.StatusOK, map[string]any{"results": results})
		return
	}
	if request.Method != http.MethodPost {
		writeFakeError(response, http.StatusMethodNotAllowed, "VALIDATION_ERROR", "", "Unsupported method.")
		return
	}
	var body struct {
		Properties map[string]string `json:"properties"`
	}
	if !decodeFakeBody(response, request, &body) {
		return
	}
	if _, hasFolder := body.Properties["hs_folder"]; hasFolder || !supportedFakeProductProperties(body.Properties, true) {
		writeFakeError(response, http.StatusBadRequest, "VALIDATION_ERROR", "", "The Product did not match the supported standard definition.")
		return
	}
	if f.activeProductUsesSKU(body.Properties["hs_sku"], "") {
		writeFakeError(response, http.StatusConflict, "CONFLICT", "", "An active Product already uses this SKU.")
		return
	}
	f.nextProductID++
	id := strconv.Itoa(f.nextProductID)
	product := hubspot.Product{
		ID: id, Name: body.Properties["name"], SKU: body.Properties["hs_sku"],
		Description: body.Properties["description"], Price: normalizeFakeProductDecimal(body.Properties["price"]),
		Cost:                   normalizeFakeProductDecimal(body.Properties["hs_cost_of_goods_sold"]),
		RecurringBillingPeriod: body.Properties["hs_recurring_billing_period"],
	}
	f.products[id] = &fakeProduct{product: product}
	switch f.nextProductFault {
	case ProductFaultCreateKnown:
		f.nextProductFault = ""
		writeFakeJSON(response, http.StatusCreated, map[string]any{"id": id, "archived": "invalid"})
		return
	case ProductFaultCreateUnknown:
		f.nextProductFault = ""
		writeFakeJSON(response, http.StatusCreated, map[string]any{"futureServiceMetadata": true})
		return
	}
	writeFakeJSON(response, http.StatusCreated, fakeProductDocument(product))
}

func (f *FakeHubSpot) handleProductItem(response http.ResponseWriter, request *http.Request, id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	entry := f.products[id]
	switch request.Method {
	case http.MethodGet:
		if f.nextProductFault == ProductFaultReadRejected {
			f.nextProductFault = ""
			writeFakeError(response, http.StatusForbidden, "MISSING_SCOPES", "", "The Product read was rejected.")
			return
		}
		archived := request.URL.Query().Get("archived") == "true"
		if entry == nil || entry.product.Archived != archived {
			writeFakeError(response, http.StatusNotFound, "OBJECT_NOT_FOUND", "", "No Product matched this identity.")
			return
		}
		writeFakeJSON(response, http.StatusOK, fakeProductDocument(entry.product))
	case http.MethodPatch:
		if entry == nil || entry.product.Archived {
			writeFakeError(response, http.StatusNotFound, "OBJECT_NOT_FOUND", "", "No active Product matched this identity.")
			return
		}
		var body struct {
			Properties map[string]string `json:"properties"`
		}
		if !decodeFakeBody(response, request, &body) {
			return
		}
		if len(body.Properties) == 0 || !supportedFakeProductProperties(body.Properties, false) {
			writeFakeError(response, http.StatusBadRequest, "VALIDATION_ERROR", "", "The Product patch was not supported.")
			return
		}
		if f.rejectNextProductPatch {
			f.rejectNextProductPatch = false
			writeFakeError(response, http.StatusBadRequest, "VALIDATION_ERROR", "", "The Product patch was rejected.")
			return
		}
		if sku, ok := body.Properties["hs_sku"]; ok && f.activeProductUsesSKU(sku, id) {
			writeFakeError(response, http.StatusConflict, "CONFLICT", "", "An active Product already uses this SKU.")
			return
		}
		applyFakeProductPatch(&entry.product, body.Properties)
		entry.patchCount++
		writeFakeJSON(response, http.StatusOK, fakeProductDocument(entry.product))
	case http.MethodDelete:
		if entry == nil || entry.product.Archived {
			writeFakeError(response, http.StatusNotFound, "OBJECT_NOT_FOUND", "", "No active Product matched this identity.")
			return
		}
		if f.nextProductFault == ProductFaultArchiveRejected {
			f.nextProductFault = ""
			writeFakeError(response, http.StatusForbidden, "MISSING_SCOPES", "", "The Product archive was rejected.")
			return
		}
		if f.nextProductFault == ProductFaultArchiveDisappears {
			f.nextProductFault = ""
			delete(f.products, id)
			response.WriteHeader(http.StatusNoContent)
			return
		}
		entry.product.Archived = true
		entry.deleteCount++
		if f.nextProductFault == ProductFaultArchiveAmbiguous {
			f.nextProductFault = ""
			writeFakeError(response, http.StatusForbidden, "MISSING_SCOPES", "", "The Product archive response was ambiguous.")
			return
		}
		response.WriteHeader(http.StatusNoContent)
	default:
		writeFakeError(response, http.StatusMethodNotAllowed, "VALIDATION_ERROR", "", "Unsupported method.")
	}
}

func supportedFakeProductProperties(properties map[string]string, requireAll bool) bool {
	allowed := map[string]bool{
		"name": true, "hs_sku": true, "description": true, "price": true,
		"hs_cost_of_goods_sold": true, "hs_recurring_billing_period": true,
	}
	for name := range properties {
		if !allowed[name] {
			return false
		}
	}
	if !requireAll {
		return true
	}
	for _, name := range []string{"name", "hs_sku", "description", "price"} {
		if properties[name] == "" {
			return false
		}
	}
	return true
}

func (f *FakeHubSpot) activeProductUsesSKU(sku, exceptID string) bool {
	for id, entry := range f.products {
		if id != exceptID && !entry.product.Archived && entry.product.SKU == sku {
			return true
		}
	}
	return false
}

func applyFakeProductPatch(product *hubspot.Product, properties map[string]string) {
	if value, ok := properties["name"]; ok {
		product.Name = value
	}
	if value, ok := properties["hs_sku"]; ok {
		product.SKU = value
	}
	if value, ok := properties["description"]; ok {
		product.Description = value
	}
	if value, ok := properties["price"]; ok {
		product.Price = normalizeFakeProductDecimal(value)
	}
	if value, ok := properties["hs_cost_of_goods_sold"]; ok {
		product.Cost = normalizeFakeProductDecimal(value)
	}
	if value, ok := properties["hs_recurring_billing_period"]; ok {
		product.RecurringBillingPeriod = value
	}
}

func normalizeFakeProductDecimal(value string) string {
	if !strings.Contains(value, ".") {
		return value
	}
	value = strings.TrimRight(value, "0")
	value = strings.TrimRight(value, ".")
	if value == "" {
		return "0"
	}
	return value
}

func fakeProductDocument(product hubspot.Product) map[string]any {
	return map[string]any{
		"id": product.ID, "archived": product.Archived,
		"properties": map[string]string{
			"name": product.Name, "hs_sku": product.SKU,
			"description": product.Description, "price": product.Price,
			"hs_cost_of_goods_sold":       product.Cost,
			"hs_folder":                   product.Folder,
			"hs_recurring_billing_period": product.RecurringBillingPeriod,
		},
		"futureServiceMetadata": map[string]any{"revision": 1},
	}
}

func (f *FakeHubSpot) ProductPatchCount(id string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	if entry := f.products[id]; entry != nil {
		return entry.patchCount
	}
	return 0
}

func (f *FakeHubSpot) ProductDeleteCount(id string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	if entry := f.products[id]; entry != nil {
		return entry.deleteCount
	}
	return 0
}

func (f *FakeHubSpot) ProductSnapshot(id string) (hubspot.Product, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	entry := f.products[id]
	if entry == nil {
		return hubspot.Product{}, false
	}
	return entry.product, true
}

func (f *FakeHubSpot) ActiveProductIDs() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	ids := make([]string, 0, len(f.products))
	for id, entry := range f.products {
		if !entry.product.Archived {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}

func (f *FakeHubSpot) FailNextProductOperation(fault ProductFault) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextProductFault = fault
}

func (f *FakeHubSpot) DriftProduct(id string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	entry := f.products[id]
	if entry == nil || entry.product.Archived {
		return false
	}
	entry.product.Description = "Out-of-band Product description"
	entry.product.Price = "1300"
	entry.product.Cost = "350"
	entry.product.RecurringBillingPeriod = "P6M"
	return true
}

func (f *FakeHubSpot) DriftProductOptionals(id string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	entry := f.products[id]
	if entry == nil || entry.product.Archived {
		return false
	}
	entry.product.Cost = "999"
	entry.product.RecurringBillingPeriod = "P3M"
	return true
}

func (f *FakeHubSpot) RejectNextProductPatch() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rejectNextProductPatch = true
}

func (f *FakeHubSpot) ArchiveProduct(id string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	entry := f.products[id]
	if entry == nil || entry.product.Archived {
		return false
	}
	entry.product.Archived = true
	return true
}

func (f *FakeHubSpot) RemoveProduct(id string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.products[id] == nil {
		return false
	}
	delete(f.products, id)
	return true
}
