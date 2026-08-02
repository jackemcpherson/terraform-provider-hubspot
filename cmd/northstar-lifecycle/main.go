// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/jackemcpherson/terraform-provider-hubspot/internal/acceptance"
	"github.com/jackemcpherson/terraform-provider-hubspot/internal/hubspot"
)

const driftLabel = "Out-of-band Northstar buyer role"

func main() {
	if len(os.Args) != 2 {
		fatal(errors.New("usage: northstar-lifecycle drift|archive-for-refresh"))
	}
	token := os.Getenv("HUBSPOT_ACCESS_TOKEN")
	if token == "" {
		fatal(errors.New("HUBSPOT_ACCESS_TOKEN is required"))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	clients, err := acceptance.NewRealPortalClientSet(ctx, token, "terraform-provider-hubspot/northstar-lifecycle")
	if err != nil {
		fatal(err)
	}
	if err := execute(ctx, os.Args[1], clients); err != nil {
		fatal(err)
	}
}

func execute(ctx context.Context, action string, clients *hubspot.ClientSet) error {
	switch action {
	case "drift":
		current, err := clients.Properties.Get(ctx, "contacts", "ns_buyer_role", false, "non_sensitive", "")
		if err != nil {
			return fmt.Errorf("read Northstar drift target: %s", acceptance.SanitizedHubSpotError(err))
		}
		if _, err := clients.Properties.Update(ctx, "contacts", current.Name, current.WriteWithLabel(driftLabel)); err != nil {
			return fmt.Errorf("author Northstar drift: %s", acceptance.SanitizedHubSpotError(err))
		}
		updated, err := clients.Properties.Get(ctx, "contacts", current.Name, false, "non_sensitive", "")
		if err != nil || updated.Label != driftLabel {
			return errors.New("northstar drift mutation was not observable")
		}
		return nil
	case "archive-for-refresh":
		const name = "ns_last_success_review"
		if err := clients.Properties.Archive(ctx, "contacts", name); err != nil {
			return fmt.Errorf("archive Northstar refresh target: %s", acceptance.SanitizedHubSpotError(err))
		}
		_, err := clients.Properties.Get(ctx, "contacts", name, false, "non_sensitive", "")
		var apiError *hubspot.Error
		if !errors.As(err, &apiError) || apiError.Status != 404 {
			return errors.New("northstar refresh target remained active after archive")
		}
		return nil
	default:
		return errors.New("action must be drift or archive-for-refresh")
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
