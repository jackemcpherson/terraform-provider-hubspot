// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

package hubspot

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPropertyGroupClientUsesTypedLifecycleRoutes(t *testing.T) {
	requests := make([]string, 0, 3)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		requests = append(requests, request.Method+" "+request.URL.Path+" "+string(body))
		switch request.Method {
		case http.MethodPost:
			writer.WriteHeader(http.StatusCreated)
			io.WriteString(writer, `{"name":"marketing","label":"Marketing","displayOrder":-1}`)
		case http.MethodPatch:
			io.WriteString(writer, `{"name":"marketing","label":"Updated","displayOrder":2}`)
		case http.MethodDelete:
			writer.WriteHeader(http.StatusNoContent)
		}
	}))
	defer server.Close()

	transport := newTestTransport(t, server.URL)
	client := &PropertyGroupClient{transport: transport}
	created, err := client.Create(context.Background(), "contacts", PropertyGroupCreate{Name: "marketing", Label: "Marketing", DisplayOrder: -1})
	if err != nil {
		t.Fatal(err)
	}
	if created.Name != "marketing" {
		t.Fatalf("created name = %q", created.Name)
	}
	updated, err := client.Update(context.Background(), "contacts", "marketing", PropertyGroupUpdate{Label: "Updated", DisplayOrder: 2})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Label != "Updated" || updated.DisplayOrder != 2 {
		t.Fatalf("updated = %#v", updated)
	}
	if err := client.Archive(context.Background(), "contacts", "marketing"); err != nil {
		t.Fatal(err)
	}

	if len(requests) != 3 {
		t.Fatalf("requests = %#v", requests)
	}
	var createPayload map[string]any
	if err := json.Unmarshal([]byte(strings.TrimPrefix(requests[0], "POST /crm/properties/2026-03/contacts/groups ")), &createPayload); err != nil {
		t.Fatal(err)
	}
	if createPayload["name"] != "marketing" || createPayload["label"] != "Marketing" || createPayload["displayOrder"] != float64(-1) {
		t.Fatalf("create payload = %#v", createPayload)
	}
}

func TestPropertyGroupClientRejectsPathLikeIdentity(t *testing.T) {
	client := &PropertyGroupClient{transport: nil}
	if _, err := client.Get(context.Background(), "contacts/groups", "marketing"); err == nil {
		t.Fatal("expected object type validation error")
	}
	if _, err := client.Get(context.Background(), "contacts", "marketing/groups"); err == nil {
		t.Fatal("expected group name validation error")
	}
}

func TestPropertyDefinitionClientSelectorsAndPagination(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		calls++
		if request.URL.Path != "/crm/properties/2026-03/contacts" || request.URL.Query().Get("archived") != "true" || request.URL.Query().Get("dataSensitivity") != "sensitive" || request.URL.Query().Get("locale") != "fr-fr" {
			t.Fatalf("unexpected request: %s", request.URL.String())
		}
		if request.URL.Query().Get("after") == "" {
			io.WriteString(writer, `{"results":[{"name":"first","label":"First","options":[{"value":"a","label":"A"}]}],"paging":{"next":{"after":"2"}}}`)
			return
		}
		io.WriteString(writer, `{"results":[{"name":"second","label":"Second"}]}`)
	}))
	defer server.Close()
	client := &PropertyDefinitionClient{transport: newTestTransport(t, server.URL)}
	definitions, err := client.List(context.Background(), "contacts", true, "sensitive", "fr-fr")
	if err != nil || len(definitions) != 2 || calls != 2 {
		t.Fatalf("definitions=%#v calls=%d err=%v", definitions, calls, err)
	}
	if len(definitions[0].Options) != 1 || definitions[0].Options[0].Value != "a" {
		t.Fatalf("options = %#v", definitions[0].Options)
	}
}
