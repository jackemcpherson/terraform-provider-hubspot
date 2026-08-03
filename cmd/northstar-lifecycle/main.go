// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jackemcpherson/terraform-provider-hubspot/internal/acceptance"
	"github.com/jackemcpherson/terraform-provider-hubspot/internal/hubspot"
)

const driftLabel = "Out-of-band Northstar buyer role"

func main() {
	if len(os.Args) < 2 || len(os.Args) > 3 {
		fatal(errors.New("usage: northstar-lifecycle drift|archive-for-refresh|verify-form-terminal [form-id]"))
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
	formID := ""
	if len(os.Args) == 3 {
		formID = os.Args[2]
	}
	result, err := execute(ctx, os.Args[1], formID, clients)
	if err != nil {
		fatal(err)
	}
	if result != "" {
		fmt.Println(result)
	}
}

func execute(ctx context.Context, action, formID string, clients *hubspot.ClientSet) (string, error) {
	switch action {
	case "drift":
		if formID == "" {
			return "", errors.New("northstar form ID is required for drift")
		}
		if err := driftNorthstarProperty(ctx, clients); err != nil {
			return "", err
		}
		if err := driftNorthstarForm(ctx, clients, formID); err != nil {
			return "", err
		}
		return "", nil
	case "archive-for-refresh":
		const name = "ns_last_success_review"
		if err := clients.Properties.Archive(ctx, "contacts", name); err != nil {
			return "", fmt.Errorf("archive Northstar refresh target: %s", acceptance.SanitizedHubSpotError(err))
		}
		_, err := clients.Properties.Get(ctx, "contacts", name, false, "non_sensitive", "")
		var apiError *hubspot.Error
		if !errors.As(err, &apiError) || apiError.Status != 404 {
			return "", errors.New("northstar refresh target remained active after archive")
		}
		return "", nil
	case "verify-form-terminal":
		return verifyNorthstarFormTerminal(ctx, clients, formID)
	default:
		return "", errors.New("action must be drift, archive-for-refresh, or verify-form-terminal")
	}
}

func driftNorthstarProperty(ctx context.Context, clients *hubspot.ClientSet) error {
	current, err := clients.Properties.Get(ctx, "contacts", "ns_buyer_role", false, "non_sensitive", "")
	if err != nil {
		return fmt.Errorf("read Northstar drift target: %s", acceptance.SanitizedHubSpotError(err))
	}
	if _, err := clients.Properties.Update(ctx, "contacts", current.Name, current.WriteWithLabel(driftLabel)); err != nil {
		return fmt.Errorf("author Northstar property drift: %s", acceptance.SanitizedHubSpotError(err))
	}
	updated, err := clients.Properties.Get(ctx, "contacts", current.Name, false, "non_sensitive", "")
	if err != nil || updated.Label != driftLabel {
		return errors.New("northstar property drift mutation was not observable")
	}
	return nil
}

func driftNorthstarForm(ctx context.Context, clients *hubspot.ClientSet, formID string) error {
	form, err := clients.Forms.Get(ctx, formID)
	if err != nil {
		return fmt.Errorf("read Northstar form drift target: %s", acceptance.SanitizedHubSpotError(err))
	}
	if len(form.FieldGroups) != 1 || len(form.FieldGroups[0].Fields) != 1 {
		return errors.New("northstar form drift target has unsupported structure")
	}
	form.Name = "ns_contact_us_drift"
	form.FieldGroups[0].Fields[0].Label = "Out-of-band work email"
	form.Configuration.PostSubmitAction.Value = "Out-of-band Northstar thank you"
	form.DisplayOptions.SubmitButtonText = "Out-of-band contact"
	form, err = clients.Forms.Update(ctx, formID, hubspot.FormDefinitionPatch{
		Name: &form.Name, FieldGroups: &form.FieldGroups, Configuration: &form.Configuration, DisplayOptions: &form.DisplayOptions,
	})
	if err != nil {
		return fmt.Errorf("author Northstar form drift: %s", acceptance.SanitizedHubSpotError(err))
	}
	if len(form.FieldGroups) != 1 || len(form.FieldGroups[0].Fields) != 1 || form.ID != formID || form.Name != "ns_contact_us_drift" || form.FieldGroups[0].Fields[0].Label != "Out-of-band work email" {
		return errors.New("northstar form drift mutation was not observable")
	}
	return nil
}

func verifyNorthstarFormTerminal(ctx context.Context, clients *hubspot.ClientSet, formID string) (string, error) {
	if formID == "" {
		return "", errors.New("northstar form ID is required for terminal verification")
	}
	if _, err := clients.Forms.Get(ctx, formID); err == nil {
		return "", errors.New("northstar form remained active after teardown")
	} else {
		var apiError *hubspot.Error
		if !errors.As(err, &apiError) || apiError.Status != 404 {
			return "", fmt.Errorf("verify Northstar form active absence: %s", acceptance.SanitizedHubSpotError(err))
		}
	}
	archived, err := clients.Forms.GetArchived(ctx, formID)
	if err != nil {
		return "", fmt.Errorf("verify Northstar Archived form: %s", acceptance.SanitizedHubSpotError(err))
	}
	if archived.ID != formID || !archived.Archived {
		return "", errors.New("northstar terminal form identity was not exact")
	}
	active, err := clients.Forms.List(ctx, false)
	if err != nil {
		return "", fmt.Errorf("list active Northstar forms: %s", acceptance.SanitizedHubSpotError(err))
	}
	for _, form := range active {
		if strings.HasPrefix(form.Name, "ns_") {
			return "", errors.New("northstar teardown retained an active owned form")
		}
	}
	digest := sha256.Sum256([]byte("northstar-form-identity\x00" + formID))
	record, err := json.Marshal(struct {
		GeneratedIdentityHash string `json:"generated_identity_hash"`
		Terminal              string `json:"terminal"`
		ActiveOwnedForms      int    `json:"active_owned_forms"`
		Cleanup               string `json:"cleanup"`
	}{hex.EncodeToString(digest[:]), "archived", 0, "passed"})
	if err != nil {
		return "", errors.New("encode Northstar terminal form record")
	}
	return string(record), nil
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
