// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

package acceptance

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/jackemcpherson/terraform-provider-hubspot/internal/hubspot"
)

func TestPortalIdentityGuardVerifiesMatchingPortal(t *testing.T) {
	t.Setenv(portalIdentityEnvVar, "12345")
	clients := accountInfoStubClients(t, http.StatusOK, `{"portalId":12345}`)
	guard := &portalIdentityGuard{}
	if err := guard.verify(context.Background(), clients); err != nil {
		t.Fatalf("verify() = %v, want no error for a matching portal", err)
	}
}

func TestPortalIdentityGuardFailsOnMismatch(t *testing.T) {
	t.Setenv(portalIdentityEnvVar, "12345")
	clients := accountInfoStubClients(t, http.StatusOK, `{"portalId":99999}`)
	guard := &portalIdentityGuard{}
	if err := guard.verify(context.Background(), clients); err == nil {
		t.Fatal("verify() = nil, want an error for a mismatched portal")
	}
}

func TestPortalIdentityGuardFailsClosedWhenExpectedIDMissing(t *testing.T) {
	t.Setenv(portalIdentityEnvVar, "")
	clients := accountInfoStubClients(t, http.StatusOK, `{"portalId":12345}`)
	guard := &portalIdentityGuard{}
	if err := guard.verify(context.Background(), clients); err == nil {
		t.Fatal("verify() = nil, want an error when the expected portal id is unset")
	}
}

func TestPortalIdentityGuardFailsClosedWhenUnreachable(t *testing.T) {
	t.Setenv(portalIdentityEnvVar, "12345")
	clients := accountInfoStubClients(t, http.StatusOK, `{"portalId":12345}`)
	// A canceled context makes the account-info call fail before any
	// response is available, simulating an unreachable API. The guard must
	// fail closed rather than treat inability to verify as a pass.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	guard := &portalIdentityGuard{}
	if err := guard.verify(ctx, clients); err == nil {
		t.Fatal("verify() = nil, want an error when account-info is unreachable")
	}
}

func TestPortalIdentityGuardCachesVerdictAcrossCalls(t *testing.T) {
	t.Setenv(portalIdentityEnvVar, "12345")
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		requests++
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"portalId":12345}`))
	}))
	defer server.Close()
	origin, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	clients, err := hubspot.NewClientSet(hubspot.TransportConfig{BaseURL: origin, AccessToken: "sentinel", UserAgent: "portal-guard-test"})
	if err != nil {
		t.Fatal(err)
	}
	guard := &portalIdentityGuard{}
	for i := 0; i < 3; i++ {
		if err := guard.verify(context.Background(), clients); err != nil {
			t.Fatalf("verify() call %d = %v", i, err)
		}
	}
	if requests != 1 {
		t.Fatalf("account-info requests = %d, want exactly 1 across repeated verify calls", requests)
	}
}

func TestPortalIdentityGuardCachesFailureVerdictAcrossCalls(t *testing.T) {
	t.Setenv(portalIdentityEnvVar, "12345")
	requests := 0
	// Unauthorized is not a retryable status, so exactly one HTTP request
	// per logical account-info call is expected here.
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		requests++
		response.WriteHeader(http.StatusUnauthorized)
		_, _ = response.Write([]byte(`{"status":"error","message":"unauthorized"}`))
	}))
	defer server.Close()
	origin, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	clients, err := hubspot.NewClientSet(hubspot.TransportConfig{BaseURL: origin, AccessToken: "sentinel", UserAgent: "portal-guard-test"})
	if err != nil {
		t.Fatal(err)
	}
	guard := &portalIdentityGuard{}
	for i := 0; i < 3; i++ {
		if err := guard.verify(context.Background(), clients); err == nil {
			t.Fatalf("verify() call %d = nil, want the cached failure verdict", i)
		}
	}
	if requests != 1 {
		t.Fatalf("account-info requests = %d, want exactly 1 across repeated verify calls", requests)
	}
}

func accountInfoStubClients(t *testing.T, status int, body string) *hubspot.ClientSet {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		response.WriteHeader(status)
		_, _ = response.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	origin, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	clients, err := hubspot.NewClientSet(hubspot.TransportConfig{BaseURL: origin, AccessToken: "sentinel", UserAgent: "portal-guard-test"})
	if err != nil {
		t.Fatal(err)
	}
	return clients
}
