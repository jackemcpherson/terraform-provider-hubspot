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
	"sort"
	"strings"
	"time"

	"github.com/jackemcpherson/terraform-provider-hubspot/internal/acceptance"
	"github.com/jackemcpherson/terraform-provider-hubspot/internal/hubspot"
)

const driftLabel = "Out-of-band Northstar buyer role"

type northstarFilesIDs struct {
	BrandFolder     string
	DownloadsFolder string
	PrivateFile     string
	PublicFile      string
}

type northstarFilesNames struct {
	Prefix             string
	BrandFolder        string
	BrandFolderRefresh string
	DownloadsFolder    string
	PrivateFile        string
	PublicFile         string
	PublicFileDrift    string
}

func northstarFilesNamesFromEnvironment() (northstarFilesNames, error) {
	prefix := os.Getenv("HUBSPOT_NORTHSTAR_FILES_PREFIX")
	if prefix == "" {
		prefix = "ns_"
	}
	if !strings.HasPrefix(prefix, "ns_") || !strings.HasSuffix(prefix, "_") || len(prefix) > 100 {
		return northstarFilesNames{}, errors.New("HUBSPOT_NORTHSTAR_FILES_PREFIX must start with ns_, end with _, and be at most 100 characters")
	}
	for _, character := range prefix {
		if (character < 'a' || character > 'z') && (character < 'A' || character > 'Z') && (character < '0' || character > '9') && character != '_' {
			return northstarFilesNames{}, errors.New("HUBSPOT_NORTHSTAR_FILES_PREFIX may contain only letters, digits, and underscores")
		}
	}
	return northstarFilesNames{
		Prefix:             prefix,
		BrandFolder:        prefix + "brand",
		BrandFolderRefresh: prefix + "brand_refresh",
		DownloadsFolder:    prefix + "downloads",
		PrivateFile:        prefix + "private_readme.txt",
		PublicFile:         prefix + "public_logo.svg",
		PublicFileDrift:    prefix + "public_logo_drift.svg",
	}, nil
}

func newNorthstarFilesIDs(values []string) northstarFilesIDs {
	return northstarFilesIDs{
		BrandFolder: values[0], DownloadsFolder: values[1],
		PrivateFile: values[2], PublicFile: values[3],
	}
}

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
		names, err := northstarFilesNamesFromEnvironment()
		if err != nil {
			return "", err
		}
		return "", verifyNorthstarFiles(ctx, clients, newNorthstarFilesIDs(ids), names)
	case "drift-files":
		if len(ids) != 1 {
			return "", errors.New("one Northstar Managed file ID is required for drift")
		}
		names, err := northstarFilesNamesFromEnvironment()
		if err != nil {
			return "", err
		}
		return "", driftNorthstarFile(ctx, clients, ids[0], names)
	case "drift-folder-path":
		if len(ids) != 2 {
			return "", errors.New("northstar parent and child folder IDs are required for path drift")
		}
		names, err := northstarFilesNamesFromEnvironment()
		if err != nil {
			return "", err
		}
		return "", driftNorthstarFolderPath(ctx, clients, ids[0], ids[1], names)
	case "verify-files-terminal":
		if len(ids) != 4 {
			return "", errors.New("four Northstar Files generated IDs are required for terminal verification")
		}
		names, err := northstarFilesNamesFromEnvironment()
		if err != nil {
			return "", err
		}
		return verifyNorthstarFilesTerminal(ctx, clients, newNorthstarFilesIDs(ids), names)
	case "cleanup":
		if len(ids) != 0 {
			return "", errors.New("northstar cleanup does not accept generated IDs")
		}
		names, err := northstarFilesNamesFromEnvironment()
		if err != nil {
			return "", err
		}
		if err := cleanupNorthstar(ctx, clients, names); err != nil {
			return "", err
		}
		return "Northstar cleanup verified zero active owned configuration", nil
	default:
		return "", errors.New("unknown Northstar lifecycle action")
	}
}

var northstarCRMNames = map[string]struct {
	groups     map[string]struct{}
	properties map[string]struct{}
}{
	"contacts": {
		groups: stringSet("ns_customer_context"),
		properties: stringSet(
			"ns_buyer_role", "ns_onboarding_status", "ns_last_success_review",
		),
	},
	"companies": {
		groups:     stringSet("ns_account_profile"),
		properties: stringSet("ns_account_tier", "ns_renewal_date"),
	},
	"deals": {
		groups:     stringSet("ns_commercial_context"),
		properties: stringSet("ns_commercial_motion", "ns_implementation_risk"),
	},
	"tickets": {
		groups:     stringSet("ns_support_context"),
		properties: stringSet("ns_support_priority", "ns_support_summary", "ns_response_due_at"),
	},
}

type northstarCleanupPlan struct {
	properties map[string][]string
	groups     map[string][]string
	forms      []string
	files      []string
	folders    []hubspot.FileFolder
}

func stringSet(values ...string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}

func cleanupNorthstar(ctx context.Context, clients *hubspot.ClientSet, names northstarFilesNames) error {
	plan, err := inspectNorthstarCleanup(ctx, clients, names)
	if err != nil {
		return err
	}
	for _, id := range plan.files {
		if err := deleteNorthstarIdentity(ctx, clients.Files.Delete, func(ctx context.Context, id string) error {
			_, err := clients.Files.Get(ctx, id)
			return err
		}, id); err != nil {
			return fmt.Errorf("delete Northstar Managed file: %s", acceptance.SanitizedHubSpotError(err))
		}
	}
	sort.Slice(plan.folders, func(left, right int) bool {
		leftDepth := strings.Count(plan.folders[left].Path, "/")
		rightDepth := strings.Count(plan.folders[right].Path, "/")
		if leftDepth == rightDepth {
			return plan.folders[left].Path > plan.folders[right].Path
		}
		return leftDepth > rightDepth
	})
	for _, folder := range plan.folders {
		if err := deleteNorthstarIdentity(ctx, clients.FileFolders.Delete, func(ctx context.Context, id string) error {
			_, err := clients.FileFolders.Get(ctx, id)
			return err
		}, folder.ID); err != nil {
			return fmt.Errorf("delete Northstar File folder: %s", acceptance.SanitizedHubSpotError(err))
		}
	}
	for _, id := range plan.forms {
		if err := clients.Forms.Archive(ctx, id); err != nil && !northstarNotFound(err) {
			return fmt.Errorf("archive Northstar Form definition: %s", acceptance.SanitizedHubSpotError(err))
		}
	}
	for objectType, names := range plan.properties {
		for _, name := range names {
			if err := clients.Properties.Archive(ctx, objectType, name); err != nil && !northstarNotFound(err) {
				return fmt.Errorf("archive Northstar %s property: %s", objectType, acceptance.SanitizedHubSpotError(err))
			}
		}
	}
	for objectType, names := range plan.groups {
		for _, name := range names {
			if err := clients.PropertyGroups.Archive(ctx, objectType, name); err != nil && !northstarNotFound(err) {
				return fmt.Errorf("archive Northstar %s property group: %s", objectType, acceptance.SanitizedHubSpotError(err))
			}
		}
	}
	return verifyNorthstarCleanup(ctx, clients, names)
}

func inspectNorthstarCleanup(ctx context.Context, clients *hubspot.ClientSet, names northstarFilesNames) (northstarCleanupPlan, error) {
	plan := northstarCleanupPlan{properties: map[string][]string{}, groups: map[string][]string{}}
	for objectType, expected := range northstarCRMNames {
		properties, err := clients.Properties.List(ctx, objectType, false, "non_sensitive", "")
		if err != nil {
			return plan, fmt.Errorf("list Northstar %s properties: %s", objectType, acceptance.SanitizedHubSpotError(err))
		}
		for _, property := range properties {
			if !strings.HasPrefix(property.Name, "ns_") {
				continue
			}
			if _, ok := expected.properties[property.Name]; !ok {
				return plan, fmt.Errorf("refusing unexpected Northstar %s property %q", objectType, property.Name)
			}
			plan.properties[objectType] = append(plan.properties[objectType], property.Name)
		}
		groups, err := clients.PropertyGroups.List(ctx, objectType)
		if err != nil {
			return plan, fmt.Errorf("list Northstar %s property groups: %s", objectType, acceptance.SanitizedHubSpotError(err))
		}
		for _, group := range groups {
			if !strings.HasPrefix(group.Name, "ns_") {
				continue
			}
			if _, ok := expected.groups[group.Name]; !ok {
				return plan, fmt.Errorf("refusing unexpected Northstar %s property group %q", objectType, group.Name)
			}
			plan.groups[objectType] = append(plan.groups[objectType], group.Name)
		}
	}
	forms, err := clients.Forms.List(ctx, false)
	if err != nil {
		return plan, fmt.Errorf("list Northstar Form definitions: %s", acceptance.SanitizedHubSpotError(err))
	}
	for _, form := range forms {
		if !strings.HasPrefix(form.Name, "ns_") {
			continue
		}
		if form.Name != "ns_contact_us" && form.Name != "ns_contact_us_drift" {
			return plan, fmt.Errorf("refusing unexpected Northstar Form definition %q", form.Name)
		}
		plan.forms = append(plan.forms, form.ID)
	}
	files, err := clients.Files.Search(ctx, nil, "")
	if err != nil {
		return plan, fmt.Errorf("list Northstar Managed files: %s", acceptance.SanitizedHubSpotError(err))
	}
	folders, err := clients.FileFolders.Search(ctx, nil, "")
	if err != nil {
		return plan, fmt.Errorf("list Northstar File folders: %s", acceptance.SanitizedHubSpotError(err))
	}
	expectedFolderNames := stringSet(names.BrandFolder, names.BrandFolderRefresh, names.DownloadsFolder)
	expectedFileNames := stringSet(names.PrivateFile, names.PublicFile, names.PublicFileDrift)
	ownedFolderNames := map[string]string{}
	for _, folder := range folders {
		if folder.Archived || !strings.HasPrefix(folder.Name, "ns_") {
			continue
		}
		if _, ok := expectedFolderNames[folder.Name]; !ok {
			return plan, fmt.Errorf("refusing unexpected Northstar File folder %q", folder.Name)
		}
		plan.folders = append(plan.folders, folder)
		ownedFolderNames[folder.ID] = folder.Name
	}
	for _, folder := range plan.folders {
		switch folder.Name {
		case names.BrandFolder, names.BrandFolderRefresh:
			if folder.ParentFolderID != nil || folder.Path != "/"+folder.Name {
				return plan, errors.New("refusing Northstar brand folder with unexpected placement")
			}
		case names.DownloadsFolder:
			if folder.ParentFolderID == nil {
				return plan, errors.New("refusing Northstar downloads folder without its owned parent")
			}
			parentName, ok := ownedFolderNames[*folder.ParentFolderID]
			if !ok || (parentName != names.BrandFolder && parentName != names.BrandFolderRefresh) || folder.Path != "/"+parentName+"/"+folder.Name {
				return plan, errors.New("refusing Northstar downloads folder with unexpected placement")
			}
		}
	}
	for _, file := range files {
		if file.Archived {
			continue
		}
		folderName, inOwnedFolder := ownedFolderNames[file.FolderID]
		ownedName := strings.HasPrefix(file.Name, "ns_")
		if inOwnedFolder && !ownedName {
			return plan, errors.New("refusing Northstar File folder containing an unowned Managed file")
		}
		if !ownedName {
			continue
		}
		if _, ok := expectedFileNames[file.Name]; !ok {
			return plan, fmt.Errorf("refusing unexpected Northstar Managed file %q", file.Name)
		}
		expectedFolderName := names.DownloadsFolder
		if file.Name == names.PrivateFile {
			expectedFolderName = names.BrandFolder
			if folderName == names.BrandFolderRefresh {
				expectedFolderName = names.BrandFolderRefresh
			}
		}
		if !inOwnedFolder || folderName != expectedFolderName || !strings.HasSuffix(file.Path, "/"+file.Name) {
			return plan, errors.New("refusing Northstar Managed file with unexpected placement")
		}
		plan.files = append(plan.files, file.ID)
	}
	for _, folder := range folders {
		if folder.Archived || folder.ParentFolderID == nil {
			continue
		}
		if _, inOwnedFolder := ownedFolderNames[*folder.ParentFolderID]; inOwnedFolder {
			if _, expected := expectedFolderNames[folder.Name]; !expected {
				return plan, errors.New("refusing Northstar File folder containing an unowned child folder")
			}
		}
	}
	return plan, nil
}

func verifyNorthstarCleanup(ctx context.Context, clients *hubspot.ClientSet, names northstarFilesNames) error {
	plan, err := inspectNorthstarCleanup(ctx, clients, names)
	if err != nil {
		return err
	}
	for _, names := range plan.properties {
		if len(names) != 0 {
			return errors.New("northstar cleanup left active CRM properties")
		}
	}
	for _, names := range plan.groups {
		if len(names) != 0 {
			return errors.New("northstar cleanup left active CRM property groups")
		}
	}
	if len(plan.forms) != 0 || len(plan.files) != 0 || len(plan.folders) != 0 {
		return errors.New("northstar cleanup left active Forms or Files configuration")
	}
	return nil
}

func deleteNorthstarIdentity(ctx context.Context, deleteByID func(context.Context, string) error, readByID func(context.Context, string) error, id string) error {
	if err := deleteByID(ctx, id); err != nil && !northstarNotFound(err) {
		var apiError *hubspot.Error
		if !errors.As(err, &apiError) || !apiError.Ambiguous {
			return err
		}
	}
	for {
		err := readByID(ctx, id)
		if northstarNotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}
		timer := time.NewTimer(250 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return errors.New("northstar Files cleanup timed out")
		case <-timer.C:
		}
	}
}

func northstarNotFound(err error) bool {
	var apiError *hubspot.Error
	return errors.As(err, &apiError) && apiError.Status == 404
}

func verifyNorthstarFiles(ctx context.Context, clients *hubspot.ClientSet, ids northstarFilesIDs, names northstarFilesNames) error {
	brand, err := clients.FileFolders.Get(ctx, ids.BrandFolder)
	if err != nil {
		return fmt.Errorf("read Northstar brand folder: %s", acceptance.SanitizedHubSpotError(err))
	}
	downloads, err := clients.FileFolders.Get(ctx, ids.DownloadsFolder)
	if err != nil {
		return fmt.Errorf("read Northstar downloads folder: %s", acceptance.SanitizedHubSpotError(err))
	}
	privateFile, err := clients.Files.Get(ctx, ids.PrivateFile)
	if err != nil {
		return fmt.Errorf("read Northstar private file: %s", acceptance.SanitizedHubSpotError(err))
	}
	publicFile, err := clients.Files.Get(ctx, ids.PublicFile)
	if err != nil {
		return fmt.Errorf("read Northstar public file: %s", acceptance.SanitizedHubSpotError(err))
	}
	if brand.ID != ids.BrandFolder || brand.Name != names.BrandFolder || brand.ParentFolderID != nil || brand.Path != "/"+names.BrandFolder {
		return errors.New("northstar brand folder did not match canonical exact-ID state")
	}
	if downloads.ID != ids.DownloadsFolder || downloads.Name != names.DownloadsFolder || downloads.ParentFolderID == nil || *downloads.ParentFolderID != ids.BrandFolder || downloads.Path != "/"+names.BrandFolder+"/"+names.DownloadsFolder {
		return errors.New("northstar downloads folder did not match canonical exact-ID state")
	}
	if privateFile.ID != ids.PrivateFile || privateFile.Name != names.PrivateFile || privateFile.FolderID != ids.BrandFolder || privateFile.Access != "PRIVATE" || privateFile.FileMD5 != "6062568b21ab5f9deb2a2c2f25cfbc37" || privateFile.Size != 23 {
		return errors.New("northstar private file did not match canonical exact-ID state")
	}
	if publicFile.ID != ids.PublicFile || publicFile.Name != names.PublicFile || publicFile.FolderID != ids.DownloadsFolder || publicFile.Access != "PUBLIC_NOT_INDEXABLE" || publicFile.FileMD5 != "21ebff031bb7f11ce0a0ab78c4347832" || publicFile.Size != 88 {
		return errors.New("northstar public file did not match canonical exact-ID state")
	}
	return nil
}

func driftNorthstarFile(ctx context.Context, clients *hubspot.ClientSet, id string, names northstarFilesNames) error {
	current, err := clients.Files.Get(ctx, id)
	if err != nil {
		return fmt.Errorf("read Northstar file drift target: %s", acceptance.SanitizedHubSpotError(err))
	}
	name, access := names.PublicFileDrift, "PRIVATE"
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

func driftNorthstarFolderPath(ctx context.Context, clients *hubspot.ClientSet, parentID, childID string, names northstarFilesNames) error {
	parent, err := clients.FileFolders.Get(ctx, parentID)
	if err != nil {
		return fmt.Errorf("read Northstar folder path drift target: %s", acceptance.SanitizedHubSpotError(err))
	}
	task, err := clients.FileFolders.Update(ctx, parentID, hubspot.FileFolderWrite{Name: names.BrandFolderRefresh, ParentFolderID: parent.ParentFolderID})
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
	if child.ID != childID || child.ParentFolderID == nil || *child.ParentFolderID != parentID || child.Path != "/"+names.BrandFolderRefresh+"/"+names.DownloadsFolder {
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

func verifyNorthstarFilesTerminal(ctx context.Context, clients *hubspot.ClientSet, ids northstarFilesIDs, names northstarFilesNames) (string, error) {
	return acceptance.VerifyFilesTerminal(
		ctx,
		clients,
		[]string{ids.BrandFolder, ids.DownloadsFolder},
		[]string{ids.PrivateFile, ids.PublicFile},
		names.Prefix,
		"northstar-files-identities",
	)
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
