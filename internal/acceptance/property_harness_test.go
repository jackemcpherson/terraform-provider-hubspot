// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

package acceptance_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"sync"
	"testing"

	"github.com/jackemcpherson/terraform-provider-hubspot/internal/acceptance"
)

func TestRunExecutesPropertyLifecycleThroughOpenTofu(t *testing.T) {
	if _, err := exec.LookPath("tofu"); err != nil {
		t.Skip("pinned OpenTofu executable is not installed")
	}

	api := newPropertyAPI(t)
	server := httptest.NewServer(api)
	t.Cleanup(server.Close)
	t.Setenv("HUBSPOT_ACCESS_TOKEN", "acceptance-sentinel")
	ledger := t.TempDir() + "/cleanup.jsonl"

	acceptance.Run(t, acceptance.Options{
		Engine:       acceptance.OpenTofu,
		Shard:        acceptance.FreeProperties,
		Prefix:       "tf_acc_harness_",
		LedgerPath:   ledger,
		ProbeBaseURL: server.URL,
	}, func(session *acceptance.Session) {
		initial := propertyConfig(server.URL, "Initial property", "")
		advanced := strings.Replace(initial, `description = ""`, `description         = ""
  calculation_formula = "1 + 1"`, 1)
		session.RequirePlanFailure(advanced, "Free alpha property surface")
		emptyAdvanced := strings.Replace(initial, `description = ""`, `description         = ""
  calculation_formula = ""`, 1)
		session.RequirePlanFailure(emptyAdvanced, "Free alpha property surface")
		unknownAdvanced := strings.Replace(initial, `provider "hubspot" {`, `resource "terraform_data" "formula" {
  input = "1 + 1"
}

provider "hubspot" {`, 1)
		unknownAdvanced = strings.Replace(unknownAdvanced, `description = ""`, `description         = ""
  calculation_formula = terraform_data.formula.output`, 1)
		session.RequirePlanFailure(unknownAdvanced, "Free alpha property surface")
		invalidDateHint := strings.Replace(initial, `description = ""`, `date_display_hint = "absolute"`, 1)
		session.RequireValidationFailure(invalidDateHint, "Unsupported argument")
		session.Apply(initial)
		session.RequireStateString("hubspot_property.test", "label", "Initial property")
		session.RequireEmptyPlan(initial)

		updated := propertyConfig(server.URL, "Updated property", "Updated description")
		session.RequirePlanWithoutWarning(updated, acceptance.PropertyTypeTransition)
		session.Apply(updated)
		session.RequireEmptyPlan(updated)
		session.MutatePropertyLabel("contacts", "tf_acc_harness_property", "Out-of-band property label")
		session.RequirePlanDiffAttributes(updated, "hubspot_property.test", "label")
		session.Apply(updated)
		session.RequireEmptyPlan(updated)
		session.RemoveState("hubspot_property.test")
		session.Import("hubspot_property.test", "contacts/tf_acc_harness_property")
		session.RequireEmptyPlan(updated)
		transition := strings.Replace(updated, `field_type  = "text"`, `field_type  = "textarea"`, 1)
		session.RequirePlanWarning(transition, acceptance.PropertyTypeTransition)
		managedOnly := strings.Split(updated, `
data "hubspot_property_definition"`)[0]
		session.Apply(managedOnly)
		session.ArchiveProperty("contacts", "tf_acc_harness_property")
		session.Refresh(managedOnly)
		session.RequireStateAbsent("hubspot_property.test")
	})

	if api.isActive() {
		t.Fatal("acceptance cleanup left the property active")
	}
}

func TestRunWarnsBeforeEnumerationOptionValuesChange(t *testing.T) {
	if _, err := exec.LookPath("tofu"); err != nil {
		t.Skip("pinned OpenTofu executable is not installed")
	}

	api := newPropertyAPI(t)
	server := httptest.NewServer(api)
	t.Cleanup(server.Close)
	ledger := t.TempDir() + "/cleanup.jsonl"

	acceptance.Run(t, acceptance.Options{
		Engine:     acceptance.OpenTofu,
		Shard:      acceptance.FreeProperties,
		Prefix:     "tf_acc_harness_",
		LedgerPath: ledger,
	}, func(session *acceptance.Session) {
		initial := enumerationConfig(server.URL, "Alpha", true)
		session.Apply(initial)
		session.RequireEmptyPlan(initial)

		safe := enumerationConfig(server.URL, "Alpha updated", true)
		session.RequirePlanWithoutWarning(safe, acceptance.PropertyOptionValuesChanged)
		session.Apply(safe)
		session.RequireEmptyPlan(safe)

		destructive := enumerationConfig(server.URL, "Alpha updated", false)
		session.RequirePlanWarning(destructive, acceptance.PropertyOptionValuesChanged)
	})
}

func propertyConfig(apiBaseURL, label, description string) string {
	return fmt.Sprintf(`
terraform {
  required_providers {
    hubspot = {
      source = "registry.opentofu.org/jackemcpherson/hubspot"
    }
  }
}

provider "hubspot" {
  access_token = "acceptance-sentinel"
  api_base_url = %q
}

resource "hubspot_property" "test" {
  object_type = "contacts"
  name        = "tf_acc_harness_property"
  label       = %q
  group_name  = "contactinformation"
  type        = "string"
  field_type  = "text"
  description = %q
  display_order = 10
}

data "hubspot_property_definition" "test" {
  object_type = "contacts"
  name        = hubspot_property.test.name
}

data "hubspot_property_definitions" "test" {
  object_type = "contacts"
  depends_on  = [hubspot_property.test]
}
`, apiBaseURL, label, description)
}

func enumerationConfig(apiBaseURL, alphaLabel string, includeBeta bool) string {
	beta := ""
	if includeBeta {
		beta = `beta = { label = "Beta", display_order = 21 }`
	}
	return fmt.Sprintf(`
terraform {
  required_providers {
    hubspot = {
      source = "registry.opentofu.org/jackemcpherson/hubspot"
    }
  }
}

provider "hubspot" {
  access_token = "acceptance-sentinel"
  api_base_url = %q
}

resource "hubspot_property" "test" {
  object_type = "contacts"
  name        = "tf_acc_harness_property"
  label       = "Acceptance enumeration"
  group_name  = "contactinformation"
  type        = "enumeration"
  field_type  = "select"

  options = {
    alpha = { label = %q, display_order = 20 }
    %s
  }
}
`, apiBaseURL, alphaLabel, beta)
}

type propertyAPI struct {
	t           *testing.T
	mu          sync.Mutex
	active      bool
	archived    bool
	label       string
	description string
	kind        string
	fieldType   string
	options     []map[string]any
}

type propertyRequest struct {
	Label       string           `json:"label"`
	Description string           `json:"description"`
	Type        string           `json:"type"`
	FieldType   string           `json:"fieldType"`
	Options     []map[string]any `json:"options"`
}

func newPropertyAPI(t *testing.T) *propertyAPI {
	t.Helper()
	return &propertyAPI{t: t}
}

func (a *propertyAPI) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()

	collection := "/crm/properties/2026-03/contacts"
	item := collection + "/tf_acc_harness_property"
	response.Header().Set("Content-Type", "application/json")

	switch {
	case request.Method == http.MethodPost && request.URL.Path == collection:
		var body propertyRequest
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			a.t.Errorf("decode property create request: %v", err)
			response.WriteHeader(http.StatusBadRequest)
			return
		}
		a.applyRequest(body)
		a.active = true
		a.archived = false
		response.WriteHeader(http.StatusCreated)
		a.writeProperty(response, false)
	case request.Method == http.MethodGet && request.URL.Path == collection:
		results := []any{}
		if a.active {
			results = append(results, a.propertyDocument(false))
		}
		if err := json.NewEncoder(response).Encode(map[string]any{"results": results}); err != nil {
			a.t.Errorf("encode property collection response: %v", err)
		}
	case request.Method == http.MethodGet && request.URL.Path == item:
		wantArchived := request.URL.Query().Get("archived") == "true"
		if wantArchived && a.archived {
			a.writeProperty(response, true)
			return
		}
		if !wantArchived && a.active {
			a.writeProperty(response, false)
			return
		}
		response.WriteHeader(http.StatusNotFound)
	case request.Method == http.MethodPatch && request.URL.Path == item:
		var body propertyRequest
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			a.t.Errorf("decode property update request: %v", err)
			response.WriteHeader(http.StatusBadRequest)
			return
		}
		a.applyRequest(body)
		a.writeProperty(response, false)
	case request.Method == http.MethodDelete && request.URL.Path == item:
		a.active = false
		a.archived = true
		response.WriteHeader(http.StatusNoContent)
	default:
		a.t.Errorf("unexpected property request: %s %s", request.Method, request.URL.RequestURI())
		response.WriteHeader(http.StatusNotFound)
	}
}

func (a *propertyAPI) applyRequest(body propertyRequest) {
	a.label = body.Label
	a.description = body.Description
	a.kind = body.Type
	a.fieldType = body.FieldType
	a.options = make([]map[string]any, 0, len(body.Options))
	for index, option := range body.Options {
		copy := make(map[string]any, len(option))
		for key, value := range option {
			copy[key] = value
		}
		copy["displayOrder"] = 20 + index
		a.options = append(a.options, copy)
	}
}

func (a *propertyAPI) writeProperty(response http.ResponseWriter, archived bool) {
	if err := json.NewEncoder(response).Encode(a.propertyDocument(archived)); err != nil {
		a.t.Errorf("encode property response: %v", err)
	}
}

func (a *propertyAPI) propertyDocument(archived bool) map[string]any {
	return map[string]any{
		"name":               "tf_acc_harness_property",
		"label":              a.label,
		"groupName":          "contactinformation",
		"type":               a.kind,
		"fieldType":          a.fieldType,
		"description":        a.description,
		"displayOrder":       10,
		"formField":          false,
		"hidden":             false,
		"hasUniqueValue":     false,
		"dataSensitivity":    "non_sensitive",
		"externalOptions":    false,
		"showCurrencySymbol": false,
		"options":            a.options,
		"hubSpotDefined":     false,
		"archived":           archived,
		"modificationMetadata": map[string]any{
			"readOnlyDefinition": false,
		},
	}
}

func (a *propertyAPI) isActive() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.active
}
