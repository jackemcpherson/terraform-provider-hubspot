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
	"net/url"
	"reflect"
	"testing"
	"time"
)

func TestContactSegmentClientCreatesSemanticFilterKinds(t *testing.T) {
	handler := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.RequestURI() != "/crm/lists/2026-03" {
			t.Fatalf("request = %s %s", request.Method, request.URL.RequestURI())
		}
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatalf("decode create request: %v", err)
		}
		want := map[string]any{
			"name": "Active customers", "objectTypeId": "0-1", "processingType": "DYNAMIC",
			"filterBranch": map[string]any{
				"filterBranchType": "OR", "filterBranchOperator": "OR", "filters": []any{},
				"filterBranches": []any{map[string]any{
					"filterBranchType": "AND", "filterBranchOperator": "AND", "filterBranches": []any{},
					"filters": []any{
						map[string]any{
							"filterType": "PROPERTY", "property": "firstname",
							"operation": map[string]any{"operationType": "STRING", "operator": "IS_EQUAL_TO", "value": "Alice", "includeObjectsWithNoValueSet": false},
						},
						map[string]any{
							"filterType": "PROPERTY", "property": "lifecyclestage",
							"operation": map[string]any{"operationType": "ENUMERATION", "operator": "IS_NONE_OF", "values": []any{"lead"}, "includeObjectsWithNoValueSet": false},
						},
						map[string]any{
							"filterType": "PROPERTY", "property": "email",
							"operation": map[string]any{"operationType": "ALL_PROPERTY", "operator": "IS_KNOWN", "includeObjectsWithNoValueSet": false},
						},
					},
				}},
			},
		}
		if !reflect.DeepEqual(body, want) {
			encoded, _ := json.Marshal(body)
			t.Fatalf("create body = %s", encoded)
		}
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusCreated)
		io.WriteString(writer, `{"list":{"listId":"701","listVersion":1,"name":"Active customers","objectTypeId":"0-1","processingStatus":"COMPLETE","processingType":"DYNAMIC","filterBranch":null,"futureField":true},"futureEnvelope":true}`)
	})

	client := newContactSegmentTestClient(t, handler)
	created, err := client.Create(context.Background(), ContactSegmentWrite{
		Name: "Active customers", ProcessingType: "dynamic",
		FilterGroups: []ContactSegmentFilterGroup{{Filters: []ContactSegmentFilter{
			{Property: "firstname", Operator: "is_equal_to", PropertyKind: "text", Value: "Alice"},
			{Property: "lifecyclestage", Operator: "is_not_equal_to", PropertyKind: "select", Value: "lead"},
			{Property: "email", Operator: "is_known"},
		}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.ID != "701" || created.Version != 1 || created.Name != "Active customers" || created.ProcessingType != "dynamic" || created.ProcessingStatus != "COMPLETE" || created.Deleted {
		t.Fatalf("created segment = %#v", created)
	}
}

func TestContactSegmentClientReadsAndNormalizesSupportedDefinitions(t *testing.T) {
	handler := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.RequestURI() != "/crm/lists/2026-03/701?includeFilters=true" {
			t.Fatalf("request = %s %s", request.Method, request.URL.RequestURI())
		}
		writer.Header().Set("Content-Type", "application/json")
		io.WriteString(writer, `{"list":{"listId":"701","listVersion":3,"name":"Active customers","objectTypeId":"0-1","processingStatus":"COMPLETE","processingType":"DYNAMIC","deletedAt":"2026-08-18T05:12:00Z","filterBranch":{"filterBranchType":"OR","filterBranchOperator":"OR","filters":[],"filterBranches":[{"filterBranchType":"AND","filterBranchOperator":"AND","filterBranches":[],"filters":[{"filterType":"PROPERTY","property":"firstname","operation":{"operationType":"STRING","operator":"IS_EQUAL_TO","value":"Alice","includeObjectsWithNoValueSet":false}},{"filterType":"PROPERTY","property":"lastname","operation":{"operationType":"MULTISTRING","operator":"IS_NOT_EQUAL_TO","values":["Smith"],"includeObjectsWithNoValueSet":false}},{"filterType":"PROPERTY","property":"lifecyclestage","operation":{"operationType":"ENUMERATION","operator":"IS_ANY_OF","values":["lead"],"includeObjectsWithNoValueSet":false}},{"filterType":"PROPERTY","property":"email","operation":{"operationType":"ALL_PROPERTY","operator":"IS_UNKNOWN","includeObjectsWithNoValueSet":false}}]}]}}}`)
	})

	client := newContactSegmentTestClient(t, handler)
	segment, err := client.Get(context.Background(), "701")
	if err != nil {
		t.Fatal(err)
	}
	wantFilters := []ContactSegmentFilter{
		{Property: "firstname", Operator: "is_equal_to", PropertyKind: "text", Value: "Alice"},
		{Property: "lastname", Operator: "is_not_equal_to", PropertyKind: "text", Value: "Smith"},
		{Property: "lifecyclestage", Operator: "is_equal_to", PropertyKind: "select", Value: "lead"},
		{Property: "email", Operator: "is_not_known"},
	}
	if segment.ID != "701" || segment.Version != 3 || !segment.Deleted || segment.ProcessingType != "dynamic" || len(segment.FilterGroups) != 1 || !reflect.DeepEqual(segment.FilterGroups[0].Filters, wantFilters) {
		t.Fatalf("segment = %#v", segment)
	}
}

func TestContactSegmentClientUsesExactMutationRoutes(t *testing.T) {
	requests := make([]string, 0, 4)
	handler := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests = append(requests, request.Method+" "+request.URL.RequestURI())
		writer.Header().Set("Content-Type", "application/json")
		switch len(requests) {
		case 1:
			io.WriteString(writer, `{"updatedList":{"listId":"701","listVersion":2,"name":"Renamed","objectTypeId":"0-1","processingStatus":"COMPLETE","processingType":"DYNAMIC"}}`)
		case 2:
			var body map[string]any
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Fatalf("decode filter update: %v", err)
			}
			if len(body) != 1 || body["filterBranch"] == nil {
				t.Fatalf("filter update body = %#v", body)
			}
			io.WriteString(writer, `{"updatedList":{"listId":"701","listVersion":3,"name":"Renamed","objectTypeId":"0-1","processingStatus":"PROCESSING","processingType":"DYNAMIC"}}`)
		case 3, 4:
			writer.WriteHeader(http.StatusNoContent)
		default:
			t.Fatal("unexpected extra request")
		}
	})

	client := newContactSegmentTestClient(t, handler)
	renamed, err := client.UpdateName(context.Background(), "701", "Renamed")
	if err != nil || renamed.ID != "701" || renamed.Version != 2 || renamed.Name != "Renamed" {
		t.Fatalf("rename = %#v, %v", renamed, err)
	}
	updated, err := client.UpdateFilters(context.Background(), "701", []ContactSegmentFilterGroup{{Filters: []ContactSegmentFilter{
		{Property: "firstname", Operator: "is_equal_to", PropertyKind: "text", Value: "Alice"},
	}}})
	if err != nil || updated.ID != "701" || updated.Version != 3 || updated.ProcessingStatus != "PROCESSING" {
		t.Fatalf("filter update = %#v, %v", updated, err)
	}
	if err := client.Delete(context.Background(), "701"); err != nil {
		t.Fatal(err)
	}
	if err := client.Restore(context.Background(), "701"); err != nil {
		t.Fatal(err)
	}
	wantRequests := []string{
		"PUT /crm/lists/2026-03/701/update-list-name?includeFilters=true&listName=Renamed",
		"PUT /crm/lists/2026-03/701/update-list-filters",
		"DELETE /crm/lists/2026-03/701",
		"PUT /crm/lists/2026-03/701/restore",
	}
	if !reflect.DeepEqual(requests, wantRequests) {
		t.Fatalf("requests = %#v", requests)
	}
}

func TestContactSegmentClientRejectsUnsupportedDesiredDefinitions(t *testing.T) {
	tests := map[string]ContactSegmentWrite{
		"blank name":              {Name: " ", ProcessingType: "manual"},
		"unknown processing type": {Name: "Owned", ProcessingType: "active"},
		"manual filters": {
			Name: "Owned", ProcessingType: "manual",
			FilterGroups: []ContactSegmentFilterGroup{{Filters: []ContactSegmentFilter{{Property: "email", Operator: "is_known"}}}},
		},
		"dynamic without groups": {Name: "Owned", ProcessingType: "dynamic"},
		"snapshot empty group": {
			Name: "Owned", ProcessingType: "snapshot",
			FilterGroups: []ContactSegmentFilterGroup{{}},
		},
		"equality without kind": {
			Name: "Owned", ProcessingType: "dynamic",
			FilterGroups: []ContactSegmentFilterGroup{{Filters: []ContactSegmentFilter{{Property: "firstname", Operator: "is_equal_to", Value: "Alice"}}}},
		},
		"presence with value": {
			Name: "Owned", ProcessingType: "dynamic",
			FilterGroups: []ContactSegmentFilterGroup{{Filters: []ContactSegmentFilter{{Property: "email", Operator: "is_known", Value: "set"}}}},
		},
	}
	for name, input := range tests {
		t.Run(name, func(t *testing.T) {
			requests := 0
			client := newContactSegmentTestClient(t, http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				requests++
				writer.Header().Set("Content-Type", "application/json")
				io.WriteString(writer, `{"list":{"listId":"701","listVersion":1,"name":"Owned","objectTypeId":"0-1","processingStatus":"COMPLETE","processingType":"DYNAMIC"}}`)
			}))
			if _, err := client.Create(context.Background(), input); err == nil {
				t.Fatal("unsupported desired definition was accepted")
			}
			if requests != 0 {
				t.Fatalf("requests = %d, want 0", requests)
			}
		})
	}
}

func TestContactSegmentClientRejectsDuplicateRemotePredicates(t *testing.T) {
	handler := http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		io.WriteString(writer, `{"list":{"listId":"701","listVersion":1,"name":"Owned","objectTypeId":"0-1","processingStatus":"COMPLETE","processingType":"DYNAMIC","filterBranch":{"filterBranchType":"OR","filterBranchOperator":"OR","filters":[],"filterBranches":[{"filterBranchType":"AND","filterBranchOperator":"AND","filterBranches":[],"filters":[{"filterType":"PROPERTY","property":"email","operation":{"operationType":"ALL_PROPERTY","operator":"IS_KNOWN","includeObjectsWithNoValueSet":false}},{"filterType":"PROPERTY","property":"email","operation":{"operationType":"ALL_PROPERTY","operator":"IS_KNOWN","includeObjectsWithNoValueSet":false}}]}]}}}`)
	})
	client := newContactSegmentTestClient(t, handler)
	if _, err := client.Get(context.Background(), "701"); err == nil {
		t.Fatal("duplicate remote predicates were silently collapsed")
	}
}

func TestContactSegmentClientRejectsDuplicateRemoteGroups(t *testing.T) {
	handler := http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		io.WriteString(writer, `{"list":{"listId":"701","listVersion":1,"name":"Owned","objectTypeId":"0-1","processingStatus":"COMPLETE","processingType":"DYNAMIC","filterBranch":{"filterBranchType":"OR","filterBranchOperator":"OR","filters":[],"filterBranches":[{"filterBranchType":"AND","filterBranchOperator":"AND","filterBranches":[],"filters":[{"filterType":"PROPERTY","property":"email","operation":{"operationType":"ALL_PROPERTY","operator":"IS_KNOWN","includeObjectsWithNoValueSet":false}}]},{"filterBranchType":"AND","filterBranchOperator":"AND","filterBranches":[],"filters":[{"filterType":"PROPERTY","property":"email","operation":{"operationType":"ALL_PROPERTY","operator":"IS_KNOWN","includeObjectsWithNoValueSet":false}}]}]}}}`)
	})
	client := newContactSegmentTestClient(t, handler)
	if _, err := client.Get(context.Background(), "701"); err == nil {
		t.Fatal("duplicate remote groups were silently collapsed")
	}
}

func TestContactSegmentClientPreservesIDFromAmbiguousCreateResponse(t *testing.T) {
	handler := http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusCreated)
		io.WriteString(writer, `{"list":{"listId":"701","listVersion":"malformed"}}`)
	})
	client := newContactSegmentTestClient(t, handler)
	created, err := client.Create(context.Background(), ContactSegmentWrite{Name: "Owned", ProcessingType: "manual"})
	if err == nil || created.ID != "701" {
		t.Fatalf("create = %#v, %v", created, err)
	}
	var apiError *Error
	if !errors.As(err, &apiError) || !apiError.Ambiguous {
		t.Fatalf("error = %#v, want ambiguous", err)
	}
}

func TestContactSegmentClientMarksMalformedUpdateSuccessAmbiguous(t *testing.T) {
	handler := http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		io.WriteString(writer, `{"updatedList":{"listId":"701","listVersion":"malformed"}}`)
	})
	client := newContactSegmentTestClient(t, handler)
	updated, err := client.UpdateName(context.Background(), "701", "Renamed")
	if err == nil || updated.ID != "701" {
		t.Fatalf("update = %#v, %v", updated, err)
	}
	var apiError *Error
	if !errors.As(err, &apiError) || !apiError.Ambiguous {
		t.Fatalf("error = %#v, want ambiguous", err)
	}
}

func TestContactSegmentClientRejectsUnsupportedRemoteDefinitions(t *testing.T) {
	tests := map[string]string{
		"missing identity":      `{"list":{"listVersion":1,"name":"Owned","objectTypeId":"0-1","processingStatus":"COMPLETE","processingType":"MANUAL"}}`,
		"noncanonical identity": `{"list":{"listId":"0701","listVersion":1,"name":"Owned","objectTypeId":"0-1","processingStatus":"COMPLETE","processingType":"MANUAL"}}`,
		"wrong object type":     `{"list":{"listId":"701","listVersion":1,"name":"Owned","objectTypeId":"0-2","processingStatus":"COMPLETE","processingType":"MANUAL"}}`,
		"unsupported type":      `{"list":{"listId":"701","listVersion":1,"name":"Owned","objectTypeId":"0-1","processingStatus":"COMPLETE","processingType":"BATCH"}}`,
		"dynamic without tree":  `{"list":{"listId":"701","listVersion":1,"name":"Owned","objectTypeId":"0-1","processingStatus":"COMPLETE","processingType":"DYNAMIC"}}`,
		"manual with tree":      `{"list":{"listId":"701","listVersion":1,"name":"Owned","objectTypeId":"0-1","processingStatus":"COMPLETE","processingType":"MANUAL","filterBranch":{"filterBranchType":"OR","filterBranchOperator":"OR","filters":[],"filterBranches":[{"filterBranchType":"AND","filterBranchOperator":"AND","filterBranches":[],"filters":[{"filterType":"PROPERTY","property":"email","operation":{"operationType":"ALL_PROPERTY","operator":"IS_KNOWN","includeObjectsWithNoValueSet":false}}]}]}}}`,
		"multiple values":       `{"list":{"listId":"701","listVersion":1,"name":"Owned","objectTypeId":"0-1","processingStatus":"COMPLETE","processingType":"DYNAMIC","filterBranch":{"filterBranchType":"OR","filterBranchOperator":"OR","filters":[],"filterBranches":[{"filterBranchType":"AND","filterBranchOperator":"AND","filterBranches":[],"filters":[{"filterType":"PROPERTY","property":"status","operation":{"operationType":"ENUMERATION","operator":"IS_ANY_OF","values":["one","two"],"includeObjectsWithNoValueSet":false}}]}]}}}`,
		"includes unset":        `{"list":{"listId":"701","listVersion":1,"name":"Owned","objectTypeId":"0-1","processingStatus":"COMPLETE","processingType":"DYNAMIC","filterBranch":{"filterBranchType":"OR","filterBranchOperator":"OR","filters":[],"filterBranches":[{"filterBranchType":"AND","filterBranchOperator":"AND","filterBranches":[],"filters":[{"filterType":"PROPERTY","property":"email","operation":{"operationType":"ALL_PROPERTY","operator":"IS_KNOWN","includeObjectsWithNoValueSet":true}}]}]}}}`,
	}
	for name, document := range tests {
		t.Run(name, func(t *testing.T) {
			client := newContactSegmentTestClient(t, http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writer.Header().Set("Content-Type", "application/json")
				io.WriteString(writer, document)
			}))
			if _, err := client.Get(context.Background(), "701"); err == nil {
				t.Fatal("unsupported remote definition was accepted")
			}
		})
	}
}

func TestContactSegmentClientMarksTypedModelSuccessAmbiguous(t *testing.T) {
	client := newContactSegmentTestClient(t, http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusCreated)
		io.WriteString(writer, `{"list":{"future":true}}`)
	}))
	created, err := client.Create(context.Background(), ContactSegmentWrite{Name: "Owned", ProcessingType: "manual"})
	var apiError *Error
	if created.ID != "" || !errors.As(err, &apiError) || !apiError.Ambiguous || apiError.Status != http.StatusCreated {
		t.Fatalf("create/model error = %#v, %#v", created, err)
	}
}

func TestContactSegmentClientClassifiesUnsafeServerFailuresAsAmbiguous(t *testing.T) {
	status := http.StatusServiceUnavailable
	client := newContactSegmentTestClient(t, http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(status)
		io.WriteString(writer, `{"category":"TEMPORARY_UNAVAILABLE","message":"outcome unknown"}`)
	}))
	operations := []struct {
		name string
		run  func() error
	}{
		{name: "create", run: func() error {
			_, err := client.Create(context.Background(), ContactSegmentWrite{Name: "Owned", ProcessingType: "manual"})
			return err
		}},
		{name: "name update", run: func() error { _, err := client.UpdateName(context.Background(), "701", "Renamed"); return err }},
		{name: "filter update", run: func() error {
			_, err := client.UpdateFilters(context.Background(), "701", []ContactSegmentFilterGroup{{Filters: []ContactSegmentFilter{{Property: "email", Operator: "is_known"}}}})
			return err
		}},
		{name: "delete", run: func() error { return client.Delete(context.Background(), "701") }},
		{name: "restore", run: func() error { return client.Restore(context.Background(), "701") }},
	}
	for _, operation := range operations {
		t.Run(operation.name, func(t *testing.T) {
			var apiError *Error
			if err := operation.run(); !errors.As(err, &apiError) || !apiError.Ambiguous || apiError.Status != http.StatusServiceUnavailable {
				t.Fatalf("error = %#v, want ambiguous HTTP 503", err)
			}
		})
	}

	status = http.StatusBadRequest
	var apiError *Error
	if err := client.Delete(context.Background(), "701"); !errors.As(err, &apiError) || apiError.Ambiguous {
		t.Fatalf("authoritative rejection = %#v, want non-ambiguous", err)
	}
}

func TestValidContactSegmentIDRequiresCanonicalPositiveDecimal(t *testing.T) {
	for _, id := range []string{"1", "701", "999999999999999999999999999999999999999999"} {
		if !ValidContactSegmentID(id) {
			t.Fatalf("valid ID rejected: %q", id)
		}
	}
	for _, id := range []string{"", "0", "-1", "+1", "01", "1.0", " 1"} {
		if ValidContactSegmentID(id) {
			t.Fatalf("invalid ID accepted: %q", id)
		}
	}
}

func TestContactSegmentMatchesIgnoresGroupAndFilterOrder(t *testing.T) {
	status := ContactSegmentFilter{Property: "status", Operator: "is_equal_to", PropertyKind: "select", Value: "active"}
	note := ContactSegmentFilter{Property: "note", Operator: "is_known"}
	tier := ContactSegmentFilter{Property: "tier", Operator: "is_not_equal_to", PropertyKind: "text", Value: "free"}
	segment := ContactSegment{ID: "701", Name: "Active", ProcessingType: "dynamic", FilterGroups: []ContactSegmentFilterGroup{
		{Filters: []ContactSegmentFilter{status, note}}, {Filters: []ContactSegmentFilter{tier}},
	}}
	want := ContactSegmentWrite{Name: "Active", ProcessingType: "dynamic", FilterGroups: []ContactSegmentFilterGroup{
		{Filters: []ContactSegmentFilter{tier}}, {Filters: []ContactSegmentFilter{note, status}},
	}}
	if !ContactSegmentMatches(segment, want) {
		t.Fatal("group and filter order must not be semantic")
	}
	inactive := ContactSegmentFilter{Property: "status", Operator: "is_equal_to", PropertyKind: "select", Value: "inactive"}
	textStatus := ContactSegmentFilter{Property: "status", Operator: "is_equal_to", PropertyKind: "text", Value: "active"}
	for name, changed := range map[string]ContactSegmentWrite{
		"name":              {Name: "Other", ProcessingType: "dynamic", FilterGroups: want.FilterGroups},
		"processing type":   {Name: "Active", ProcessingType: "snapshot", FilterGroups: want.FilterGroups},
		"filters regrouped": {Name: "Active", ProcessingType: "dynamic", FilterGroups: []ContactSegmentFilterGroup{{Filters: []ContactSegmentFilter{status}}, {Filters: []ContactSegmentFilter{note, tier}}}},
		"value":             {Name: "Active", ProcessingType: "dynamic", FilterGroups: []ContactSegmentFilterGroup{{Filters: []ContactSegmentFilter{tier}}, {Filters: []ContactSegmentFilter{note, inactive}}}},
		"kind":              {Name: "Active", ProcessingType: "dynamic", FilterGroups: []ContactSegmentFilterGroup{{Filters: []ContactSegmentFilter{tier}}, {Filters: []ContactSegmentFilter{note, textStatus}}}},
		"missing group":     {Name: "Active", ProcessingType: "dynamic", FilterGroups: []ContactSegmentFilterGroup{{Filters: []ContactSegmentFilter{tier}}}},
	} {
		if ContactSegmentMatches(segment, changed) {
			t.Fatalf("changed %s matched", name)
		}
	}
}

func TestContactSegmentProcessingFailedRecognisesTerminalStatuses(t *testing.T) {
	for _, status := range []string{"FAILED", "error", "Canceled", "CANCELLED", "REJECTED"} {
		if !ContactSegmentProcessingFailed(status) {
			t.Fatalf("terminal failure %q was not recognised", status)
		}
	}
	for _, status := range []string{"", "COMPLETE", "PROCESSING", "QUEUED"} {
		if ContactSegmentProcessingFailed(status) {
			t.Fatalf("non-failure %q was treated as terminal failure", status)
		}
	}
}

func TestClientSetProvidesContactSegmentClient(t *testing.T) {
	baseURL, err := url.Parse("https://contact-segments.test")
	if err != nil {
		t.Fatal(err)
	}
	clients, err := NewClientSet(TransportConfig{BaseURL: baseURL})
	if err != nil {
		t.Fatal(err)
	}
	if clients.ContactSegments == nil {
		t.Fatal("Contact segment client is unavailable")
	}
}

type contactSegmentRoundTripFunc func(*http.Request) (*http.Response, error)

func (f contactSegmentRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func newContactSegmentTestClient(t *testing.T, handler http.Handler) *ContactSegmentClient {
	t.Helper()
	baseURL, err := url.Parse("https://contact-segments.test")
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: contactSegmentRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		return recorder.Result(), nil
	})}
	transport, err := NewTransport(TransportConfig{
		BaseURL: baseURL, AccessToken: "secret-token", HTTPClient: client,
		Jitter: func(duration time.Duration) time.Duration { return duration },
	})
	if err != nil {
		t.Fatal(err)
	}
	return &ContactSegmentClient{transport: transport}
}
