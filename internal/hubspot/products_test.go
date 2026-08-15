// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

package hubspot

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestProductClientUsesExactIdentityAndManagedProperties(t *testing.T) {
	requests := make([]string, 0, 5)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests = append(requests, request.Method+" "+request.URL.RequestURI())
		writer.Header().Set("Content-Type", "application/json")
		switch request.Method + " " + request.URL.Path {
		case "POST /crm/objects/2026-03/products":
			var body struct {
				Properties map[string]string `json:"properties"`
			}
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			want := map[string]string{
				"description": "Annual support",
				"hs_sku":      "SUPPORT-1",
				"name":        "Support",
				"price":       "1200.00",
			}
			if !reflect.DeepEqual(body.Properties, want) {
				t.Fatalf("create properties = %#v, want %#v", body.Properties, want)
			}
			if _, exists := body.Properties["hs_folder"]; exists {
				t.Fatal("create sent hs_folder")
			}
			io.WriteString(writer, `{"id":"701","properties":{"name":"Support","hs_sku":"SUPPORT-1","description":"Annual support","price":"1200"},"archived":false,"futureField":true}`)
		case "GET /crm/objects/2026-03/products/701":
			io.WriteString(writer, `{"id":"701","properties":{"name":"Support","hs_sku":"SUPPORT-1","description":"Annual support","price":"1200.0"},"archived":false,"futureField":true}`)
		case "PATCH /crm/objects/2026-03/products/701":
			var body struct {
				Properties map[string]string `json:"properties"`
			}
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(body.Properties, map[string]string{"description": "Priority support"}) {
				t.Fatalf("patch properties = %#v", body.Properties)
			}
			io.WriteString(writer, `{"id":"701","properties":{"description":"Priority support"},"archived":false}`)
		case "DELETE /crm/objects/2026-03/products/701":
			writer.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.RequestURI())
		}
	}))
	defer server.Close()

	client := &ProductClient{transport: newTestTransport(t, server.URL)}
	created, err := client.Create(context.Background(), ProductWrite{
		Name: "Support", SKU: "SUPPORT-1", Description: "Annual support", Price: "1200.00",
	})
	if err != nil || created.ID != "701" {
		t.Fatalf("create = %#v, %v", created, err)
	}
	read, err := client.Get(context.Background(), "701")
	if err != nil || read.Price != "1200.0" {
		t.Fatalf("read = %#v, %v", read, err)
	}
	patched, err := client.Patch(context.Background(), "701", map[string]string{"description": "Priority support"})
	if err != nil || patched.ID != "701" {
		t.Fatalf("patch = %#v, %v", patched, err)
	}
	if err := client.Archive(context.Background(), "701"); err != nil {
		t.Fatal(err)
	}

	wantRequests := []string{
		"POST /crm/objects/2026-03/products",
		"GET /crm/objects/2026-03/products/701?properties=description%2Chs_cost_of_goods_sold%2Chs_recurring_billing_period%2Chs_sku%2Cname%2Cprice",
		"PATCH /crm/objects/2026-03/products/701",
		"DELETE /crm/objects/2026-03/products/701",
	}
	if !reflect.DeepEqual(requests, wantRequests) {
		t.Fatalf("requests = %#v, want %#v", requests, wantRequests)
	}
}

func TestProductClientListsAllActiveAndReadsArchivedIdentity(t *testing.T) {
	requests := make([]string, 0, 3)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests = append(requests, request.Method+" "+request.URL.RequestURI())
		writer.Header().Set("Content-Type", "application/json")
		switch {
		case request.URL.Path == "/crm/objects/2026-03/products" && request.URL.Query().Get("after") == "":
			io.WriteString(writer, `{"results":[{"id":"701","properties":{"hs_sku":"A"},"archived":false}],"paging":{"next":{"after":"next-page"}}}`)
		case request.URL.Path == "/crm/objects/2026-03/products" && request.URL.Query().Get("after") == "next-page":
			io.WriteString(writer, `{"results":[{"id":"702","properties":{"hs_sku":"B"},"archived":false}]}`)
		case request.URL.Path == "/crm/objects/2026-03/products/701" && request.URL.Query().Get("archived") == "true":
			io.WriteString(writer, `{"id":"701","properties":{"hs_sku":"A"},"archived":true,"archivedAt":"2026-08-15T00:00:00Z"}`)
		default:
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.RequestURI())
		}
	}))
	defer server.Close()

	client := &ProductClient{transport: newTestTransport(t, server.URL)}
	products, err := client.List(context.Background())
	if err != nil || len(products) != 2 || products[0].ID != "701" || products[1].ID != "702" {
		t.Fatalf("products = %#v, %v", products, err)
	}
	archived, err := client.GetArchived(context.Background(), "701")
	if err != nil || archived.ID != "701" || !archived.Archived {
		t.Fatalf("archived = %#v, %v", archived, err)
	}
	if !strings.Contains(requests[0], "limit=100") || !strings.Contains(requests[1], "after=next-page") ||
		!strings.Contains(requests[2], "archived=true") {
		t.Fatalf("requests = %#v", requests)
	}
}

func TestProductClientClassifiesMalformedCreateAndAPIRejection(t *testing.T) {
	t.Run("missing create id is ambiguous", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			writer.Header().Set("Content-Type", "application/json")
			io.WriteString(writer, `{"properties":{"name":"Support"}}`)
		}))
		defer server.Close()
		client := &ProductClient{transport: newTestTransport(t, server.URL)}
		_, err := client.Create(context.Background(), ProductWrite{Name: "Support", SKU: "S", Description: "D", Price: "1"})
		var operationError *Error
		if !errors.As(err, &operationError) || !operationError.Ambiguous {
			t.Fatalf("create error = %v, want ambiguous", err)
		}
	})

	t.Run("API rejection retains safe detail", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			writer.Header().Set("Content-Type", "application/json")
			writer.WriteHeader(http.StatusConflict)
			io.WriteString(writer, `{"status":"error","category":"CONFLICT","message":"duplicate SKU"}`)
		}))
		defer server.Close()
		client := &ProductClient{transport: newTestTransport(t, server.URL)}
		_, err := client.Create(context.Background(), ProductWrite{Name: "Support", SKU: "S", Description: "D", Price: "1"})
		var operationError *Error
		if !errors.As(err, &operationError) || operationError.Status != http.StatusConflict || operationError.Ambiguous ||
			!strings.Contains(err.Error(), "duplicate SKU") {
			t.Fatalf("create error = %#v", err)
		}
	})
}

func TestProductClientReadsRuntimePropertySchema(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Path != "/crm/properties/2026-03/products" {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.RequestURI())
		}
		writer.Header().Set("Content-Type", "application/json")
		io.WriteString(writer, `{"results":[{"name":"name","type":"string"},{"name":"hs_sku","type":"string","hasUniqueValue":true},{"name":"price","type":"number"},{"name":"future_property","type":"enumeration"}]}`)
	}))
	defer server.Close()
	client := &ProductClient{transport: newTestTransport(t, server.URL)}
	properties, err := client.PropertySchema(context.Background())
	if err != nil || len(properties) != 4 || properties[1].Name != "hs_sku" || !properties[1].HasUniqueValue {
		t.Fatalf("Product property schema = %#v, %v", properties, err)
	}
}
