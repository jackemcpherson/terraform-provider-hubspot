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
	"math/big"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/jackemcpherson/terraform-provider-hubspot/internal/acceptance"
	"github.com/jackemcpherson/terraform-provider-hubspot/internal/hubspot"
)

const driftLabel = "Out-of-band Northstar buyer role"

const (
	northstarProfileJobTitle         = "Cloud Operations Engineer"
	northstarProfileAvailability     = "available"
	northstarProfileTimeZone         = "Australia/Melbourne"
	northstarProductSKU              = "ns_support_annual"
	northstarProductName             = "Northstar annual support"
	northstarProductDescription      = "Priority support for Northstar customers"
	northstarProductPrice            = "1200.00"
	northstarProductCost             = "300.00"
	northstarProductRecurrence       = "P12M"
	northstarApprovedMembershipEmail = "tfhs-probe-16-20260802024807@example.com"
)

var (
	northstarFilesConvergenceDelays          = []time.Duration{0, time.Second, 2 * time.Second, 3 * time.Second, 5 * time.Second, 8 * time.Second, 13 * time.Second}
	northstarFolderTaskConvergenceDelays     = []time.Duration{0, time.Second, 2 * time.Second, 3 * time.Second, 5 * time.Second, 8 * time.Second, 13 * time.Second}
	northstarDescendantPathConvergenceDelays = []time.Duration{0, time.Second, 2 * time.Second, 3 * time.Second, 5 * time.Second, 8 * time.Second, 13 * time.Second, 21 * time.Second, 34 * time.Second, 55 * time.Second}
)

var errNorthstarFolderReadBack = errors.New("northstar folder read-back did not converge")

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
	if !strings.HasPrefix(prefix, "ns_") || !strings.HasSuffix(prefix, "_") || (prefix != "ns_" && len(prefix) > 14) {
		return northstarFilesNames{}, errors.New("HUBSPOT_NORTHSTAR_FILES_PREFIX must be ns_ or a run prefix of at most 14 characters")
	}
	for _, character := range prefix {
		if (character < 'a' || character > 'z') && (character < 'A' || character > 'Z') && (character < '0' || character > '9') && character != '_' {
			return northstarFilesNames{}, errors.New("HUBSPOT_NORTHSTAR_FILES_PREFIX may contain only letters, digits, and underscores")
		}
	}
	names := northstarFilesNames{
		Prefix:             prefix,
		BrandFolder:        prefix + "brand",
		BrandFolderRefresh: prefix + "brand_refresh",
		DownloadsFolder:    prefix + "downloads",
		PrivateFile:        prefix + "private_readme.txt",
		PublicFile:         prefix + "public_logo.svg",
		PublicFileDrift:    prefix + "public_logo_drift.svg",
	}
	if prefix != "ns_" {
		names.BrandFolder = prefix + "b"
		names.BrandFolderRefresh = prefix + "br"
		names.DownloadsFolder = prefix + "d"
		names.PrivateFile = prefix + "p.txt"
		names.PublicFile = prefix + "l.svg"
		names.PublicFileDrift = prefix + "x.svg"
	}
	return names, nil
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
	ctx, cancel := context.WithTimeout(context.Background(), northstarActionTimeout(os.Args[1]))
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

func northstarActionTimeout(action string) time.Duration {
	switch action {
	case "drift-folder-path", "repair-folder-path":
		return 4 * time.Minute
	case "verify-files":
		return 3 * time.Minute
	default:
		return 2 * time.Minute
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
	case "verify-membership":
		if len(ids) != 2 {
			return "", errors.New("northstar membership Settings ID and email are required for verification")
		}
		return "", verifyNorthstarMembership(ctx, clients, ids[0], ids[1])
	case "verify-membership-terminal":
		if len(ids) != 2 {
			return "", errors.New("northstar membership Settings ID and email are required for terminal verification")
		}
		return verifyNorthstarMembershipTerminal(ctx, clients, ids[0], ids[1])
	case "verify-profile":
		if len(ids) != 2 {
			return "", errors.New("northstar CRM profile ID and membership Settings ID are required for verification")
		}
		return "", verifyNorthstarCRMProfile(ctx, clients, ids[0], ids[1])
	case "drift-profile":
		if len(ids) != 2 {
			return "", errors.New("northstar CRM profile ID and membership Settings ID are required for drift")
		}
		return "", driftNorthstarCRMProfile(ctx, clients, ids[0], ids[1])
	case "verify-profile-terminal":
		if len(ids) != 2 {
			return "", errors.New("northstar CRM profile ID and membership Settings ID are required for terminal verification")
		}
		return verifyNorthstarCRMProfileTerminal(ctx, clients, ids[0], ids[1])
	case "verify-product":
		if len(ids) != 1 {
			return "", errors.New("northstar Product ID is required for verification")
		}
		return "", verifyNorthstarProduct(ctx, clients, ids[0])
	case "drift-product":
		if len(ids) != 1 {
			return "", errors.New("northstar Product ID is required for drift")
		}
		return "", driftNorthstarProduct(ctx, clients, ids[0])
	case "archive-product-for-refresh":
		if len(ids) != 1 {
			return "", errors.New("northstar Product ID is required for refresh archival")
		}
		return "", archiveNorthstarProductForRefresh(ctx, clients, ids[0])
	case "verify-product-terminal":
		if len(ids) != 1 {
			return "", errors.New("northstar Product ID is required for terminal verification")
		}
		return verifyNorthstarProductTerminal(ctx, clients, ids[0])
	case "verify-files":
		if len(ids) != 4 {
			return "", errors.New("four Northstar Files generated IDs are required for verification")
		}
		names, err := northstarFilesNamesFromEnvironment()
		if err != nil {
			return "", err
		}
		return "", verifyNorthstarFiles(ctx, clients, newNorthstarFilesIDs(ids), names, os.Getenv("HUBSPOT_NORTHSTAR_FILES_STAGED") == "1")
	case "stage-file-for-folder-rename":
		if len(ids) != 3 {
			return "", errors.New("northstar private file, parent folder, and staging folder IDs are required")
		}
		names, err := northstarFilesNamesFromEnvironment()
		if err != nil {
			return "", err
		}
		return "", stageNorthstarFileForFolderRename(ctx, clients, ids[0], ids[1], ids[2], names)
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
		if os.Getenv("HUBSPOT_NORTHSTAR_FILE_REFRESH_DRIFT") == "1" {
			fileID := os.Getenv("HUBSPOT_NORTHSTAR_PRIVATE_FILE_ID")
			if fileID == "" {
				return "", errors.New("HUBSPOT_NORTHSTAR_PRIVATE_FILE_ID is required for staged file refresh drift")
			}
			return "", verifyNorthstarStagedFileDrift(ctx, clients, ids[0], ids[1], fileID, names)
		}
		return "", driftNorthstarFolderPath(ctx, clients, ids[0], ids[1], names)
	case "repair-folder-path":
		if len(ids) != 2 {
			return "", errors.New("northstar parent and child folder IDs are required for path repair")
		}
		names, err := northstarFilesNamesFromEnvironment()
		if err != nil {
			return "", err
		}
		return "", repairNorthstarDescendantPath(ctx, clients, ids[0], ids[1], names)
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
		membershipEmail, err := northstarMembershipEmailFromEnvironment()
		if err != nil {
			return "", err
		}
		if err := cleanupNorthstar(ctx, clients, names, membershipEmail); err != nil {
			return "", err
		}
		return "Northstar cleanup verified zero active owned configuration", nil
	default:
		return "", errors.New("unknown Northstar lifecycle action")
	}
}

func northstarMembershipEmailFromEnvironment() (string, error) {
	email := os.Getenv("HUBSPOT_NORTHSTAR_MEMBERSHIP_EMAIL")
	if email != northstarApprovedMembershipEmail {
		return "", errors.New("HUBSPOT_NORTHSTAR_MEMBERSHIP_EMAIL must be the exact approved reserved Northstar fixture")
	}
	return email, nil
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
	products   []string
	membership *hubspot.AccountMembership
}

func stringSet(values ...string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}

func cleanupNorthstar(ctx context.Context, clients *hubspot.ClientSet, names northstarFilesNames, membershipEmail string) error {
	plan, err := inspectNorthstarCleanup(ctx, clients, names, membershipEmail)
	if err != nil {
		return err
	}
	if plan.membership != nil {
		if err := deleteNorthstarMembership(ctx, clients, *plan.membership); err != nil {
			return err
		}
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
	for _, id := range plan.products {
		if err := clients.Products.Archive(ctx, id); err != nil && !northstarNotFound(err) {
			return fmt.Errorf("archive Northstar Product definition: %s", acceptance.SanitizedHubSpotError(err))
		}
		archived, err := clients.Products.GetArchived(ctx, id)
		if err != nil || archived.ID != id || !archived.Archived {
			return errors.New("northstar Product cleanup did not verify the exact archived identity")
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
	return verifyNorthstarCleanup(ctx, clients, names, membershipEmail)
}

func inspectNorthstarCleanup(ctx context.Context, clients *hubspot.ClientSet, names northstarFilesNames, membershipEmail string) (northstarCleanupPlan, error) {
	plan := northstarCleanupPlan{properties: map[string][]string{}, groups: map[string][]string{}}
	memberships, err := clients.AccountMemberships.List(ctx)
	if err != nil {
		return plan, fmt.Errorf("list Northstar account memberships: %s", acceptance.SanitizedHubSpotError(err))
	}
	for _, membership := range memberships {
		if membership.Email != membershipEmail {
			continue
		}
		if plan.membership != nil || membership.SuperAdmin || membership.HasRoleOrTeamAssignments() {
			return plan, errors.New("refusing unsafe Northstar account membership")
		}
		owned := membership
		plan.membership = &owned
	}
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
	products, err := clients.Products.List(ctx)
	if err != nil {
		return plan, fmt.Errorf("list Northstar Product definitions: %s", acceptance.SanitizedHubSpotError(err))
	}
	for _, product := range products {
		if !strings.HasPrefix(product.SKU, "ns_") {
			continue
		}
		if product.SKU != northstarProductSKU || product.ID == "" || product.Archived {
			return plan, fmt.Errorf("refusing unexpected Northstar Product definition %q", product.SKU)
		}
		exact, err := clients.Products.Get(ctx, product.ID)
		if err != nil || exact.ID != product.ID || exact.SKU != product.SKU || exact.Archived {
			return plan, errors.New("refusing Northstar Product whose exact identity was not verified")
		}
		plan.products = append(plan.products, product.ID)
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
			if folderName == names.DownloadsFolder {
				expectedFolderName = names.DownloadsFolder
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

func verifyNorthstarCleanup(ctx context.Context, clients *hubspot.ClientSet, names northstarFilesNames, membershipEmail string) error {
	plan, err := inspectNorthstarCleanup(ctx, clients, names, membershipEmail)
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
	if len(plan.forms) != 0 || len(plan.files) != 0 || len(plan.folders) != 0 || len(plan.products) != 0 {
		return errors.New("northstar cleanup left active Forms, Files, or Product configuration")
	}
	if plan.membership != nil {
		return errors.New("northstar cleanup left the disposable account membership active")
	}
	return nil
}

func deleteNorthstarMembership(ctx context.Context, clients *hubspot.ClientSet, membership hubspot.AccountMembership) error {
	current, err := clients.AccountMemberships.GetByID(ctx, membership.ID)
	if northstarNotFound(err) {
		_, emailErr := clients.AccountMemberships.GetByEmail(ctx, membership.Email)
		if !northstarNotFound(emailErr) {
			if emailErr != nil {
				return fmt.Errorf("read absent Northstar account membership by email: %s", acceptance.SanitizedHubSpotError(emailErr))
			}
			return errors.New("refusing Northstar account membership whose email resolved after ID absence")
		}
		if err := clients.AccountMemberships.WaitForAbsence(ctx, membership.ID, membership.Email); err != nil {
			return fmt.Errorf("verify absent Northstar account membership cleanup: %s", acceptance.SanitizedHubSpotError(err))
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("read Northstar account membership: %s", acceptance.SanitizedHubSpotError(err))
	}
	byEmail, err := clients.AccountMemberships.GetByEmail(ctx, membership.Email)
	if err != nil {
		return fmt.Errorf("read Northstar account membership by email: %s", acceptance.SanitizedHubSpotError(err))
	}
	if current.ID != membership.ID || current.Email != membership.Email ||
		byEmail.ID != membership.ID || byEmail.Email != membership.Email ||
		current.SuperAdmin || byEmail.SuperAdmin || current.HasRoleOrTeamAssignments() || byEmail.HasRoleOrTeamAssignments() {
		return errors.New("refusing Northstar account membership whose exact safe identity was not verified")
	}
	deleteErr := clients.AccountMemberships.Delete(ctx, membership.ID)
	if deleteErr != nil && !northstarNotFound(deleteErr) && !northstarAmbiguous(deleteErr) {
		return fmt.Errorf("delete Northstar account membership: %s", acceptance.SanitizedHubSpotError(deleteErr))
	}
	if err := clients.AccountMemberships.WaitForAbsence(ctx, membership.ID, membership.Email); err != nil {
		return fmt.Errorf("verify Northstar account membership cleanup: %s", acceptance.SanitizedHubSpotError(err))
	}
	return nil
}

func northstarAmbiguous(err error) bool {
	var apiError *hubspot.Error
	return errors.As(err, &apiError) && apiError.Ambiguous
}

func deleteNorthstarIdentity(ctx context.Context, deleteByID func(context.Context, string) error, readByID func(context.Context, string) error, id string) error {
	if err := deleteByID(ctx, id); err != nil && !northstarNotFound(err) {
		if !northstarAmbiguous(err) {
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

func verifyNorthstarFiles(ctx context.Context, clients *hubspot.ClientSet, ids northstarFilesIDs, names northstarFilesNames, staged bool) error {
	brand, err := clients.FileFolders.Get(ctx, ids.BrandFolder)
	if err != nil {
		return fmt.Errorf("read Northstar brand folder: %s", acceptance.SanitizedHubSpotError(err))
	}
	expectedDownloadsPath := "/" + names.BrandFolder + "/" + names.DownloadsFolder
	downloads, err := waitForNorthstarDescendantPath(ctx, clients.FileFolders.Get, clients.FileFolders.Search, ids.BrandFolder, ids.DownloadsFolder, func(folder hubspot.FileFolder) bool {
		return folder.ID == ids.DownloadsFolder && folder.Name == names.DownloadsFolder && folder.ParentFolderID != nil && *folder.ParentFolderID == ids.BrandFolder && folder.Path == expectedDownloadsPath
	})
	if err != nil {
		if errors.Is(err, errNorthstarFolderReadBack) {
			return errors.New("northstar downloads folder descendant path did not converge")
		}
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
	if downloads.ID != ids.DownloadsFolder || downloads.Name != names.DownloadsFolder || downloads.ParentFolderID == nil || *downloads.ParentFolderID != ids.BrandFolder || downloads.Path != expectedDownloadsPath {
		return errors.New("northstar downloads folder did not match canonical exact-ID state")
	}
	privateFolderID := ids.BrandFolder
	if staged {
		privateFolderID = ids.DownloadsFolder
	}
	if privateFile.ID != ids.PrivateFile || privateFile.Name != names.PrivateFile || privateFile.FolderID != privateFolderID || privateFile.Access != "PRIVATE" || privateFile.FileMD5 != "6062568b21ab5f9deb2a2c2f25cfbc37" || privateFile.Size != 23 {
		return errors.New("northstar private file did not match canonical exact-ID state")
	}
	if publicFile.ID != ids.PublicFile || publicFile.Name != names.PublicFile || publicFile.FolderID != ids.DownloadsFolder || publicFile.Access != "PUBLIC_NOT_INDEXABLE" || publicFile.FileMD5 != "21ebff031bb7f11ce0a0ab78c4347832" || publicFile.Size != 88 {
		return errors.New("northstar public file did not match canonical exact-ID state")
	}
	return nil
}

func waitForNorthstarDescendantPath(
	ctx context.Context,
	read func(context.Context, string) (hubspot.FileFolder, error),
	search func(context.Context, *string, string) ([]hubspot.FileFolder, error),
	parentID string,
	id string,
	matches func(hubspot.FileFolder) bool,
) (hubspot.FileFolder, error) {
	var observed hubspot.FileFolder
	for _, delay := range northstarDescendantPathConvergenceDelays {
		if delay > 0 {
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return observed, errors.New("northstar folder read-back timed out")
			case <-timer.C:
			}
		}
		folder, err := read(ctx, id)
		if err != nil {
			return folder, err
		}
		if folder.ID != id {
			return folder, errors.New("northstar folder read-back identity mismatch")
		}
		observed = folder
		if matches(folder) {
			return folder, nil
		}
		folders, err := search(ctx, &parentID, "")
		if err != nil {
			return observed, err
		}
		for _, candidate := range folders {
			if candidate.ID != id {
				continue
			}
			observed = candidate
			if matches(candidate) {
				return candidate, nil
			}
		}
	}
	return observed, errNorthstarFolderReadBack
}

func waitForNorthstarFolderTask(ctx context.Context, read func(context.Context, string) (hubspot.FolderUpdateTask, error), id string, delays []time.Duration) error {
	for _, delay := range delays {
		if delay > 0 {
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return errors.New("northstar folder update task timed out")
			case <-timer.C:
			}
		}
		task, err := read(ctx, id)
		if err != nil {
			return err
		}
		if len(task.Errors) > 0 {
			return errors.New("northstar folder update task result was invalid")
		}
		switch task.Status {
		case "COMPLETE":
			return nil
		case "PENDING", "RUNNING", "PROCESSING":
		case "CANCELED", "ERROR", "FAILED", "":
			return errors.New("northstar folder update task did not complete")
		default:
			return errors.New("northstar folder update task returned an unknown state")
		}
	}
	return errors.New("northstar folder update task did not converge")
}

func stageNorthstarFileForFolderRename(ctx context.Context, clients *hubspot.ClientSet, fileID, parentFolderID, stagingFolderID string, names northstarFilesNames) error {
	file, err := clients.Files.Get(ctx, fileID)
	if err != nil {
		return fmt.Errorf("read Northstar folder-rename staging file: %s", acceptance.SanitizedHubSpotError(err))
	}
	parentFolder, err := clients.FileFolders.Get(ctx, parentFolderID)
	if err != nil {
		return fmt.Errorf("read Northstar folder-rename parent folder: %s", acceptance.SanitizedHubSpotError(err))
	}
	stagingFolder, err := clients.FileFolders.Get(ctx, stagingFolderID)
	if err != nil {
		return fmt.Errorf("read Northstar folder-rename staging folder: %s", acceptance.SanitizedHubSpotError(err))
	}
	if file.Name != names.PrivateFile || file.FolderID != parentFolderID || parentFolder.Name != names.BrandFolder || stagingFolder.Name != names.DownloadsFolder || stagingFolder.ParentFolderID == nil || *stagingFolder.ParentFolderID != parentFolderID {
		return errors.New("northstar folder-rename staging identities did not match the owned configuration")
	}
	updated, err := clients.Files.Update(ctx, fileID, hubspot.FilePatch{FolderID: &stagingFolderID})
	if err != nil {
		return fmt.Errorf("stage Northstar file before folder rename: %s", acceptance.SanitizedHubSpotError(err))
	}
	if updated.ID != fileID || updated.FolderID != stagingFolderID || updated.Name != names.PrivateFile {
		return errors.New("northstar folder-rename staging move did not preserve the exact file identity")
	}
	if err := waitForNorthstarFileStaging(ctx, clients, fileID, parentFolderID, stagingFolderID, names, northstarFilesConvergenceDelays); err != nil {
		return fmt.Errorf("verify Northstar file staging before folder rename: %s", acceptance.SanitizedHubSpotError(err))
	}
	return nil
}

func waitForNorthstarFileStaging(ctx context.Context, clients *hubspot.ClientSet, fileID, parentFolderID, stagingFolderID string, names northstarFilesNames, delays []time.Duration) error {
	for _, delay := range delays {
		if delay > 0 {
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return errors.New("northstar staged file read-back timed out")
			case <-timer.C:
			}
		}
		file, err := clients.Files.Get(ctx, fileID)
		if err != nil {
			return err
		}
		if file.ID != fileID || file.Name != names.PrivateFile || file.FolderID != stagingFolderID {
			continue
		}
		directFiles, err := clients.Files.Search(ctx, &parentFolderID, "")
		if err != nil {
			return err
		}
		if len(directFiles) == 0 {
			return nil
		}
	}
	return errors.New("northstar staged file and empty parent folder did not converge")
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
	task, err := clients.FileFolders.Update(ctx, parentID, hubspot.FileFolderWrite{Name: names.BrandFolderRefresh})
	if err != nil {
		return fmt.Errorf("author Northstar folder path drift: %s", acceptance.SanitizedHubSpotError(err))
	}
	if err := waitForNorthstarFolderTask(ctx, clients.FileFolders.GetUpdateTask, task.ID, northstarFolderTaskConvergenceDelays); err != nil {
		return fmt.Errorf("author Northstar folder path drift: %s", acceptance.SanitizedHubSpotError(err))
	}
	expectedPath := "/" + names.BrandFolderRefresh + "/" + names.DownloadsFolder
	child, err := waitForNorthstarDescendantPath(ctx, clients.FileFolders.Get, clients.FileFolders.Search, parentID, childID, func(folder hubspot.FileFolder) bool {
		return folder.ID == childID && folder.ParentFolderID != nil && *folder.ParentFolderID == parentID && folder.Path == expectedPath
	})
	if err != nil {
		if errors.Is(err, errNorthstarFolderReadBack) {
			return errors.New("northstar refreshed child folder descendant path did not converge")
		}
		return fmt.Errorf("read Northstar refreshed child folder: %s", acceptance.SanitizedHubSpotError(err))
	}
	if child.ID != childID || child.ParentFolderID == nil || *child.ParentFolderID != parentID || child.Path != expectedPath {
		return errors.New("northstar child folder path drift was not observable with preserved identity")
	}
	return nil
}

func repairNorthstarDescendantPath(ctx context.Context, clients *hubspot.ClientSet, parentID, childID string, names northstarFilesNames) error {
	parent, err := clients.FileFolders.Get(ctx, parentID)
	if err != nil {
		return fmt.Errorf("read Northstar folder-repair parent: %s", acceptance.SanitizedHubSpotError(err))
	}
	child, err := clients.FileFolders.Get(ctx, childID)
	if err != nil {
		return fmt.Errorf("read Northstar folder-repair child: %s", acceptance.SanitizedHubSpotError(err))
	}
	if parent.ID != parentID || parent.Name != names.BrandFolder || parent.ParentFolderID != nil || child.ID != childID || child.Name != names.DownloadsFolder || child.ParentFolderID == nil || *child.ParentFolderID != parentID {
		return errors.New("northstar folder-repair identities did not match the exact owned configuration")
	}
	children, err := clients.FileFolders.Search(ctx, &parentID, "")
	if err != nil {
		return fmt.Errorf("search Northstar folder-repair parent: %s", acceptance.SanitizedHubSpotError(err))
	}
	found := false
	for _, candidate := range children {
		if candidate.ID == childID && candidate.Name == names.DownloadsFolder && candidate.ParentFolderID != nil && *candidate.ParentFolderID == parentID {
			found = true
			break
		}
	}
	if !found {
		return errors.New("northstar folder-repair identities did not match the exact owned configuration")
	}
	task, err := clients.FileFolders.Update(ctx, childID, hubspot.FileFolderWrite{Name: names.DownloadsFolder, ParentFolderID: &parentID})
	if err != nil {
		return fmt.Errorf("repair Northstar descendant folder path: %s", acceptance.SanitizedHubSpotError(err))
	}
	if err := waitForNorthstarFolderTask(ctx, clients.FileFolders.GetUpdateTask, task.ID, northstarFolderTaskConvergenceDelays); err != nil {
		return fmt.Errorf("repair Northstar descendant folder path: %s", acceptance.SanitizedHubSpotError(err))
	}
	expectedPath := "/" + names.BrandFolder + "/" + names.DownloadsFolder
	repaired, err := waitForNorthstarDescendantPath(ctx, clients.FileFolders.Get, clients.FileFolders.Search, parentID, childID, func(folder hubspot.FileFolder) bool {
		return folder.ID == childID && folder.Name == names.DownloadsFolder && folder.ParentFolderID != nil && *folder.ParentFolderID == parentID && folder.Path == expectedPath
	})
	if err != nil {
		if errors.Is(err, errNorthstarFolderReadBack) {
			return errors.New("northstar repaired child folder descendant path did not converge")
		}
		return fmt.Errorf("read Northstar repaired child folder: %s", acceptance.SanitizedHubSpotError(err))
	}
	if repaired.ID != childID || repaired.Path != expectedPath {
		return errors.New("northstar repaired child folder did not match canonical exact-ID state")
	}
	return nil
}

func verifyNorthstarStagedFileDrift(ctx context.Context, clients *hubspot.ClientSet, parentID, childID, fileID string, names northstarFilesNames) error {
	parent, err := clients.FileFolders.Get(ctx, parentID)
	if err != nil {
		return fmt.Errorf("read Northstar staged-file parent folder: %s", acceptance.SanitizedHubSpotError(err))
	}
	child, err := clients.FileFolders.Get(ctx, childID)
	if err != nil {
		return fmt.Errorf("read Northstar staged-file child folder: %s", acceptance.SanitizedHubSpotError(err))
	}
	file, err := clients.Files.Get(ctx, fileID)
	if err != nil {
		return fmt.Errorf("read Northstar staged file: %s", acceptance.SanitizedHubSpotError(err))
	}
	if parent.ID != parentID || parent.Name != names.BrandFolder || parent.ParentFolderID != nil || child.ID != childID || child.Name != names.DownloadsFolder || child.ParentFolderID == nil || *child.ParentFolderID != parentID || file.ID != fileID || file.Name != names.PrivateFile || file.FolderID != childID {
		return errors.New("northstar staged file refresh drift did not match the exact owned configuration")
	}
	return nil
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

func verifyNorthstarMembership(ctx context.Context, clients *hubspot.ClientSet, id, email string) error {
	byID, err := clients.AccountMemberships.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("read Northstar membership by Settings ID: %s", acceptance.SanitizedHubSpotError(err))
	}
	byEmail, err := clients.AccountMemberships.GetByEmail(ctx, email)
	if err != nil {
		return fmt.Errorf("read Northstar membership by email: %s", acceptance.SanitizedHubSpotError(err))
	}
	if byID.ID != id || byEmail.ID != id || byID.Email != email || byEmail.Email != email {
		return errors.New("northstar membership did not match its exact Settings ID and email")
	}
	if byID.SuperAdmin || byEmail.SuperAdmin {
		return errors.New("northstar membership unexpectedly has Super Admin privileges")
	}
	return nil
}

func verifyNorthstarMembershipTerminal(ctx context.Context, clients *hubspot.ClientSet, id, email string) (string, error) {
	if err := clients.AccountMemberships.WaitForAbsence(ctx, id, email); err != nil {
		return "", fmt.Errorf("verify Northstar membership absence: %s", acceptance.SanitizedHubSpotError(err))
	}
	digest := sha256.Sum256([]byte("northstar-membership-identity\x00" + id + "\x00" + email))
	record, err := json.Marshal(struct {
		GeneratedIdentityHash  string `json:"generated_identity_hash"`
		ActiveOwnedMemberships int    `json:"active_owned_memberships"`
		Cleanup                string `json:"cleanup"`
	}{hex.EncodeToString(digest[:]), 0, "passed"})
	if err != nil {
		return "", errors.New("encode Northstar membership terminal record")
	}
	return string(record), nil
}

func verifyNorthstarCRMProfile(ctx context.Context, clients *hubspot.ClientSet, crmID, membershipID string) error {
	profile, err := exactNorthstarCRMProfile(ctx, clients, crmID, membershipID, true)
	if err != nil {
		return err
	}
	if !northstarCRMProfileMatches(profile) {
		return errors.New("northstar CRM profile did not match every managed property")
	}
	return nil
}

func driftNorthstarCRMProfile(ctx context.Context, clients *hubspot.ClientSet, crmID, membershipID string) error {
	if _, err := exactNorthstarCRMProfile(ctx, clients, crmID, membershipID, true); err != nil {
		return err
	}
	if _, err := clients.CRMUserProfiles.PatchProperties(ctx, crmID, map[string]string{
		"hs_job_title": "Out-of-band Northstar role", "hs_availability_status": "away",
	}); err != nil {
		return fmt.Errorf("author Northstar CRM profile drift: %s", acceptance.SanitizedHubSpotError(err))
	}
	drifted, err := exactNorthstarCRMProfile(ctx, clients, crmID, membershipID, true)
	if err != nil {
		return err
	}
	if drifted.JobTitle != "Out-of-band Northstar role" || drifted.AvailabilityStatus != "away" {
		return errors.New("northstar CRM profile drift mutation was not observable")
	}
	return nil
}

func verifyNorthstarCRMProfileTerminal(ctx context.Context, clients *hubspot.ClientSet, crmID, membershipID string) (string, error) {
	if _, err := clients.AccountMemberships.GetByID(ctx, membershipID); err == nil {
		return "", errors.New("northstar account membership remained active after teardown")
	} else {
		var apiError *hubspot.Error
		if !errors.As(err, &apiError) || apiError.Status != 404 {
			return "", fmt.Errorf("verify Northstar membership terminal identity: %s", acceptance.SanitizedHubSpotError(err))
		}
	}
	profile, err := exactNorthstarCRMProfile(ctx, clients, crmID, membershipID, false)
	if err != nil {
		return "", err
	}
	if !northstarCRMProfileMatches(profile) {
		return "", errors.New("northstar teardown did not retain the managed CRM profile values")
	}
	digest := sha256.Sum256([]byte("northstar-crm-profile-identity\x00" + crmID + "\x00" + membershipID))
	record, err := json.Marshal(struct {
		GeneratedIdentityHash string `json:"generated_identity_hash"`
		Residual              string `json:"residual"`
		RemoteWrite           string `json:"remote_write"`
		Cleanup               string `json:"cleanup"`
	}{hex.EncodeToString(digest[:]), "retained_profile_values", "none", "passed"})
	if err != nil {
		return "", errors.New("encode Northstar CRM profile terminal record")
	}
	return string(record), nil
}

func exactNorthstarCRMProfile(ctx context.Context, clients *hubspot.ClientSet, crmID, membershipID string, requireMembership bool) (hubspot.CRMUserProfile, error) {
	if requireMembership {
		membership, err := clients.AccountMemberships.GetByID(ctx, membershipID)
		if err != nil {
			return hubspot.CRMUserProfile{}, fmt.Errorf("read Northstar profile membership: %s", acceptance.SanitizedHubSpotError(err))
		}
		if membership.ID != membershipID {
			return hubspot.CRMUserProfile{}, errors.New("northstar profile membership returned a different Settings ID")
		}
	}
	profile, err := clients.CRMUserProfiles.Get(ctx, crmID)
	if err != nil {
		return hubspot.CRMUserProfile{}, fmt.Errorf("read Northstar CRM profile: %s", acceptance.SanitizedHubSpotError(err))
	}
	if profile.ID != crmID || profile.SettingsID != membershipID {
		return hubspot.CRMUserProfile{}, errors.New("northstar CRM profile did not match its exact CRM and Settings identities")
	}
	joined, err := clients.CRMUserProfiles.FindBySettingsID(ctx, membershipID)
	if err != nil {
		return hubspot.CRMUserProfile{}, fmt.Errorf("join Northstar CRM profile: %s", acceptance.SanitizedHubSpotError(err))
	}
	if joined.ID != crmID || joined.SettingsID != membershipID {
		return hubspot.CRMUserProfile{}, errors.New("northstar CRM profile join returned a different identity")
	}
	return profile, nil
}

func northstarCRMProfileMatches(profile hubspot.CRMUserProfile) bool {
	desiredHours, err := hubspot.SerializeCRMUserWorkingHours([]hubspot.CRMUserWorkingHours{{
		Days: "MONDAY_TO_FRIDAY", StartMinute: 540, EndMinute: 1020,
	}})
	if err != nil {
		return false
	}
	actualHours, err := hubspot.SerializeCRMUserWorkingHours(profile.WorkingHours)
	return err == nil && profile.JobTitle == northstarProfileJobTitle &&
		profile.AvailabilityStatus == northstarProfileAvailability && profile.TimeZone == northstarProfileTimeZone &&
		actualHours == desiredHours
}

func verifyNorthstarProduct(ctx context.Context, clients *hubspot.ClientSet, id string) error {
	product, err := clients.Products.Get(ctx, id)
	if err != nil {
		return fmt.Errorf("read Northstar Product: %s", acceptance.SanitizedHubSpotError(err))
	}
	if product.ID != id || product.Archived || product.Name != northstarProductName ||
		product.SKU != northstarProductSKU || product.Description != northstarProductDescription ||
		!northstarProductDecimalsEqual(product.Price, northstarProductPrice) ||
		!northstarProductDecimalsEqual(product.Cost, northstarProductCost) ||
		product.RecurringBillingPeriod != northstarProductRecurrence {
		return errors.New("northstar Product did not match every managed value and exact identity")
	}
	return nil
}

func driftNorthstarProduct(ctx context.Context, clients *hubspot.ClientSet, id string) error {
	if err := verifyNorthstarProduct(ctx, clients, id); err != nil {
		return err
	}
	if _, err := clients.Products.Patch(ctx, id, map[string]string{
		"description": "Out-of-band Northstar Product", "price": "1300",
		"hs_cost_of_goods_sold": "350", "hs_recurring_billing_period": "P6M",
	}); err != nil {
		return fmt.Errorf("author Northstar Product drift: %s", acceptance.SanitizedHubSpotError(err))
	}
	drifted, err := clients.Products.Get(ctx, id)
	if err != nil || drifted.ID != id || drifted.Description != "Out-of-band Northstar Product" ||
		!northstarProductDecimalsEqual(drifted.Price, "1300") || drifted.RecurringBillingPeriod != "P6M" {
		return errors.New("northstar Product drift mutation was not observable with preserved identity")
	}
	return nil
}

func archiveNorthstarProductForRefresh(ctx context.Context, clients *hubspot.ClientSet, id string) error {
	product, err := clients.Products.Get(ctx, id)
	if err != nil || product.ID != id || product.SKU != northstarProductSKU || product.Archived {
		return errors.New("northstar Product refresh target did not match the exact owned identity")
	}
	if err := clients.Products.Archive(ctx, id); err != nil {
		return fmt.Errorf("archive Northstar Product refresh target: %s", acceptance.SanitizedHubSpotError(err))
	}
	archived, err := clients.Products.GetArchived(ctx, id)
	if err != nil || archived.ID != id || !archived.Archived {
		return errors.New("northstar Product refresh target did not reach the exact archived identity")
	}
	return nil
}

func verifyNorthstarProductTerminal(ctx context.Context, clients *hubspot.ClientSet, id string) (string, error) {
	if _, err := clients.Products.Get(ctx, id); err == nil {
		return "", errors.New("northstar Product remained active after teardown")
	} else if !northstarNotFound(err) {
		return "", fmt.Errorf("verify Northstar Product active absence: %s", acceptance.SanitizedHubSpotError(err))
	}
	archived, err := clients.Products.GetArchived(ctx, id)
	if err != nil || archived.ID != id || !archived.Archived {
		return "", errors.New("northstar Product terminal identity was not exact")
	}
	active, err := clients.Products.List(ctx)
	if err != nil {
		return "", fmt.Errorf("list active Northstar Products: %s", acceptance.SanitizedHubSpotError(err))
	}
	for _, product := range active {
		if strings.HasPrefix(product.SKU, "ns_") {
			return "", errors.New("northstar teardown retained an active owned Product")
		}
	}
	digest := sha256.Sum256([]byte("northstar-product-identity\x00" + id))
	record, err := json.Marshal(struct {
		GeneratedIdentityHash string `json:"generated_identity_hash"`
		Terminal              string `json:"terminal"`
		ActiveOwnedProducts   int    `json:"active_owned_products"`
		Cleanup               string `json:"cleanup"`
	}{hex.EncodeToString(digest[:]), "archived", 0, "passed"})
	if err != nil {
		return "", errors.New("encode Northstar terminal Product record")
	}
	return string(record), nil
}

func northstarProductDecimalsEqual(first, second string) bool {
	firstValue, firstOK := new(big.Rat).SetString(first)
	secondValue, secondOK := new(big.Rat).SetString(second)
	return firstOK && secondOK && firstValue.Cmp(secondValue) == 0
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
