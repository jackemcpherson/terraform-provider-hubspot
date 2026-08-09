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
	if len(os.Args) < 2 {
		fatal(errors.New("usage: northstar-lifecycle <action> [generated-id ...]"))
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
	result, err := execute(ctx, os.Args[1], os.Args[2:], clients)
	if err != nil {
		fatal(err)
	}
	if result != "" {
		fmt.Println(result)
	}
}

func execute(ctx context.Context, action string, ids []string, clients *hubspot.ClientSet) (string, error) {
	switch action {
	case "drift":
		if len(ids) != 1 {
			return "", errors.New("northstar form ID is required for drift")
		}
		if err := driftNorthstarProperty(ctx, clients); err != nil {
			return "", err
		}
		if err := driftNorthstarForm(ctx, clients, ids[0]); err != nil {
			return "", err
		}
		return "", nil
	case "archive-for-refresh":
		if len(ids) != 0 {
			return "", errors.New("archive-for-refresh does not accept generated IDs")
		}
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
		if len(ids) != 1 {
			return "", errors.New("northstar form ID is required for terminal verification")
		}
		return verifyNorthstarFormTerminal(ctx, clients, ids[0])
	case "verify-files":
		if len(ids) != 4 {
			return "", errors.New("four Northstar Files generated IDs are required for verification")
		}
		return "", verifyNorthstarFiles(ctx, clients, ids)
	case "drift-files":
		if len(ids) != 1 {
			return "", errors.New("one Northstar Managed file ID is required for drift")
		}
		return "", driftNorthstarFile(ctx, clients, ids[0])
	case "drift-folder-path":
		if len(ids) != 2 {
			return "", errors.New("northstar parent and child folder IDs are required for path drift")
		}
		return "", driftNorthstarFolderPath(ctx, clients, ids[0], ids[1])
	case "verify-files-terminal":
		if len(ids) != 4 {
			return "", errors.New("four Northstar Files generated IDs are required for terminal verification")
		}
		return verifyNorthstarFilesTerminal(ctx, clients, ids)
	default:
		return "", errors.New("unknown Northstar lifecycle action")
	}
}

func verifyNorthstarFiles(ctx context.Context, clients *hubspot.ClientSet, ids []string) error {
	brand, err := clients.FileFolders.Get(ctx, ids[0])
	if err != nil {
		return fmt.Errorf("read Northstar brand folder: %s", acceptance.SanitizedHubSpotError(err))
	}
	downloads, err := clients.FileFolders.Get(ctx, ids[1])
	if err != nil {
		return fmt.Errorf("read Northstar downloads folder: %s", acceptance.SanitizedHubSpotError(err))
	}
	privateFile, err := clients.Files.Get(ctx, ids[2])
	if err != nil {
		return fmt.Errorf("read Northstar private file: %s", acceptance.SanitizedHubSpotError(err))
	}
	publicFile, err := clients.Files.Get(ctx, ids[3])
	if err != nil {
		return fmt.Errorf("read Northstar public file: %s", acceptance.SanitizedHubSpotError(err))
	}
	if brand.ID != ids[0] || brand.Name != "ns_brand" || brand.ParentFolderID != nil || brand.Path != "/ns_brand" {
		return errors.New("northstar brand folder did not match canonical exact-ID state")
	}
	if downloads.ID != ids[1] || downloads.Name != "ns_downloads" || downloads.ParentFolderID == nil || *downloads.ParentFolderID != ids[0] || downloads.Path != "/ns_brand/ns_downloads" {
		return errors.New("northstar downloads folder did not match canonical exact-ID state")
	}
	if privateFile.ID != ids[2] || privateFile.Name != "ns_private_readme.txt" || privateFile.FolderID != ids[0] || privateFile.Access != "PRIVATE" || privateFile.FileMD5 != "6062568b21ab5f9deb2a2c2f25cfbc37" || privateFile.Size != 23 {
		return errors.New("northstar private file did not match canonical exact-ID state")
	}
	if publicFile.ID != ids[3] || publicFile.Name != "ns_public_logo.svg" || publicFile.FolderID != ids[1] || publicFile.Access != "PUBLIC_NOT_INDEXABLE" || publicFile.FileMD5 != "21ebff031bb7f11ce0a0ab78c4347832" || publicFile.Size != 88 {
		return errors.New("northstar public file did not match canonical exact-ID state")
	}
	return nil
}

func driftNorthstarFile(ctx context.Context, clients *hubspot.ClientSet, id string) error {
	current, err := clients.Files.Get(ctx, id)
	if err != nil {
		return fmt.Errorf("read Northstar file drift target: %s", acceptance.SanitizedHubSpotError(err))
	}
	name, access := "ns_public_logo_drift.svg", "PRIVATE"
	updated, err := clients.Files.Update(ctx, id, hubspot.FilePatch{Name: &name, Access: &access})
	if err != nil {
		return fmt.Errorf("author Northstar file metadata drift: %s", acceptance.SanitizedHubSpotError(err))
	}
	updated, err = clients.Files.Replace(ctx, id, hubspot.FileReplacement{Name: updated.Name, Access: updated.Access, Bytes: []byte("out-of-band Northstar content\n")})
	if err != nil {
		return fmt.Errorf("author Northstar file content drift: %s", acceptance.SanitizedHubSpotError(err))
	}
	if updated.ID != id || updated.Name != name || updated.Access != access || updated.FileMD5 == current.FileMD5 || updated.CreatedAt != current.CreatedAt {
		return errors.New("northstar file drift mutation was not observable with preserved identity")
	}
	return nil
}

func driftNorthstarFolderPath(ctx context.Context, clients *hubspot.ClientSet, parentID, childID string) error {
	parent, err := clients.FileFolders.Get(ctx, parentID)
	if err != nil {
		return fmt.Errorf("read Northstar folder path drift target: %s", acceptance.SanitizedHubSpotError(err))
	}
	task, err := clients.FileFolders.Update(ctx, parentID, hubspot.FileFolderWrite{Name: "ns_brand_refresh", ParentFolderID: parent.ParentFolderID})
	if err != nil {
		return fmt.Errorf("author Northstar folder path drift: %s", acceptance.SanitizedHubSpotError(err))
	}
	if err := waitForFolderTask(ctx, clients, task.ID); err != nil {
		return err
	}
	child, err := clients.FileFolders.Get(ctx, childID)
	if err != nil {
		return fmt.Errorf("read Northstar refreshed child folder: %s", acceptance.SanitizedHubSpotError(err))
	}
	if child.ID != childID || child.ParentFolderID == nil || *child.ParentFolderID != parentID || child.Path != "/ns_brand_refresh/ns_downloads" {
		return errors.New("northstar child folder path drift was not observable with preserved identity")
	}
	return nil
}

func waitForFolderTask(ctx context.Context, clients *hubspot.ClientSet, id string) error {
	for {
		task, err := clients.FileFolders.GetUpdateTask(ctx, id)
		if err != nil {
			return fmt.Errorf("read Northstar folder update task: %s", acceptance.SanitizedHubSpotError(err))
		}
		if len(task.Errors) > 0 {
			return errors.New("northstar folder path drift task reported errors")
		}
		switch task.Status {
		case "COMPLETE":
			return nil
		case "PENDING", "RUNNING", "PROCESSING":
		default:
			return errors.New("northstar folder path drift task did not complete")
		}
		delay := task.RetryAfter
		if delay <= 0 {
			delay = 250 * time.Millisecond
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return errors.New("northstar folder path drift task timed out")
		case <-timer.C:
		}
	}
}

func verifyNorthstarFilesTerminal(ctx context.Context, clients *hubspot.ClientSet, ids []string) (string, error) {
	for index, id := range ids {
		var err error
		if index < 2 {
			_, err = clients.FileFolders.Get(ctx, id)
		} else {
			_, err = clients.Files.Get(ctx, id)
		}
		var apiError *hubspot.Error
		if !errors.As(err, &apiError) || apiError.Status != 404 {
			return "", errors.New("northstar Files identity remained active after teardown")
		}
	}
	folders, err := clients.FileFolders.Search(ctx, nil, "")
	if err != nil {
		return "", fmt.Errorf("list active Northstar folders: %s", acceptance.SanitizedHubSpotError(err))
	}
	files, err := clients.Files.Search(ctx, nil, "")
	if err != nil {
		return "", fmt.Errorf("list active Northstar files: %s", acceptance.SanitizedHubSpotError(err))
	}
	activeFolders, activeFiles := 0, 0
	for _, folder := range folders {
		if strings.HasPrefix(folder.Name, "ns_") {
			activeFolders++
		}
	}
	for _, file := range files {
		if strings.HasPrefix(file.Name, "ns_") {
			activeFiles++
		}
	}
	if activeFolders != 0 || activeFiles != 0 {
		return "", errors.New("northstar teardown retained active owned Files configuration")
	}
	digest := sha256.Sum256([]byte("northstar-files-identities\x00" + strings.Join(ids, "\x00")))
	record, err := json.Marshal(struct {
		GeneratedIdentityHash string `json:"generated_identity_hash"`
		ActiveOwnedFiles      int    `json:"active_owned_files"`
		ActiveOwnedFolders    int    `json:"active_owned_folders"`
		Cleanup               string `json:"cleanup"`
	}{hex.EncodeToString(digest[:]), activeFiles, activeFolders, "passed"})
	if err != nil {
		return "", errors.New("encode Northstar Files terminal record")
	}
	return string(record), nil
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
