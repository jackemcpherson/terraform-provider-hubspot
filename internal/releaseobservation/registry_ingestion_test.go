package releaseobservation_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestRegistryIngestionAcceptsFreshOrdinaryResponses(t *testing.T) {
	t.Parallel()

	newRegistry := func() (*httptest.Server, *atomic.Int32) {
		requests := &atomic.Int32{}
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests.Add(1)
			if got := r.Header.Get("Cache-Control"); got != "" {
				t.Errorf("ordinary request Cache-Control = %q, want empty", got)
			}
			if got := r.Header.Get("Pragma"); got != "" {
				t.Errorf("ordinary request Pragma = %q, want empty", got)
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"versions":[{"version":"1.2.3"}]}`)
		}))
		return server, requests
	}

	terraform, terraformRequests := newRegistry()
	t.Cleanup(terraform.Close)
	opentofu, opentofuRequests := newRegistry()
	t.Cleanup(opentofu.Close)

	output, err := runRegistryObservation(t, terraform.URL+" "+opentofu.URL)
	if err != nil {
		t.Fatalf("observe registry ingestion: %v\n%s", err, output)
	}
	if got := terraformRequests.Load(); got != 1 {
		t.Errorf("Terraform Registry request count = %d, want 1", got)
	}
	if got := opentofuRequests.Load(); got != 1 {
		t.Errorf("OpenTofu Registry request count = %d, want 1", got)
	}
}

func TestRegistryIngestionRevalidatesStaleResponsesThenWaitsForOrdinaryConfirmation(t *testing.T) {
	t.Parallel()

	newRegistry := func() (*httptest.Server, *atomic.Int32) {
		requests := &atomic.Int32{}
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			request := requests.Add(1)
			w.Header().Set("Content-Type", "application/json")
			switch request {
			case 1:
				assertCacheHeaders(t, r, "", "")
				fmt.Fprint(w, `{"versions":[{"version":"1.2.2"}]}`)
			case 2:
				assertCacheHeaders(t, r, "no-cache", "no-cache")
				fmt.Fprint(w, `{"versions":[{"version":"1.2.3"}]}`)
			case 3:
				assertCacheHeaders(t, r, "", "")
				fmt.Fprint(w, `{"versions":[{"version":"1.2.3"}]}`)
			default:
				t.Errorf("unexpected request %d", request)
				http.Error(w, "unexpected", http.StatusInternalServerError)
			}
		}))
		return server, requests
	}

	terraform, terraformRequests := newRegistry()
	t.Cleanup(terraform.Close)
	opentofu, opentofuRequests := newRegistry()
	t.Cleanup(opentofu.Close)

	output, err := runRegistryObservation(t, terraform.URL+" "+opentofu.URL)
	if err != nil {
		t.Fatalf("observe registry ingestion: %v\n%s", err, output)
	}
	if got := terraformRequests.Load(); got != 3 {
		t.Errorf("Terraform Registry request count = %d, want 3", got)
	}
	if got := opentofuRequests.Load(); got != 3 {
		t.Errorf("OpenTofu Registry request count = %d, want 3", got)
	}
}

func TestRegistryIngestionRejectsUnconfirmedOrUnsafeResponses(t *testing.T) {
	testCases := map[string]struct {
		response       func(*testing.T, http.ResponseWriter, *http.Request)
		responseClass  string
		requestTimeout string
		requestCount   int32
	}{
		"revalidation fresh but ordinary stale": {
			response: func(t *testing.T, w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Cache-Control") == "no-cache" {
					assertCacheHeaders(t, r, "no-cache", "no-cache")
					fmt.Fprint(w, `{"versions":[{"version":"1.2.3"}]}`)
					return
				}
				assertCacheHeaders(t, r, "", "")
				fmt.Fprint(w, `{"versions":[{"version":"1.2.2"}]}`)
			},
			responseClass: "ordinary-version-absent,revalidation-version-present",
			requestCount:  24,
		},
		"persistent stale": {
			response: func(t *testing.T, w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, `{"versions":[{"version":"1.2.2"}]}`)
			},
			responseClass: "ordinary-version-absent,revalidation-version-absent",
			requestCount:  24,
		},
		"malformed ordinary": {
			response: func(t *testing.T, w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, "malformed-ordinary-sensitive-body")
			},
			responseClass: "ordinary-malformed-json",
			requestCount:  12,
		},
		"malformed revalidation": {
			response: func(t *testing.T, w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Cache-Control") == "no-cache" {
					fmt.Fprint(w, "malformed-revalidation-sensitive-body")
					return
				}
				fmt.Fprint(w, `{"versions":[{"version":"1.2.2"}]}`)
			},
			responseClass: "ordinary-version-absent,revalidation-malformed-json",
			requestCount:  24,
		},
		"HTTP failure": {
			response: func(t *testing.T, w http.ResponseWriter, r *http.Request) {
				http.Error(w, "http-failure-sensitive-body", http.StatusServiceUnavailable)
			},
			responseClass: "ordinary-http-status",
			requestCount:  12,
		},
		"redirect status with valid-looking body": {
			response: func(t *testing.T, w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusFound)
				fmt.Fprint(w, `{"versions":[{"version":"1.2.3"}]}`)
			},
			responseClass: "ordinary-http-status",
			requestCount:  12,
		},
		"timeout": {
			response: func(t *testing.T, w http.ResponseWriter, r *http.Request) {
				time.Sleep(50 * time.Millisecond)
				fmt.Fprint(w, "timeout-sensitive-body")
			},
			responseClass:  "ordinary-timeout",
			requestTimeout: "0.01",
			requestCount:   12,
		},
		"missing versions collection": {
			response: func(t *testing.T, w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, `{"providers":[]}`)
			},
			responseClass: "ordinary-missing-versions",
			requestCount:  12,
		},
		"valid JSON without an object": {
			response: func(t *testing.T, w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, `false`)
			},
			responseClass: "ordinary-missing-versions",
			requestCount:  12,
		},
		"invalid version record": {
			response: func(t *testing.T, w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, `{"versions":[{"version":"not-semver"}]}`)
			},
			responseClass: "ordinary-invalid-version-records",
			requestCount:  12,
		},
		"invalid SemVer identifiers": {
			response: func(t *testing.T, w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, `{"versions":[{"version":"1.2.3-01"},{"version":"1.2.3-alpha..1"},{"version":"1.2.3-alpha."},{"version":"1.2.3+meta..x"}]}`)
			},
			responseClass: "ordinary-invalid-version-records",
			requestCount:  12,
		},
		"duplicate version record": {
			response: func(t *testing.T, w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, `{"versions":[{"version":"1.2.3"},{"version":"1.2.3"}]}`)
			},
			responseClass: "ordinary-duplicate-version-records",
			requestCount:  12,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				w.Header().Set("Content-Type", "application/json")
				testCase.response(t, w, r)
			}))
			t.Cleanup(server.Close)

			extraEnv := []string(nil)
			if testCase.requestTimeout != "" {
				extraEnv = append(extraEnv, "REGISTRY_OBSERVATION_REQUEST_TIMEOUT_SECONDS="+testCase.requestTimeout)
			}
			output, err := runRegistryObservation(t, server.URL, extraEnv...)
			if err == nil {
				t.Fatalf("registry observation unexpectedly passed\n%s", output)
			}
			if !strings.Contains(output, "after 12 attempts: "+testCase.responseClass) {
				t.Errorf("diagnostic = %q, want response class %q", output, testCase.responseClass)
			}
			if !strings.Contains(output, strings.TrimPrefix(server.URL, "http://")) {
				t.Errorf("diagnostic = %q, want registry host", output)
			}
			if strings.Contains(output, "sensitive-body") {
				t.Errorf("diagnostic disclosed response body: %q", output)
			}
			if got := requests.Load(); got != testCase.requestCount {
				t.Errorf("request count = %d, want %d", got, testCase.requestCount)
			}
		})
	}
}

func assertCacheHeaders(t *testing.T, request *http.Request, cacheControl, pragma string) {
	t.Helper()
	if got := request.Header.Get("Cache-Control"); got != cacheControl {
		t.Errorf("Cache-Control = %q, want %q", got, cacheControl)
	}
	if got := request.Header.Get("Pragma"); got != pragma {
		t.Errorf("Pragma = %q, want %q", got, pragma)
	}
}

func runRegistryObservation(t *testing.T, endpoints string, extraEnv ...string) (string, error) {
	t.Helper()

	root := filepath.Clean(filepath.Join("..", ".."))
	script := filepath.Join(root, "scripts", "verify-registry-ingestion.sh")
	bin := t.TempDir()
	sleep := filepath.Join(bin, "sleep")
	if err := os.WriteFile(sleep, []byte("#!/bin/sh\ntest \"$1\" = 10\n"), 0o755); err != nil {
		t.Fatalf("write sleep test double: %v", err)
	}
	command := exec.Command("/bin/sh", script, "v1.2.3")
	command.Env = append(os.Environ(),
		"PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"REGISTRY_OBSERVATION_ENDPOINTS="+endpoints,
	)
	command.Env = append(command.Env, extraEnv...)
	output, err := command.CombinedOutput()
	return strings.TrimSpace(string(output)), err
}
