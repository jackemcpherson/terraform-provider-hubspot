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
	"regexp"
	"strings"
	"time"

	"github.com/jackemcpherson/terraform-provider-hubspot/internal/acceptance"
	"github.com/jackemcpherson/terraform-provider-hubspot/internal/hubspot"
)

var releasedPrefixPattern = regexp.MustCompile(`^tf_acc_[A-Za-z0-9_]+_$`)

func main() {
	if len(os.Args) != 4 {
		fatal(errors.New("usage: released-form-lifecycle verify-active|drift|archive|verify-terminal form-id acceptance-prefix"))
	}
	token := os.Getenv("HUBSPOT_ACCESS_TOKEN")
	if token == "" {
		fatal(errors.New("HUBSPOT_ACCESS_TOKEN is required"))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	clients, err := acceptance.NewRealPortalClientSet(ctx, token, "terraform-provider-hubspot/released-form-lifecycle")
	if err != nil {
		fatal(err)
	}
	record, err := execute(ctx, os.Args[1], os.Args[2], os.Args[3], clients)
	if err != nil {
		fatal(err)
	}
	if record != "" {
		fmt.Println(record)
	}
}

func execute(ctx context.Context, action, formID, prefix string, clients *hubspot.ClientSet) (string, error) {
	if !releasedPrefixPattern.MatchString(prefix) {
		return "", errors.New("unsafe released Form prefix")
	}
	if formID == "" {
		return "", errors.New("released Form ID is required")
	}
	switch action {
	case "verify-active":
		return "", verifyActive(ctx, clients, formID, prefix)
	case "drift":
		return "", drift(ctx, clients, formID, prefix)
	case "archive":
		return "", archive(ctx, clients, formID, prefix)
	case "verify-terminal":
		return verifyTerminal(ctx, clients, formID, prefix)
	default:
		return "", errors.New("action must be verify-active, drift, archive, or verify-terminal")
	}
}

func verifyActive(ctx context.Context, clients *hubspot.ClientSet, formID, prefix string) error {
	_, err := activeForm(ctx, clients, formID, prefix)
	return err
}

func activeForm(ctx context.Context, clients *hubspot.ClientSet, formID, prefix string) (hubspot.FormDefinition, error) {
	form, err := clients.Forms.Get(ctx, formID)
	if err != nil {
		return hubspot.FormDefinition{}, fmt.Errorf("read released Form identity: %s", acceptance.SanitizedHubSpotError(err))
	}
	if form.ID != formID || form.Name != prefix+"released_form" || form.Archived {
		return hubspot.FormDefinition{}, errors.New("released Form active identity was not exact")
	}
	forms, err := clients.Forms.List(ctx, false)
	if err != nil {
		return hubspot.FormDefinition{}, fmt.Errorf("list active released Forms: %s", acceptance.SanitizedHubSpotError(err))
	}
	owned := 0
	for _, candidate := range forms {
		if strings.HasPrefix(candidate.Name, prefix) {
			owned++
			if candidate.ID != formID {
				return hubspot.FormDefinition{}, errors.New("released Form journey created a second active identity")
			}
		}
	}
	if owned != 1 {
		return hubspot.FormDefinition{}, fmt.Errorf("released Form journey has %d active owned identities; want 1", owned)
	}
	return form, nil
}

func drift(ctx context.Context, clients *hubspot.ClientSet, formID, prefix string) error {
	form, err := activeForm(ctx, clients, formID, prefix)
	if err != nil {
		return err
	}
	if len(form.FieldGroups) != 1 || len(form.FieldGroups[0].Fields) != 1 {
		return errors.New("released Form drift target has unsupported structure")
	}
	form.FieldGroups[0].Fields[0].Label = "Out-of-band released work email"
	form.Configuration.PostSubmitAction.Value = "Out-of-band released thank you"
	form.DisplayOptions.SubmitButtonText = "Out-of-band released submit"
	updated, err := clients.Forms.Update(ctx, formID, hubspot.FormDefinitionPatch{
		FieldGroups: &form.FieldGroups, Configuration: &form.Configuration, DisplayOptions: &form.DisplayOptions,
	})
	if err != nil {
		return fmt.Errorf("author released Form drift: %s", acceptance.SanitizedHubSpotError(err))
	}
	if updated.ID != formID || updated.FieldGroups[0].Fields[0].Label != "Out-of-band released work email" {
		return errors.New("released Form drift mutation was not observable")
	}
	return verifyActive(ctx, clients, formID, prefix)
}

func archive(ctx context.Context, clients *hubspot.ClientSet, formID, prefix string) error {
	if err := verifyActive(ctx, clients, formID, prefix); err != nil {
		archived, archivedErr := clients.Forms.GetArchived(ctx, formID)
		if archivedErr == nil && archived.ID == formID && archived.Name == prefix+"released_form" && archived.Archived {
			return nil
		}
		return err
	}
	if err := clients.Forms.Archive(ctx, formID); err != nil {
		return fmt.Errorf("archive released Form identity: %s", acceptance.SanitizedHubSpotError(err))
	}
	return nil
}

func verifyTerminal(ctx context.Context, clients *hubspot.ClientSet, formID, prefix string) (string, error) {
	_, activeErr := clients.Forms.Get(ctx, formID)
	if activeErr == nil {
		return "", errors.New("released Form remained active after teardown")
	}
	var apiError *hubspot.Error
	if !errors.As(activeErr, &apiError) || apiError.Status != 404 {
		return "", fmt.Errorf("verify released Form active absence: %s", acceptance.SanitizedHubSpotError(activeErr))
	}
	archived, err := clients.Forms.GetArchived(ctx, formID)
	if err != nil {
		return "", fmt.Errorf("verify released Archived form definition: %s", acceptance.SanitizedHubSpotError(err))
	}
	if archived.ID != formID || archived.Name != prefix+"released_form" || !archived.Archived {
		return "", errors.New("released Form terminal identity was not exact")
	}
	active, err := clients.Forms.List(ctx, false)
	if err != nil {
		return "", fmt.Errorf("list active released Forms: %s", acceptance.SanitizedHubSpotError(err))
	}
	for _, form := range active {
		if strings.HasPrefix(form.Name, prefix) {
			return "", errors.New("released Form teardown retained an active owned identity")
		}
	}
	digest := sha256.Sum256([]byte("released-form-identity\x00" + formID))
	record, err := json.Marshal(struct {
		GeneratedIdentityHash string `json:"generated_identity_hash"`
		Terminal              string `json:"terminal"`
		ActiveOwnedForms      int    `json:"active_owned_forms"`
		Cleanup               string `json:"cleanup"`
	}{hex.EncodeToString(digest[:]), "archived", 0, "passed"})
	if err != nil {
		return "", errors.New("encode released Form terminal record")
	}
	return string(record), nil
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
