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

func TestArchivePropertyGroupAndVerifyAbsentFailsClosed(t *testing.T) {
	tests := []struct {
		name          string
		archiveStatus int
		getStatus     int
		wantError     bool
	}{
		{name: "archived", archiveStatus: http.StatusNoContent, getStatus: http.StatusNotFound},
		{name: "already absent", archiveStatus: http.StatusInternalServerError, getStatus: http.StatusNotFound},
		{name: "still active", archiveStatus: http.StatusInternalServerError, getStatus: http.StatusOK, wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
				switch request.Method {
				case http.MethodDelete:
					response.WriteHeader(test.archiveStatus)
				case http.MethodGet:
					response.WriteHeader(test.getStatus)
					if test.getStatus == http.StatusOK {
						_, _ = response.Write([]byte(`{"name":"probe","label":"Probe","displayOrder":-1,"archived":false}`))
					}
				default:
					t.Fatalf("unexpected probe request: %s", request.Method)
				}
			}))
			defer server.Close()
			origin, err := url.Parse(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			clients, err := hubspot.NewClientSet(hubspot.TransportConfig{BaseURL: origin, AccessToken: "sentinel", UserAgent: "probe-test"})
			if err != nil {
				t.Fatal(err)
			}
			err = archivePropertyGroupAndVerifyAbsent(context.Background(), clients, "contacts", "probe")
			if (err != nil) != test.wantError {
				t.Fatalf("archive error = %v, want error %t", err, test.wantError)
			}
		})
	}
}
