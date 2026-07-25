// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

package acceptance

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/jackemcpherson/terraform-provider-hubspot/internal/hubspot"
)

// RealPortalOrigin is the production HubSpot API origin used by acceptance
// sessions and the janitor whenever no fake ProbeBaseURL is configured.
const RealPortalOrigin = "https://api.hubapi.com"

// portalIdentityEnvVar names the environment variable that must hold the
// expected HubSpot portal identifier before a mutating acceptance run
// against a real portal can start.
const portalIdentityEnvVar = "HUBSPOT_ACCEPTANCE_PORTAL_ID"

// portalIdentityGuard resolves a token's actual HubSpot portal identity
// exactly once and caches the verdict for the remainder of the process. It
// fails closed: a missing expected portal id, an unreachable account-info
// API, or a mismatched portal id are all treated as verification failure.
type portalIdentityGuard struct {
	once sync.Once
	err  error
}

func (g *portalIdentityGuard) verify(ctx context.Context, clients *hubspot.ClientSet) error {
	g.once.Do(func() {
		g.err = resolvePortalIdentity(ctx, clients)
	})
	return g.err
}

func resolvePortalIdentity(ctx context.Context, clients *hubspot.ClientSet) error {
	expected := strings.TrimSpace(os.Getenv(portalIdentityEnvVar))
	if expected == "" {
		return fmt.Errorf("%s is required before a mutating acceptance run against a real HubSpot portal can start", portalIdentityEnvVar)
	}
	info, err := clients.AccountInfo.Get(ctx)
	if err != nil {
		return fmt.Errorf("resolve HubSpot portal identity: %s", SanitizedHubSpotError(err))
	}
	actual := strconv.FormatInt(info.PortalID, 10)
	if actual != expected {
		return fmt.Errorf("HubSpot portal identity mismatch: the configured token did not resolve to the portal required by %s", portalIdentityEnvVar)
	}
	return nil
}

// defaultPortalGuard is the process-wide guard shared by every real-portal
// client bootstrap in this package, so the account-info API is called at
// most once per test binary regardless of how many sessions or janitor
// invocations request verification.
var defaultPortalGuard = &portalIdentityGuard{}

// NewRealPortalClientSet bootstraps a HubSpot client set against the real
// portal API (RealPortalOrigin) and verifies the token's portal identity
// through the shared portal identity guard before returning it. Both harness
// sessions (via probeClients) and the janitor share this bootstrap so no
// mutating real-portal operation can proceed without a verified identity.
func NewRealPortalClientSet(ctx context.Context, accessToken, userAgent string) (*hubspot.ClientSet, error) {
	origin, err := url.Parse(RealPortalOrigin)
	if err != nil {
		return nil, fmt.Errorf("parse HubSpot real portal origin: %w", err)
	}
	clients, err := hubspot.NewClientSet(hubspot.TransportConfig{
		BaseURL:     origin,
		AccessToken: accessToken,
		UserAgent:   userAgent,
	})
	if err != nil {
		return nil, fmt.Errorf("configure HubSpot real portal client: %w", err)
	}
	if err := defaultPortalGuard.verify(ctx, clients); err != nil {
		return nil, err
	}
	return clients, nil
}
