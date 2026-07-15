// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

package hubspot

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestTransportSendsAuthenticatedVersionedRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if got := request.URL.Path; got != "/crm/properties/2026-03/contacts/groups" {
			t.Fatalf("path = %q", got)
		}
		if got := request.Header.Get("Authorization"); got != "Bearer secret-token" {
			t.Fatalf("authorization = %q", got)
		}
		if got := request.Header.Get("User-Agent"); got != "terraform-provider-hubspot/test protocol/6" {
			t.Fatalf("user agent = %q", got)
		}
		writer.Header().Set("Content-Type", "application/json")
		io.WriteString(writer, `{"name":"marketing","label":"Marketing","displayOrder":-1,"archived":false}`)
	}))
	defer server.Close()

	transport := newTestTransport(t, server.URL)
	var response propertyGroupRead
	err := transport.Do(context.Background(), Operation{
		Name:   "property-group-read",
		Method: http.MethodGet,
		Path:   "/crm/properties/2026-03/contacts/groups",
		Replay: ReplaySafe,
	}, nil, &response)
	if err != nil {
		t.Fatal(err)
	}
	if response.Name != "marketing" {
		t.Fatalf("name = %q", response.Name)
	}
}

func TestTransportRetriesReadAfterRateLimit(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		attempts++
		if attempts == 1 {
			writer.Header().Set("Retry-After", "2")
			writer.WriteHeader(http.StatusTooManyRequests)
			io.WriteString(writer, `{"status":"error","message":"slow down"}`)
			return
		}
		io.WriteString(writer, `{"name":"marketing","label":"Marketing","displayOrder":-1,"archived":false}`)
	}))
	defer server.Close()

	var waits []time.Duration
	transport := newTestTransport(t, server.URL)
	transport.sleep = func(_ context.Context, duration time.Duration) error {
		waits = append(waits, duration)
		return nil
	}
	var response propertyGroupRead
	if err := transport.Do(context.Background(), Operation{
		Name:   "property-group-read",
		Method: http.MethodGet,
		Path:   "/crm/properties/2026-03/contacts/groups",
		Replay: ReplaySafe,
	}, nil, &response); err != nil {
		t.Fatal(err)
	}
	if attempts != 2 || len(waits) != 1 || waits[0] != 2*time.Second {
		t.Fatalf("attempts = %d, waits = %#v", attempts, waits)
	}
}

func TestTransportDoesNotReplayAmbiguousCreate(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		attempts++
		writer.WriteHeader(http.StatusServiceUnavailable)
		io.WriteString(writer, `{"status":"error","message":"unavailable"}`)
	}))
	defer server.Close()

	transport := newTestTransport(t, server.URL)
	err := transport.Do(context.Background(), Operation{
		Name:   "property-group-create",
		Method: http.MethodPost,
		Path:   "/crm/properties/2026-03/contacts/groups",
		Replay: ReplayNever,
	}, strings.NewReader(`{"name":"marketing"}`), nil)
	if err == nil {
		t.Fatal("expected ambiguous create error")
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1", attempts)
	}
	var apiError *Error
	if !errors.As(err, &apiError) || apiError.Status != http.StatusServiceUnavailable {
		t.Fatalf("error = %#v", err)
	}
}

func TestParseErrorPreservesSafeNestedSubcategory(t *testing.T) {
	err := parseError("archive property group", http.StatusBadRequest, http.Header{}, []byte(`{"message":"{\"message\":\"Can't delete a group with active properties\",\"category\":\"VALIDATION_ERROR\",\"subCategory\":\"PropertyGroupError.GROUP_WITH_ACTIVE_PROPERTIES\"}"}`), nil, time.Now())
	if err.Category != "VALIDATION_ERROR" {
		t.Fatalf("category = %q", err.Category)
	}
	if err.SubCategory != "PropertyGroupError.GROUP_WITH_ACTIVE_PROPERTIES" {
		t.Fatalf("subcategory = %q", err.SubCategory)
	}
}

func TestSafeCategoryRejectsUnsafeValues(t *testing.T) {
	unsafe := []string{
		"VALIDATION_ERROR] injected",
		"VALIDATION_ERROR\nINJECTED",
		strings.Repeat("A", 129),
		"pat-na1-credential-shaped-value",
		"ACCESS_TOKEN_EXPIRED",
		"clientSecret",
		"Bearer-value",
		"123e4567-e89b-12d3-a456-426614174000",
		"0123456789abcdef0123456789abcdef",
	}
	for _, value := range unsafe {
		if got := safeCategory(value); got != "" {
			t.Errorf("safeCategory accepted unsafe input %q as %q", value, got)
		}
	}
	for _, value := range []string{"VALIDATION_ERROR", "PropertyGroupError.GROUP_WITH_ACTIVE_PROPERTIES"} {
		if got := safeCategory(value); got != value {
			t.Errorf("safeCategory rejected vetted enum %q", value)
		}
	}
}

func newTestTransport(t *testing.T, baseURL string) *Transport {
	t.Helper()
	parsed, err := url.Parse(baseURL)
	if err != nil {
		t.Fatal(err)
	}
	transport, err := NewTransport(TransportConfig{
		BaseURL:     parsed,
		AccessToken: "secret-token",
		UserAgent:   "terraform-provider-hubspot/test protocol/6",
		Jitter:      func(duration time.Duration) time.Duration { return duration },
	})
	if err != nil {
		t.Fatal(err)
	}
	return transport
}
