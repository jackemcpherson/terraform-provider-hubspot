// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/jackemcpherson/terraform-provider-hubspot/internal/hubspot"
)

func TestContactSegmentSchemaMatchesFrozenContract(t *testing.T) {
	var response resource.SchemaResponse
	NewContactSegmentResource().Schema(context.Background(), resource.SchemaRequest{}, &response)
	if response.Diagnostics.HasError() {
		t.Fatalf("schema diagnostics = %v", response.Diagnostics)
	}
	attributes := response.Schema.Attributes
	if response.Schema.Version != 0 || len(attributes) != 4 {
		t.Fatalf("schema version/shape = %d/%d", response.Schema.Version, len(attributes))
	}
	if !attributes["id"].IsComputed() || !attributes["name"].IsRequired() || !attributes["processing_type"].IsRequired() {
		t.Fatal("root attribute contract is incorrect")
	}
	processingType, ok := attributes["processing_type"].(schema.StringAttribute)
	if !ok || len(processingType.PlanModifiers) == 0 {
		t.Fatal("processing_type must require replacement")
	}
	modifierSchema := schema.Schema{Attributes: map[string]schema.Attribute{"processing_type": schema.StringAttribute{}}}
	stateValue, _ := types.StringValue("manual").ToTerraformValue(context.Background())
	planValue, _ := types.StringValue("dynamic").ToTerraformValue(context.Background())
	modifierRequest := planmodifier.StringRequest{
		State:      tfsdk.State{Schema: modifierSchema, Raw: tftypes.NewValue(modifierSchema.Type().TerraformType(context.Background()), map[string]tftypes.Value{"processing_type": stateValue})},
		Plan:       tfsdk.Plan{Schema: modifierSchema, Raw: tftypes.NewValue(modifierSchema.Type().TerraformType(context.Background()), map[string]tftypes.Value{"processing_type": planValue})},
		StateValue: types.StringValue("manual"), PlanValue: types.StringValue("dynamic"),
	}
	modifierResponse := &planmodifier.StringResponse{PlanValue: modifierRequest.PlanValue}
	for _, modifier := range processingType.PlanModifiers {
		modifier.PlanModifyString(context.Background(), modifierRequest, modifierResponse)
	}
	if !modifierResponse.RequiresReplace {
		t.Fatal("processing_type change did not require replacement")
	}
	groups, ok := attributes["filter_groups"].(schema.SetNestedAttribute)
	if !ok || !groups.IsOptional() || groups.IsComputed() {
		t.Fatalf("filter_groups = %#v", attributes["filter_groups"])
	}
	filters, ok := groups.NestedObject.Attributes["filters"].(schema.SetNestedAttribute)
	if !ok || !filters.IsRequired() {
		t.Fatalf("filters = %#v", groups.NestedObject.Attributes["filters"])
	}
	for _, name := range []string{"property", "operator"} {
		if !filters.NestedObject.Attributes[name].IsRequired() {
			t.Fatalf("filters.%s must be required", name)
		}
	}
	for _, name := range []string{"property_kind", "value"} {
		if !filters.NestedObject.Attributes[name].IsOptional() || filters.NestedObject.Attributes[name].IsComputed() {
			t.Fatalf("filters.%s must be optional-only", name)
		}
	}
}

func TestContactSegmentFrozenFilterContract(t *testing.T) {
	tests := []struct {
		name           string
		processingType string
		filter         *contactSegmentFilterModel
		explicitEmpty  bool
		wantErrors     bool
	}{
		{name: "manual without filters", processingType: "manual"},
		{name: "dynamic text equality", processingType: "dynamic", filter: contactSegmentFilterForTest("is_equal_to", "text", "active")},
		{name: "snapshot select inequality", processingType: "snapshot", filter: contactSegmentFilterForTest("is_not_equal_to", "select", "inactive")},
		{name: "presence omits kind and value", processingType: "dynamic", filter: contactSegmentFilterForTest("is_known", "", "")},
		{name: "dynamic requires filters", processingType: "dynamic", wantErrors: true},
		{name: "manual forbids filters", processingType: "manual", filter: contactSegmentFilterForTest("is_known", "", ""), wantErrors: true},
		{name: "manual forbids an explicit empty set", processingType: "manual", explicitEmpty: true, wantErrors: true},
		{name: "equality requires kind", processingType: "dynamic", filter: contactSegmentFilterForTest("is_equal_to", "", "active"), wantErrors: true},
		{name: "equality requires value", processingType: "dynamic", filter: contactSegmentFilterForTest("is_equal_to", "text", ""), wantErrors: true},
		{name: "presence forbids explicit kind", processingType: "dynamic", filter: contactSegmentFilterForTest("is_known", "text", ""), wantErrors: true},
		{name: "presence forbids explicit value", processingType: "dynamic", filter: contactSegmentFilterForTest("is_not_known", "", "active"), wantErrors: true},
		{name: "unsupported operator", processingType: "dynamic", filter: contactSegmentFilterForTest("contains", "text", "active"), wantErrors: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			model := contactSegmentResourceModel{
				Name: types.StringValue("Active customers"), ProcessingType: types.StringValue(test.processingType),
				FilterGroups: types.SetNull(contactSegmentGroupObjectType),
			}
			if test.filter != nil {
				model.FilterGroups = contactSegmentGroupsForTest(t, *test.filter)
			} else if test.explicitEmpty {
				model.FilterGroups = types.SetValueMust(contactSegmentGroupObjectType, []attr.Value{})
			}
			write, diagnostics := contactSegmentWriteFromModel(context.Background(), model, false)
			if diagnostics.HasError() != test.wantErrors {
				t.Fatalf("HasError = %v, want %v; diagnostics = %#v", diagnostics.HasError(), test.wantErrors, diagnostics)
			}
			if !test.wantErrors && write.ProcessingType != test.processingType {
				t.Fatalf("write processing type = %q", write.ProcessingType)
			}
		})
	}
}

func TestContactSegmentModelCanonicalizesSetOrder(t *testing.T) {
	left := []hubspot.ContactSegmentFilterGroup{
		{Filters: []hubspot.ContactSegmentFilter{
			{Property: "status", Operator: "is_equal_to", PropertyKind: "select", Value: "active"},
			{Property: "note", Operator: "is_known"},
		}},
		{Filters: []hubspot.ContactSegmentFilter{{Property: "tier", Operator: "is_not_equal_to", PropertyKind: "text", Value: "free"}}},
	}
	right := []hubspot.ContactSegmentFilterGroup{
		{Filters: []hubspot.ContactSegmentFilter{{Property: "tier", Operator: "is_not_equal_to", PropertyKind: "text", Value: "free"}}},
		{Filters: []hubspot.ContactSegmentFilter{
			{Property: "note", Operator: "is_known"},
			{Property: "status", Operator: "is_equal_to", PropertyKind: "select", Value: "active"},
		}},
	}
	if !hubspot.ContactSegmentGroupsEqual(left, right) {
		t.Fatal("group and filter ordering must not be semantic")
	}
	remote := hubspot.ContactSegment{ID: "701", Name: "Active customers", ProcessingType: "dynamic", FilterGroups: left}
	var diagnostics diag.Diagnostics
	model := contactSegmentModelFromRemote(context.Background(), remote, &diagnostics)
	if diagnostics.HasError() {
		t.Fatalf("model diagnostics = %#v", diagnostics)
	}
	write, writeDiagnostics := contactSegmentWriteFromModel(context.Background(), model, false)
	if writeDiagnostics.HasError() || !hubspot.ContactSegmentGroupsEqual(write.FilterGroups, right) {
		t.Fatalf("round trip diagnostics/groups = %#v/%#v", writeDiagnostics, write.FilterGroups)
	}
}

func TestContactSegmentApplyConversionNeverDropsUnknownFilters(t *testing.T) {
	unknown := *contactSegmentFilterForTest("is_equal_to", "text", "active")
	unknown.Value = types.StringUnknown()
	model := contactSegmentResourceModel{
		Name: types.StringValue("Active customers"), ProcessingType: types.StringValue("dynamic"),
		FilterGroups: contactSegmentGroupsForTest(t, unknown),
	}
	if _, diagnostics := contactSegmentWriteFromModel(context.Background(), model, false); !diagnostics.HasError() {
		t.Fatal("apply conversion silently dropped an unknown filter")
	}
	if _, diagnostics := contactSegmentWriteFromModel(context.Background(), model, true); diagnostics.HasError() {
		t.Fatalf("configuration validation rejected an unknown planned filter: %#v", diagnostics)
	}
}

func TestContactSegmentPollingModelsPendingStaleFailureAndTimeout(t *testing.T) {
	desired := hubspot.ContactSegmentWrite{Name: "Manual", ProcessingType: "manual"}
	t.Run("pending and stale observations complete", func(t *testing.T) {
		calls := 0
		resource := contactSegmentPollingResource(t, func(*http.Request) (*http.Response, error) {
			calls++
			status, version := "PROCESSING", int64(1)
			if calls == 2 {
				status = "COMPLETE"
			}
			if calls >= 3 {
				status, version = "COMPLETE", 2
			}
			return contactSegmentPollingResponse(status, version), nil
		})
		segment, err := resource.waitForActiveDefinition(context.Background(), "701", 2, desired)
		if err != nil || segment.Version != 2 || calls != 3 {
			t.Fatalf("poll result = %#v, %v, calls=%d", segment, err, calls)
		}
	})
	t.Run("terminal failure stops immediately", func(t *testing.T) {
		calls := 0
		resource := contactSegmentPollingResource(t, func(*http.Request) (*http.Response, error) {
			calls++
			return contactSegmentPollingResponse("FAILED", 2), nil
		})
		if _, err := resource.waitForActiveDefinition(context.Background(), "701", 2, desired); err == nil || calls != 1 {
			t.Fatalf("terminal failure = %v, calls=%d", err, calls)
		}
	})
	t.Run("pending observation times out", func(t *testing.T) {
		resource := contactSegmentPollingResource(t, func(*http.Request) (*http.Response, error) {
			return contactSegmentPollingResponse("PROCESSING", 1), nil
		})
		resource.pollTimeout = 4 * time.Millisecond
		if _, err := resource.waitForActiveDefinition(context.Background(), "701", 2, desired); err == nil || !strings.Contains(err.Error(), "timed out") {
			t.Fatalf("timeout error = %v", err)
		}
	})
}

func TestContactSegmentCreateDistinguishesAmbiguousAndRejectedMissingID(t *testing.T) {
	for _, test := range []struct {
		name       string
		status     int
		body       string
		wantTitle  string
		wantDetail string
	}{
		{name: "ambiguous server failure gives import guidance", status: http.StatusServiceUnavailable, wantTitle: "Contact segment creation outcome is unknown", wantDetail: "Exact-ID import is the only safe recovery path"},
		{name: "success without listId gives import guidance", status: http.StatusCreated, body: `{"list":{"futureServiceMetadata":true}}`, wantTitle: "Contact segment creation outcome is unknown", wantDetail: "Exact-ID import is the only safe recovery path"},
		{name: "authoritative rejection fails immediately", status: http.StatusBadRequest, wantTitle: "Contact segment creation failed"},
	} {
		t.Run(test.name, func(t *testing.T) {
			requests := 0
			contactResource := contactSegmentPollingResource(t, func(*http.Request) (*http.Response, error) {
				requests++
				body := test.body
				if body == "" {
					body = `{"category":"VALIDATION_ERROR","message":"write failed"}`
				}
				return &http.Response{StatusCode: test.status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
			})
			var schemaResponse resource.SchemaResponse
			contactResource.Schema(context.Background(), resource.SchemaRequest{}, &schemaResponse)
			plan := tfsdk.Plan{Schema: schemaResponse.Schema}
			planDiagnostics := plan.Set(context.Background(), &contactSegmentResourceModel{
				Name: types.StringValue("Owned"), ProcessingType: types.StringValue("manual"),
				FilterGroups: types.SetNull(contactSegmentGroupObjectType), ID: types.StringUnknown(),
			})
			if planDiagnostics.HasError() {
				t.Fatalf("plan diagnostics = %#v", planDiagnostics)
			}
			response := resource.CreateResponse{State: tfsdk.State{Schema: schemaResponse.Schema}}
			contactResource.Create(context.Background(), resource.CreateRequest{Plan: plan}, &response)
			if !response.Diagnostics.HasError() || response.Diagnostics[0].Summary() != test.wantTitle {
				t.Fatalf("create diagnostics = %#v", response.Diagnostics)
			}
			if test.wantDetail != "" && !strings.Contains(response.Diagnostics[0].Detail(), test.wantDetail) {
				t.Fatalf("create detail = %q", response.Diagnostics[0].Detail())
			}
			if requests != 1 || !response.State.Raw.IsNull() {
				t.Fatalf("create sent %d requests and retained state %v; want one write and no state", requests, !response.State.Raw.IsNull())
			}
		})
	}
}

func TestContactSegmentDeleteCompletesWhenDefinitionIsPermanentlyAbsent(t *testing.T) {
	for _, test := range []struct {
		name         string
		deleteStatus int
		wantMethods  string
	}{
		{name: "delete reports absence", deleteStatus: http.StatusNotFound, wantMethods: "GET,DELETE"},
		{name: "ambiguous delete then absence", deleteStatus: http.StatusServiceUnavailable, wantMethods: "GET,DELETE,GET"},
	} {
		t.Run(test.name, func(t *testing.T) {
			requireContactSegmentDeleteAbsent(t, test.deleteStatus, test.wantMethods)
		})
	}
}

func requireContactSegmentDeleteAbsent(t *testing.T, deleteStatus int, wantMethods string) {
	t.Helper()
	var methods []string
	contactResource := contactSegmentPollingResource(t, func(request *http.Request) (*http.Response, error) {
		methods = append(methods, request.Method)
		if request.Method == http.MethodGet && len(methods) == 1 {
			return contactSegmentPollingResponse("COMPLETE", 1), nil
		}
		status, body := http.StatusNotFound, `{"category":"OBJECT_NOT_FOUND","message":"not found"}`
		if request.Method == http.MethodDelete {
			status = deleteStatus
		}
		return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	var schemaResponse resource.SchemaResponse
	contactResource.Schema(context.Background(), resource.SchemaRequest{}, &schemaResponse)
	state := tfsdk.State{Schema: schemaResponse.Schema}
	if diagnostics := state.Set(context.Background(), &contactSegmentResourceModel{
		ID: types.StringValue("701"), Name: types.StringValue("Manual"), ProcessingType: types.StringValue("manual"),
		FilterGroups: types.SetNull(contactSegmentGroupObjectType),
	}); diagnostics.HasError() {
		t.Fatalf("state diagnostics = %#v", diagnostics)
	}
	response := resource.DeleteResponse{State: state}
	contactResource.Delete(context.Background(), resource.DeleteRequest{State: state}, &response)
	if response.Diagnostics.HasError() {
		t.Fatalf("delete diagnostics = %#v", response.Diagnostics)
	}
	if !response.State.Raw.IsNull() {
		t.Fatal("permanently absent Contact segment remained in state")
	}
	if strings.Join(methods, ",") != wantMethods {
		t.Fatalf("delete requests = %v, want exactly %s", methods, wantMethods)
	}
}

type contactSegmentRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn contactSegmentRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func contactSegmentPollingResource(t *testing.T, roundTrip contactSegmentRoundTripFunc) *ContactSegmentResource {
	t.Helper()
	origin, err := url.Parse("https://hubspot.invalid")
	if err != nil {
		t.Fatal(err)
	}
	clients, err := hubspot.NewClientSet(hubspot.TransportConfig{
		BaseURL: origin, AccessToken: "test", HTTPClient: &http.Client{Transport: roundTrip},
		Sleep: func(context.Context, time.Duration) error { return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	return &ContactSegmentResource{client: clients.ContactSegments, pollInterval: time.Millisecond, pollTimeout: 20 * time.Millisecond}
}

func contactSegmentPollingResponse(status string, version int64) *http.Response {
	body := `{"list":{"listId":"701","listVersion":` + fmtInt64(version) + `,"name":"Manual","objectTypeId":"0-1","processingStatus":"` + status + `","processingType":"MANUAL","future":true}}`
	return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}

func fmtInt64(value int64) string {
	return strconv.FormatInt(value, 10)
}

func contactSegmentFilterForTest(operator, kind, value string) *contactSegmentFilterModel {
	model := &contactSegmentFilterModel{
		Property: types.StringValue("ns_onboarding_status"), Operator: types.StringValue(operator),
		PropertyKind: types.StringNull(), Value: types.StringNull(),
	}
	if kind != "" {
		model.PropertyKind = types.StringValue(kind)
	}
	if value != "" {
		model.Value = types.StringValue(value)
	}
	return model
}

func contactSegmentGroupsForTest(t *testing.T, filters ...contactSegmentFilterModel) types.Set {
	t.Helper()
	filterSet, diagnostics := types.SetValueFrom(context.Background(), contactSegmentFilterObjectType, filters)
	if diagnostics.HasError() {
		t.Fatalf("filter set diagnostics = %#v", diagnostics)
	}
	groupSet, diagnostics := types.SetValueFrom(context.Background(), contactSegmentGroupObjectType, []contactSegmentGroupModel{{Filters: filterSet}})
	if diagnostics.HasError() {
		t.Fatalf("group set diagnostics = %#v", diagnostics)
	}
	return groupSet
}
