// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

//go:build acceptance

package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jackemcpherson/terraform-provider-hubspot/internal/acceptance"
	"github.com/jackemcpherson/terraform-provider-hubspot/internal/hubspot"
)

const northstarTestMembershipEmail = "tfhs-probe-16-20260802024807@example.com"

func TestNorthstarActionTimeoutPreservesFolderDriftMargin(t *testing.T) {
	const minimumMargin = 30 * time.Second

	if timeout := northstarActionTimeout("drift-folder-path"); timeout != 4*time.Minute {
		t.Fatalf("folder drift timeout = %s", timeout)
	}
	if timeout := northstarActionTimeout("verify-files"); timeout != 3*time.Minute {
		t.Fatalf("file verification timeout = %s", timeout)
	}
	if timeout := northstarActionTimeout("repair-folder-path"); timeout != 10*time.Minute {
		t.Fatalf("folder repair timeout = %s", timeout)
	}
	if timeout := northstarActionTimeout("cleanup"); timeout != 2*time.Minute {
		t.Fatalf("default timeout = %s", timeout)
	}
	var taskDelay time.Duration
	for _, delay := range northstarFolderTaskConvergenceDelays {
		taskDelay += delay
	}
	var descendantDelay time.Duration
	for _, delay := range northstarDescendantPathConvergenceDelays {
		descendantDelay += delay
	}
	var filesDelay time.Duration
	for _, delay := range northstarFilesConvergenceDelays {
		filesDelay += delay
	}
	if timeout := northstarActionTimeout("verify-files"); timeout <= descendantDelay+minimumMargin {
		t.Fatalf("file verification timeout = %s, convergence = %s", timeout, descendantDelay)
	}
	if timeout := northstarActionTimeout("drift-folder-path"); timeout <= taskDelay+descendantDelay+minimumMargin {
		t.Fatalf("folder drift timeout = %s, convergence = %s", timeout, taskDelay+descendantDelay)
	}
	if timeout := northstarActionTimeout("repair-folder-path"); timeout <= 2*(taskDelay+descendantDelay+filesDelay)+minimumMargin {
		t.Fatalf("folder repair timeout = %s, convergence = %s", timeout, 2*(taskDelay+descendantDelay+filesDelay))
	}
}

func TestExecuteAuthorsDriftAndArchivesRefreshTarget(t *testing.T) {
	server := httptest.NewServer(acceptance.NewFakeHubSpot("token", 123))
	defer server.Close()
	origin, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	clients, err := hubspot.NewClientSet(hubspot.TransportConfig{BaseURL: origin, AccessToken: "token"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	order := int64(1)
	sensitivity := "non_sensitive"
	if _, err := clients.PropertyGroups.Create(ctx, "contacts", hubspot.PropertyGroupCreate{Name: "ns_customer_context", Label: "Northstar", DisplayOrder: order}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"ns_buyer_role", "ns_last_success_review"} {
		if _, err := clients.Properties.Create(ctx, "contacts", hubspot.PropertyWrite{
			Name: name, Label: name, GroupName: "ns_customer_context", Type: "string", FieldType: "text", DataSensitivity: &sensitivity,
		}); err != nil {
			t.Fatal(err)
		}
	}
	form, err := clients.Forms.Create(ctx, northstarFormFixture())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := execute(ctx, "drift", []string{form.ID}, clients); err != nil {
		t.Fatal(err)
	}
	drifted, err := clients.Properties.Get(ctx, "contacts", "ns_buyer_role", false, "non_sensitive", "")
	if err != nil || drifted.Label != driftLabel {
		t.Fatalf("drifted property = %#v, %v", drifted, err)
	}
	driftedForm, err := clients.Forms.Get(ctx, form.ID)
	if err != nil || driftedForm.Name != "ns_contact_us_drift" || driftedForm.FieldGroups[0].Fields[0].Label != "Out-of-band work email" {
		t.Fatalf("drifted form = %#v, %v", driftedForm, err)
	}
	if _, err := execute(ctx, "archive-for-refresh", nil, clients); err != nil {
		t.Fatal(err)
	}
	if err := clients.Forms.Archive(ctx, form.ID); err != nil {
		t.Fatal(err)
	}
	record, err := execute(ctx, "verify-form-terminal", []string{form.ID}, clients)
	if err != nil || record == "" || strings.Contains(record, form.ID) || !strings.Contains(record, `"terminal":"archived"`) {
		t.Fatalf("terminal record = %q, %v", record, err)
	}
}

func TestExecuteRejectsUnknownAction(t *testing.T) {
	if _, err := execute(context.Background(), "unknown", nil, &hubspot.ClientSet{}); err == nil {
		t.Fatal("unknown action accepted")
	}
}

func TestExecuteVerifiesAccountMembershipLifecycle(t *testing.T) {
	fake := acceptance.NewFakeHubSpot("token", 123)
	server := httptest.NewServer(fake)
	defer server.Close()
	origin, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	clients, err := hubspot.NewClientSet(hubspot.TransportConfig{BaseURL: origin, AccessToken: "token"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	const email = "northstar-operator@example.com"
	membership, err := clients.AccountMemberships.Create(ctx, hubspot.AccountMembershipCreate{
		Email: email, SendWelcomeEmail: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	other, err := clients.AccountMemberships.Create(ctx, hubspot.AccountMembershipCreate{
		Email: "northstar-other@example.com", SendWelcomeEmail: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := execute(ctx, "verify-membership", []string{membership.ID, email}, clients); err != nil {
		t.Fatal(err)
	}
	if _, err := execute(ctx, "verify-membership", []string{membership.ID, other.Email}, clients); err == nil {
		t.Fatal("membership verification accepted a mismatched Settings ID and email")
	}
	fake.SetAccountMembershipSuperAdmin(membership.ID, true)
	if _, err := execute(ctx, "verify-membership", []string{membership.ID, email}, clients); err == nil {
		t.Fatal("membership verification accepted a Super Admin")
	}
	fake.SetAccountMembershipSuperAdmin(membership.ID, false)
	if _, err := execute(ctx, "verify-membership-terminal", []string{membership.ID, email}, clients); err == nil {
		t.Fatal("membership terminal verification accepted an active identity")
	}
	if err := clients.AccountMemberships.Delete(ctx, membership.ID); err != nil {
		t.Fatal(err)
	}
	if err := clients.AccountMemberships.Delete(ctx, other.ID); err != nil {
		t.Fatal(err)
	}
	record, err := execute(ctx, "verify-membership-terminal", []string{membership.ID, email}, clients)
	if err != nil || record == "" || strings.Contains(record, membership.ID) || strings.Contains(record, email) || !strings.Contains(record, `"active_owned_memberships":0`) {
		t.Fatalf("membership terminal record = %q, %v", record, err)
	}
}

func TestExecuteManagesNorthstarCRMUserProfileLifecycle(t *testing.T) {
	fake := acceptance.NewFakeHubSpot("token", 123)
	crmID := fake.SeedCRMUserProfile("101")
	server := httptest.NewServer(fake)
	defer server.Close()
	origin, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	clients, err := hubspot.NewClientSet(hubspot.TransportConfig{BaseURL: origin, AccessToken: "token"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	hours := []hubspot.CRMUserWorkingHours{{Days: "MONDAY_TO_FRIDAY", StartMinute: 540, EndMinute: 1020}}
	hoursJSON, err := hubspot.SerializeCRMUserWorkingHours(hours)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := clients.CRMUserProfiles.PatchProperties(ctx, crmID, map[string]string{
		"hs_job_title": "Cloud Operations Engineer", "hs_availability_status": "available", "hs_standard_time_zone": "Australia/Melbourne",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := clients.CRMUserProfiles.PatchProperties(ctx, crmID, map[string]string{"hs_working_hours": hoursJSON}); err != nil {
		t.Fatal(err)
	}
	ids := []string{crmID, "101"}
	if _, err := execute(ctx, "verify-profile", ids, clients); err != nil {
		t.Fatal(err)
	}
	if _, err := execute(ctx, "drift-profile", ids, clients); err != nil {
		t.Fatal(err)
	}
	if _, err := execute(ctx, "verify-profile", ids, clients); err == nil {
		t.Fatal("profile verification accepted drift")
	}
	drifted, err := clients.CRMUserProfiles.Get(ctx, crmID)
	if err != nil || drifted.JobTitle != "Out-of-band Northstar role" || drifted.AvailabilityStatus != "away" {
		t.Fatalf("drifted CRM profile = %#v, %v", drifted, err)
	}
	if _, err := clients.CRMUserProfiles.PatchProperties(ctx, crmID, map[string]string{
		"hs_job_title": "Cloud Operations Engineer", "hs_availability_status": "available",
	}); err != nil {
		t.Fatal(err)
	}
	if err := clients.AccountMemberships.Delete(ctx, "101"); err != nil {
		t.Fatal(err)
	}
	record, err := execute(ctx, "verify-profile-terminal", ids, clients)
	if err != nil || strings.Contains(record, crmID) || strings.Contains(record, "101") ||
		!strings.Contains(record, `"residual":"retained_profile_values"`) || !strings.Contains(record, `"remote_write":"none"`) {
		t.Fatalf("CRM profile terminal record = %q, %v", record, err)
	}
}

func TestExecuteAcceptsMissingCRMUserProfileAfterMembershipTeardown(t *testing.T) {
	fake := acceptance.NewFakeHubSpot("token", 123)
	crmID := fake.SeedCRMUserProfile("101")
	server := httptest.NewServer(fake)
	defer server.Close()
	origin, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	clients, err := hubspot.NewClientSet(hubspot.TransportConfig{BaseURL: origin, AccessToken: "token"})
	if err != nil {
		t.Fatal(err)
	}
	if err := clients.AccountMemberships.Delete(context.Background(), "101"); err != nil {
		t.Fatal(err)
	}
	if !fake.DisappearCRMUserProfile(crmID) {
		t.Fatal("could not remove the membership-scoped CRM projection")
	}
	record, err := execute(context.Background(), "verify-profile-terminal", []string{crmID, "101"}, clients)
	if err != nil || strings.Contains(record, crmID) || strings.Contains(record, "101") ||
		!strings.Contains(record, `"residual":"profile_projection_absent"`) || !strings.Contains(record, `"remote_write":"none"`) {
		t.Fatalf("missing CRM profile terminal record = %q, %v", record, err)
	}
}

func TestExecuteManagesNorthstarProductLifecycle(t *testing.T) {
	fake := acceptance.NewFakeHubSpot("token", 123)
	server := httptest.NewServer(fake)
	defer server.Close()
	origin, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	clients, err := hubspot.NewClientSet(hubspot.TransportConfig{BaseURL: origin, AccessToken: "token"})
	if err != nil {
		t.Fatal(err)
	}
	cost := "300.00"
	recurrence := "P12M"
	product, err := clients.Products.Create(context.Background(), hubspot.ProductWrite{
		Name: "Northstar annual support", SKU: "ns_support_annual",
		Description: "Priority support for Northstar customers", Price: "1200.00",
		Cost: &cost, RecurringBillingPeriod: &recurrence,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := execute(ctx, "verify-product", []string{product.ID}, clients); err != nil {
		t.Fatal(err)
	}
	if _, err := execute(ctx, "drift-product", []string{product.ID}, clients); err != nil {
		t.Fatal(err)
	}
	if _, err := execute(ctx, "verify-product", []string{product.ID}, clients); err == nil {
		t.Fatal("Product verification accepted drift")
	}
	if _, err := execute(ctx, "archive-product-for-refresh", []string{product.ID}, clients); err != nil {
		t.Fatal(err)
	}
	record, err := execute(ctx, "verify-product-terminal", []string{product.ID}, clients)
	if err != nil || record == "" || strings.Contains(record, `"id"`) ||
		!strings.Contains(record, `"terminal":"archived"`) || !strings.Contains(record, `"active_owned_products":0`) {
		t.Fatalf("Product terminal record = %q, %v", record, err)
	}
}

func TestExecuteCleansInterruptedNorthstarLifecycle(t *testing.T) {
	t.Setenv("HUBSPOT_NORTHSTAR_FILES_PREFIX", "ns_1a2b3c4d_o_")
	names, err := northstarFilesNamesFromEnvironment()
	if err != nil {
		t.Fatal(err)
	}
	fake := acceptance.NewFakeHubSpot("token", 123)
	server := httptest.NewServer(fake)
	defer server.Close()
	origin, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	clients, err := hubspot.NewClientSet(hubspot.TransportConfig{BaseURL: origin, AccessToken: "token"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	membershipEmail := northstarTestMembershipEmail
	t.Setenv("HUBSPOT_NORTHSTAR_MEMBERSHIP_EMAIL", membershipEmail)
	membership, err := clients.AccountMemberships.Create(ctx, hubspot.AccountMembershipCreate{
		Email: membershipEmail, SendWelcomeEmail: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	baseline, err := clients.AccountMemberships.Create(ctx, hubspot.AccountMembershipCreate{
		Email: "baseline@example.invalid", SendWelcomeEmail: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	order := int64(1)
	sensitivity := "non_sensitive"
	for objectType, names := range northstarCRMNames {
		for name := range names.groups {
			if _, err := clients.PropertyGroups.Create(ctx, objectType, hubspot.PropertyGroupCreate{Name: name, Label: name, DisplayOrder: order}); err != nil {
				t.Fatal(err)
			}
		}
		for name := range names.properties {
			groupName := ""
			for candidate := range names.groups {
				groupName = candidate
				break
			}
			if _, err := clients.Properties.Create(ctx, objectType, hubspot.PropertyWrite{
				Name: name, Label: name, GroupName: groupName, Type: "string", FieldType: "text", DataSensitivity: &sensitivity,
			}); err != nil {
				t.Fatal(err)
			}
		}
	}
	form, err := clients.Forms.Create(ctx, northstarFormFixture())
	if err != nil {
		t.Fatal(err)
	}
	cost := northstarProductCost
	recurrence := northstarProductRecurrence
	product, err := clients.Products.Create(ctx, hubspot.ProductWrite{
		Name: northstarProductName, SKU: northstarProductSKU, Description: northstarProductDescription,
		Price: northstarProductPrice, Cost: &cost, RecurringBillingPeriod: &recurrence,
	})
	if err != nil {
		t.Fatal(err)
	}
	brand, err := clients.FileFolders.Create(ctx, hubspot.FileFolderWrite{Name: names.BrandFolder})
	if err != nil {
		t.Fatal(err)
	}
	downloads, err := clients.FileFolders.Create(ctx, hubspot.FileFolderWrite{Name: names.DownloadsFolder, ParentFolderID: &brand.ID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := clients.Files.Upload(ctx, hubspot.FileUpload{Name: names.PrivateFile, FolderID: brand.ID, Access: "PRIVATE", Bytes: []byte("Northstar private file\n")}); err != nil {
		t.Fatal(err)
	}
	if _, err := clients.Files.Upload(ctx, hubspot.FileUpload{Name: names.PublicFile, FolderID: downloads.ID, Access: "PUBLIC_NOT_INDEXABLE", Bytes: []byte("Northstar public file\n")}); err != nil {
		t.Fatal(err)
	}
	fake.LagNextAccountMembershipCollectionAfterDelete(2)
	result, err := execute(ctx, "cleanup", nil, clients)
	if err != nil || result != "Northstar cleanup verified zero active owned configuration" {
		t.Fatalf("cleanup result = %q, %v", result, err)
	}
	if _, err := clients.Forms.Get(ctx, form.ID); err == nil {
		t.Fatal("cleanup left the Northstar Form active")
	}
	if _, err := clients.Forms.GetArchived(ctx, form.ID); err != nil {
		t.Fatal("cleanup did not retain the Northstar Form tombstone")
	}
	if archived, err := clients.Products.GetArchived(ctx, product.ID); err != nil || archived.ID != product.ID || !archived.Archived {
		t.Fatalf("cleanup Product tombstone = %#v, %v", archived, err)
	}
	if _, err := clients.AccountMemberships.GetByID(ctx, membership.ID); !northstarNotFound(err) {
		t.Fatal("cleanup left the disposable Northstar membership active")
	}
	if current, err := clients.AccountMemberships.GetByID(ctx, baseline.ID); err != nil || current.Email != baseline.Email {
		t.Fatalf("cleanup changed the baseline membership = %#v, %v", current, err)
	}
	if err := verifyNorthstarCleanup(ctx, clients, names, membershipEmail); err != nil {
		t.Fatal(err)
	}
}

func TestExecuteCleanupRejectsUnexpectedPrefixBeforeMutation(t *testing.T) {
	t.Setenv("HUBSPOT_NORTHSTAR_MEMBERSHIP_EMAIL", northstarTestMembershipEmail)
	server := httptest.NewServer(acceptance.NewFakeHubSpot("token", 123))
	defer server.Close()
	origin, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	clients, err := hubspot.NewClientSet(hubspot.TransportConfig{BaseURL: origin, AccessToken: "token"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	form, err := clients.Forms.Create(ctx, northstarFormFixture())
	if err != nil {
		t.Fatal(err)
	}
	order := int64(1)
	if _, err := clients.PropertyGroups.Create(ctx, "contacts", hubspot.PropertyGroupCreate{Name: "ns_unexpected", Label: "Unexpected", DisplayOrder: order}); err != nil {
		t.Fatal(err)
	}
	if _, err := execute(ctx, "cleanup", nil, clients); err == nil || !strings.Contains(err.Error(), "refusing unexpected") {
		t.Fatalf("cleanup error = %v", err)
	}
	if _, err := clients.Forms.Get(ctx, form.ID); err != nil {
		t.Fatal("failed preflight partially mutated the Northstar Form")
	}
	if _, err := clients.PropertyGroups.Get(ctx, "contacts", "ns_unexpected"); err != nil {
		t.Fatal("failed preflight partially mutated the unexpected property group")
	}
}

func TestExecuteCleanupRejectsUnexpectedProductBeforeMutation(t *testing.T) {
	t.Setenv("HUBSPOT_NORTHSTAR_MEMBERSHIP_EMAIL", northstarTestMembershipEmail)
	server := httptest.NewServer(acceptance.NewFakeHubSpot("token", 123))
	defer server.Close()
	origin, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	clients, err := hubspot.NewClientSet(hubspot.TransportConfig{BaseURL: origin, AccessToken: "token"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	product, err := clients.Products.Create(ctx, hubspot.ProductWrite{
		Name: "Unexpected", SKU: "ns_unexpected", Description: "Must remain active", Price: "1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := execute(ctx, "cleanup", nil, clients); err == nil || !strings.Contains(err.Error(), "refusing unexpected") {
		t.Fatalf("cleanup error = %v", err)
	}
	if active, err := clients.Products.Get(ctx, product.ID); err != nil || active.ID != product.ID || active.Archived {
		t.Fatalf("failed preflight mutated Product = %#v, %v", active, err)
	}
}

func TestExecuteCleanupRejectsFileInsideTemporaryRepairFolder(t *testing.T) {
	t.Setenv("HUBSPOT_NORTHSTAR_MEMBERSHIP_EMAIL", northstarTestMembershipEmail)
	t.Setenv("HUBSPOT_NORTHSTAR_FILES_PREFIX", "ns_1a2b3c4d_o_")
	names, err := northstarFilesNamesFromEnvironment()
	if err != nil {
		t.Fatal(err)
	}
	fake := acceptance.NewFakeHubSpot("token", 123)
	server := httptest.NewServer(fake)
	defer server.Close()
	origin, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	clients, err := hubspot.NewClientSet(hubspot.TransportConfig{BaseURL: origin, AccessToken: "token"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	brand, err := clients.FileFolders.Create(ctx, hubspot.FileFolderWrite{Name: names.BrandFolder})
	if err != nil {
		t.Fatal(err)
	}
	repair, err := clients.FileFolders.Create(ctx, hubspot.FileFolderWrite{Name: names.DownloadsFolderRepair, ParentFolderID: &brand.ID})
	if err != nil {
		t.Fatal(err)
	}
	file, err := clients.Files.Upload(ctx, hubspot.FileUpload{Name: names.PrivateFile, FolderID: repair.ID, Access: "PRIVATE", Bytes: []byte("Northstar private file\n")})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := execute(ctx, "cleanup", nil, clients); err == nil || !strings.Contains(err.Error(), "unexpected placement") {
		t.Fatalf("cleanup error = %v", err)
	}
	if current, err := clients.Files.Get(ctx, file.ID); err != nil || current.FolderID != repair.ID {
		t.Fatalf("failed preflight mutated repair-folder file = %#v, %v", current, err)
	}
	if _, err := clients.FileFolders.Get(ctx, repair.ID); err != nil {
		t.Fatal("failed preflight mutated the temporary repair folder")
	}
	if _, err := clients.FileFolders.Get(ctx, brand.ID); err != nil {
		t.Fatal("failed preflight mutated the parent folder")
	}
}

func TestExecuteCleansReachableFolderRepairCheckpoints(t *testing.T) {
	testCases := map[string]struct {
		childRepairName bool
		privateInParent bool
		publicInParent  bool
	}{
		"files staged under parent": {privateInParent: true, publicInParent: true},
		"temporary empty child":     {childRepairName: true, privateInParent: true, publicInParent: true},
		"split outbound move":       {privateInParent: true},
		"split restore move":        {publicInParent: true},
	}
	for name, checkpoint := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Setenv("HUBSPOT_NORTHSTAR_MEMBERSHIP_EMAIL", northstarTestMembershipEmail)
			t.Setenv("HUBSPOT_NORTHSTAR_FILES_PREFIX", "ns_1a2b3c4d_o_")
			names, err := northstarFilesNamesFromEnvironment()
			if err != nil {
				t.Fatal(err)
			}
			fake := acceptance.NewFakeHubSpot("token", 123)
			server := httptest.NewServer(fake)
			defer server.Close()
			origin, err := url.Parse(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			clients, err := hubspot.NewClientSet(hubspot.TransportConfig{BaseURL: origin, AccessToken: "token"})
			if err != nil {
				t.Fatal(err)
			}
			ctx := context.Background()
			brand, err := clients.FileFolders.Create(ctx, hubspot.FileFolderWrite{Name: names.BrandFolder})
			if err != nil {
				t.Fatal(err)
			}
			childName := names.DownloadsFolder
			if checkpoint.childRepairName {
				childName = names.DownloadsFolderRepair
			}
			downloads, err := clients.FileFolders.Create(ctx, hubspot.FileFolderWrite{Name: childName, ParentFolderID: &brand.ID})
			if err != nil {
				t.Fatal(err)
			}
			privateFolderID := downloads.ID
			if checkpoint.privateInParent {
				privateFolderID = brand.ID
			}
			publicFolderID := downloads.ID
			if checkpoint.publicInParent {
				publicFolderID = brand.ID
			}
			if _, err := clients.Files.Upload(ctx, hubspot.FileUpload{Name: names.PrivateFile, FolderID: privateFolderID, Access: "PRIVATE", Bytes: []byte("Northstar private file\n")}); err != nil {
				t.Fatal(err)
			}
			if _, err := clients.Files.Upload(ctx, hubspot.FileUpload{Name: names.PublicFile, FolderID: publicFolderID, Access: "PUBLIC_NOT_INDEXABLE", Bytes: []byte("Northstar public file\n")}); err != nil {
				t.Fatal(err)
			}
			result, err := execute(ctx, "cleanup", nil, clients)
			if err != nil || result != "Northstar cleanup verified zero active owned configuration" {
				t.Fatalf("cleanup result = %q, %v", result, err)
			}
			if files, folders := fake.ActiveManagedFileIDs(), fake.ActiveFileFolderIDs(); len(files) != 0 || len(folders) != 0 {
				t.Fatalf("cleanup left files %v or folders %v", files, folders)
			}
		})
	}
}

func TestExecuteCleanupRejectsUnsafeMembershipBeforeMutation(t *testing.T) {
	testCases := map[string]func(*acceptance.FakeHubSpot, string){
		"Super Admin": func(fake *acceptance.FakeHubSpot, id string) {
			fake.SetAccountMembershipSuperAdmin(id, true)
		},
		"role assignment": func(fake *acceptance.FakeHubSpot, id string) {
			fake.SetAccountMembershipAssignments(id, true)
		},
		"team assignment": func(fake *acceptance.FakeHubSpot, id string) {
			fake.SetAccountMembershipTeamAssignment(id, true)
		},
	}
	for name, makeUnsafe := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Setenv("HUBSPOT_NORTHSTAR_MEMBERSHIP_EMAIL", northstarTestMembershipEmail)
			fake := acceptance.NewFakeHubSpot("token", 123)
			server := httptest.NewServer(fake)
			defer server.Close()
			origin, err := url.Parse(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			clients, err := hubspot.NewClientSet(hubspot.TransportConfig{BaseURL: origin, AccessToken: "token"})
			if err != nil {
				t.Fatal(err)
			}
			ctx := context.Background()
			membership, err := clients.AccountMemberships.Create(ctx, hubspot.AccountMembershipCreate{
				Email: northstarTestMembershipEmail, SendWelcomeEmail: false,
			})
			if err != nil {
				t.Fatal(err)
			}
			makeUnsafe(fake, membership.ID)
			form, err := clients.Forms.Create(ctx, northstarFormFixture())
			if err != nil {
				t.Fatal(err)
			}
			if _, err := execute(ctx, "cleanup", nil, clients); err == nil || !strings.Contains(err.Error(), "refusing unsafe") {
				t.Fatalf("cleanup error = %v", err)
			}
			if _, err := clients.AccountMemberships.GetByID(ctx, membership.ID); err != nil {
				t.Fatal("failed preflight mutated the unsafe membership")
			}
			if _, err := clients.Forms.Get(ctx, form.ID); err != nil {
				t.Fatal("failed preflight partially mutated the Northstar Form")
			}
			_, _, deleteCount := fake.AccountMembershipWriteCounts(membership.ID)
			if deleteCount != 0 {
				t.Fatal("failed preflight sent membership deletion")
			}
		})
	}
}

func TestNorthstarMembershipEmailRequiresReservedFixture(t *testing.T) {
	for _, email := range []string{"", "owner@example.com", "tfhs-probe-16-run@example.com", "tfhs-probe-16-20260802024808@example.com"} {
		t.Run(email, func(t *testing.T) {
			t.Setenv("HUBSPOT_NORTHSTAR_MEMBERSHIP_EMAIL", email)
			if _, err := northstarMembershipEmailFromEnvironment(); err == nil {
				t.Fatal("unsafe Northstar membership email accepted")
			}
		})
	}
	t.Setenv("HUBSPOT_NORTHSTAR_MEMBERSHIP_EMAIL", northstarTestMembershipEmail)
	if email, err := northstarMembershipEmailFromEnvironment(); err != nil || email != northstarTestMembershipEmail {
		t.Fatalf("reserved Northstar membership email = %q, %v", email, err)
	}
}

func TestDeleteNorthstarMembershipRejectsEmailReuseAfterIDAbsence(t *testing.T) {
	fake := acceptance.NewFakeHubSpot("token", 123)
	server := httptest.NewServer(fake)
	defer server.Close()
	origin, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	clients, err := hubspot.NewClientSet(hubspot.TransportConfig{BaseURL: origin, AccessToken: "token"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	membership, err := clients.AccountMemberships.Create(ctx, hubspot.AccountMembershipCreate{
		Email: northstarTestMembershipEmail, SendWelcomeEmail: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !fake.DisappearAccountMembership(membership.ID) {
		t.Fatal("failed to remove the inventoried membership")
	}
	fake.OverrideNextAccountMembershipEmailRead(northstarTestMembershipEmail, hubspot.AccountMembership{
		ID: "99999", Email: northstarTestMembershipEmail,
	})
	if err := deleteNorthstarMembership(ctx, clients, membership); err == nil || !strings.Contains(err.Error(), "refusing") {
		t.Fatalf("membership deletion error = %v", err)
	}
}

func TestDeleteNorthstarMembershipAcceptsConcurrentAbsence(t *testing.T) {
	fake := acceptance.NewFakeHubSpot("token", 123)
	server := httptest.NewServer(fake)
	defer server.Close()
	origin, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	clients, err := hubspot.NewClientSet(hubspot.TransportConfig{BaseURL: origin, AccessToken: "token"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	membership, err := clients.AccountMemberships.Create(ctx, hubspot.AccountMembershipCreate{
		Email: northstarTestMembershipEmail, SendWelcomeEmail: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !fake.DisappearAccountMembership(membership.ID) {
		t.Fatal("failed to remove the inventoried membership")
	}
	if err := deleteNorthstarMembership(ctx, clients, membership); err != nil {
		t.Fatalf("concurrently completed membership deletion = %v", err)
	}
	_, _, deleteCount := fake.AccountMembershipWriteCounts(membership.ID)
	if deleteCount != 0 {
		t.Fatal("concurrently absent membership sent a redundant deletion")
	}
}

func TestDeleteNorthstarMembershipRevalidatesCurrentIdentityAndSafety(t *testing.T) {
	teamID := "team-1"
	testCases := map[string]func(hubspot.AccountMembership) hubspot.AccountMembership{
		"changed ID": func(current hubspot.AccountMembership) hubspot.AccountMembership {
			current.ID = "99999"
			return current
		},
		"changed email": func(current hubspot.AccountMembership) hubspot.AccountMembership {
			current.Email = "replacement@example.invalid"
			return current
		},
		"became Super Admin": func(current hubspot.AccountMembership) hubspot.AccountMembership {
			current.SuperAdmin = true
			return current
		},
		"gained role": func(current hubspot.AccountMembership) hubspot.AccountMembership {
			current.RoleIDs = []string{"role-1"}
			return current
		},
		"gained team": func(current hubspot.AccountMembership) hubspot.AccountMembership {
			current.PrimaryTeamID = &teamID
			return current
		},
	}
	for name, changeCurrent := range testCases {
		t.Run(name, func(t *testing.T) {
			fake := acceptance.NewFakeHubSpot("token", 123)
			server := httptest.NewServer(fake)
			defer server.Close()
			origin, err := url.Parse(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			clients, err := hubspot.NewClientSet(hubspot.TransportConfig{BaseURL: origin, AccessToken: "token"})
			if err != nil {
				t.Fatal(err)
			}
			ctx := context.Background()
			membership, err := clients.AccountMemberships.Create(ctx, hubspot.AccountMembershipCreate{
				Email: northstarTestMembershipEmail, SendWelcomeEmail: false,
			})
			if err != nil {
				t.Fatal(err)
			}
			fake.OverrideNextAccountMembershipIDRead(membership.ID, changeCurrent(membership))
			if err := deleteNorthstarMembership(ctx, clients, membership); err == nil || !strings.Contains(err.Error(), "refusing") {
				t.Fatalf("membership deletion error = %v", err)
			}
			_, _, deleteCount := fake.AccountMembershipWriteCounts(membership.ID)
			if deleteCount != 0 {
				t.Fatal("unsafe revalidation sent membership deletion")
			}
		})
	}
}

func TestDeleteNorthstarMembershipRevalidatesEmailIdentityAndSafety(t *testing.T) {
	teamID := "team-1"
	testCases := map[string]func(hubspot.AccountMembership) hubspot.AccountMembership{
		"changed ID": func(byEmail hubspot.AccountMembership) hubspot.AccountMembership {
			byEmail.ID = "99999"
			return byEmail
		},
		"changed email": func(byEmail hubspot.AccountMembership) hubspot.AccountMembership {
			byEmail.Email = "replacement@example.invalid"
			return byEmail
		},
		"became Super Admin": func(byEmail hubspot.AccountMembership) hubspot.AccountMembership {
			byEmail.SuperAdmin = true
			return byEmail
		},
		"gained role": func(byEmail hubspot.AccountMembership) hubspot.AccountMembership {
			byEmail.RoleIDs = []string{"role-1"}
			return byEmail
		},
		"gained team": func(byEmail hubspot.AccountMembership) hubspot.AccountMembership {
			byEmail.PrimaryTeamID = &teamID
			return byEmail
		},
	}
	for name, changeByEmail := range testCases {
		t.Run(name, func(t *testing.T) {
			fake := acceptance.NewFakeHubSpot("token", 123)
			server := httptest.NewServer(fake)
			defer server.Close()
			origin, err := url.Parse(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			clients, err := hubspot.NewClientSet(hubspot.TransportConfig{BaseURL: origin, AccessToken: "token"})
			if err != nil {
				t.Fatal(err)
			}
			ctx := context.Background()
			membership, err := clients.AccountMemberships.Create(ctx, hubspot.AccountMembershipCreate{
				Email: northstarTestMembershipEmail, SendWelcomeEmail: false,
			})
			if err != nil {
				t.Fatal(err)
			}
			fake.OverrideNextAccountMembershipEmailRead(membership.Email, changeByEmail(membership))
			if err := deleteNorthstarMembership(ctx, clients, membership); err == nil || !strings.Contains(err.Error(), "refusing") {
				t.Fatalf("membership deletion error = %v", err)
			}
			_, _, deleteCount := fake.AccountMembershipWriteCounts(membership.ID)
			if deleteCount != 0 {
				t.Fatal("unsafe email revalidation sent membership deletion")
			}
		})
	}
}

func TestExecuteManagesNorthstarFilesLifecycle(t *testing.T) {
	t.Setenv("HUBSPOT_NORTHSTAR_FILES_PREFIX", "ns_1a2b3c4d_o_")
	names, err := northstarFilesNamesFromEnvironment()
	if err != nil {
		t.Fatal(err)
	}
	fake := acceptance.NewFakeHubSpot("token", 123)
	server := httptest.NewServer(fake)
	defer server.Close()
	origin, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	clients, err := hubspot.NewClientSet(hubspot.TransportConfig{BaseURL: origin, AccessToken: "token"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	brand, err := clients.FileFolders.Create(ctx, hubspot.FileFolderWrite{Name: names.BrandFolder})
	if err != nil {
		t.Fatal(err)
	}
	downloads, err := clients.FileFolders.Create(ctx, hubspot.FileFolderWrite{Name: names.DownloadsFolder, ParentFolderID: &brand.ID})
	if err != nil {
		t.Fatal(err)
	}
	privateFile, err := clients.Files.Upload(ctx, hubspot.FileUpload{Name: names.PrivateFile, FolderID: brand.ID, Access: "PRIVATE", Bytes: []byte("Northstar private file\n")})
	if err != nil {
		t.Fatal(err)
	}
	publicFile, err := clients.Files.Upload(ctx, hubspot.FileUpload{Name: names.PublicFile, FolderID: downloads.ID, Access: "PUBLIC_NOT_INDEXABLE", Bytes: []byte("<svg xmlns=\"http://www.w3.org/2000/svg\" viewBox=\"0 0 1 1\"><path d=\"M0 0h1v1H0z\"/></svg>\n")})
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{brand.ID, downloads.ID, privateFile.ID, publicFile.ID}
	if _, err := execute(ctx, "verify-files", ids, clients); err != nil {
		t.Fatal(err)
	}
	originalReadDelays := northstarFilesConvergenceDelays
	northstarFilesConvergenceDelays = []time.Duration{0, 0, 0, 0, 0}
	t.Cleanup(func() { northstarFilesConvergenceDelays = originalReadDelays })
	fake.LagNextManagedFileMoveVisibility(2, 2)
	if _, err := execute(ctx, "stage-file-for-folder-rename", []string{privateFile.ID, brand.ID, downloads.ID}, clients); err != nil {
		t.Fatal(err)
	}
	if readLag, searchLag := fake.ManagedFileMoveVisibilityLag(); readLag != 0 || searchLag != 0 {
		t.Fatalf("staged file visibility lag remained: read=%d search=%d", readLag, searchLag)
	}
	t.Setenv("HUBSPOT_NORTHSTAR_FILES_STAGED", "1")
	if _, err := execute(ctx, "verify-files", ids, clients); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HUBSPOT_NORTHSTAR_FILE_REFRESH_DRIFT", "1")
	t.Setenv("HUBSPOT_NORTHSTAR_PRIVATE_FILE_ID", privateFile.ID)
	if _, err := execute(ctx, "drift-folder-path", []string{brand.ID, downloads.ID}, clients); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HUBSPOT_NORTHSTAR_FILE_REFRESH_DRIFT", "")
	t.Setenv("HUBSPOT_NORTHSTAR_FILES_STAGED", "")
	if _, err := execute(ctx, "drift-files", []string{publicFile.ID}, clients); err != nil {
		t.Fatal(err)
	}
	drifted, err := clients.Files.Get(ctx, publicFile.ID)
	if err != nil || drifted.Name != names.PublicFileDrift || drifted.Access != "PRIVATE" || drifted.FileMD5 == publicFile.FileMD5 {
		t.Fatalf("drifted file = %#v, %v", drifted, err)
	}
	publicName, publicAccess := names.PublicFile, "PUBLIC_NOT_INDEXABLE"
	repairedPublic, err := clients.Files.Update(ctx, publicFile.ID, hubspot.FilePatch{Name: &publicName, Access: &publicAccess})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := clients.Files.Replace(ctx, publicFile.ID, hubspot.FileReplacement{Name: repairedPublic.Name, Access: repairedPublic.Access, Bytes: []byte("<svg xmlns=\"http://www.w3.org/2000/svg\" viewBox=\"0 0 1 1\"><path d=\"M0 0h1v1H0z\"/></svg>\n")}); err != nil {
		t.Fatal(err)
	}
	if _, err := execute(ctx, "drift-folder-path", []string{brand.ID, downloads.ID}, clients); err != nil {
		t.Fatal(err)
	}
	if patches, asyncUpdates := fake.FileFolderWriteCounts(brand.ID); patches != 0 || asyncUpdates != 1 {
		t.Fatalf("Northstar parent folder drift writes = PATCH %d, async update %d", patches, asyncUpdates)
	}
	driftedDownloads, err := clients.FileFolders.Get(ctx, downloads.ID)
	if err != nil || driftedDownloads.Path != "/"+names.BrandFolderRefresh+"/"+names.DownloadsFolder {
		t.Fatalf("drifted folder = %#v, %v", driftedDownloads, err)
	}
	if _, err := clients.FileFolders.Rename(ctx, brand.ID, names.BrandFolder); err != nil {
		t.Fatal(err)
	}
	staleDownloads, err := clients.FileFolders.Get(ctx, downloads.ID)
	if err != nil || staleDownloads.Path != "/"+names.BrandFolderRefresh+"/"+names.DownloadsFolder {
		t.Fatalf("stale repaired descendant = %#v, %v", staleDownloads, err)
	}
	privatePatches, privateReplacements := fake.ManagedFileWriteCounts(privateFile.ID)
	publicPatches, publicReplacements := fake.ManagedFileWriteCounts(publicFile.ID)
	privatePatchHistory := fake.ManagedFilePatchHistory(privateFile.ID)
	publicPatchHistory := fake.ManagedFilePatchHistory(publicFile.ID)
	if _, err := execute(ctx, "repair-folder-path", []string{brand.ID, downloads.ID, privateFile.ID, publicFile.ID}, clients); err != nil {
		t.Fatal(err)
	}
	if patches, asyncUpdates := fake.FileFolderWriteCounts(downloads.ID); patches != 0 || asyncUpdates != 2 {
		t.Fatalf("Northstar child folder repair writes = PATCH %d, async update %d", patches, asyncUpdates)
	}
	if patches, asyncUpdates := fake.FileFolderWriteCounts(brand.ID); patches != 1 || asyncUpdates != 1 {
		t.Fatalf("Northstar parent folder writes after child repair = PATCH %d, async update %d", patches, asyncUpdates)
	}
	repairedDownloads, err := clients.FileFolders.Get(ctx, downloads.ID)
	if err != nil || repairedDownloads.Path != "/"+names.BrandFolder+"/"+names.DownloadsFolder {
		t.Fatalf("repaired descendant = %#v, %v", repairedDownloads, err)
	}
	if patches, replacements := fake.ManagedFileWriteCounts(privateFile.ID); patches != privatePatches+2 || replacements != privateReplacements {
		t.Fatalf("Northstar private file repair writes = PATCH %d, replacement %d", patches, replacements)
	}
	if patches, replacements := fake.ManagedFileWriteCounts(publicFile.ID); patches != publicPatches+2 || replacements != publicReplacements {
		t.Fatalf("Northstar public file repair writes = PATCH %d, replacement %d", patches, replacements)
	}
	brandID, downloadsID := brand.ID, downloads.ID
	wantFileMoves := []hubspot.FilePatch{{FolderID: &brandID}, {FolderID: &downloadsID}}
	if history := fake.ManagedFilePatchHistory(privateFile.ID); !reflect.DeepEqual(history[len(privatePatchHistory):], wantFileMoves) {
		t.Fatalf("Northstar private file repair PATCHes = %#v", history[len(privatePatchHistory):])
	}
	if history := fake.ManagedFilePatchHistory(publicFile.ID); !reflect.DeepEqual(history[len(publicPatchHistory):], wantFileMoves) {
		t.Fatalf("Northstar public file repair PATCHes = %#v", history[len(publicPatchHistory):])
	}
	wantFolderUpdates := []hubspot.FileFolderWrite{
		{Name: names.DownloadsFolderRepair, ParentFolderID: &brandID},
		{Name: names.DownloadsFolder, ParentFolderID: &brandID},
	}
	if history := fake.FileFolderAsyncUpdateHistory(downloads.ID); !reflect.DeepEqual(history, wantFolderUpdates) {
		t.Fatalf("Northstar child folder repair updates = %#v", history)
	}
	currentPrivate, err := clients.Files.Get(ctx, privateFile.ID)
	if err != nil || !northstarRepairFileMatches(currentPrivate, privateFile, downloads.ID) {
		t.Fatalf("repaired private file = %#v, %v", currentPrivate, err)
	}
	currentPublic, err := clients.Files.Get(ctx, publicFile.ID)
	if err != nil || !northstarRepairFileMatches(currentPublic, publicFile, downloads.ID) {
		t.Fatalf("repaired public file = %#v, %v", currentPublic, err)
	}
	repairedBrand, err := clients.FileFolders.Get(ctx, brand.ID)
	if err != nil {
		t.Fatal(err)
	}
	brandPatches, brandAsyncUpdates := fake.FileFolderWriteCounts(brand.ID)
	childPatches, childAsyncUpdates := fake.FileFolderWriteCounts(downloads.ID)
	if _, err := execute(ctx, "repair-folder-path", []string{downloads.ID, brand.ID, privateFile.ID, publicFile.ID}, clients); err == nil || !strings.Contains(err.Error(), "identities") {
		t.Fatalf("unsafe folder repair = %v", err)
	}
	afterBrand, err := clients.FileFolders.Get(ctx, brand.ID)
	if err != nil {
		t.Fatal(err)
	}
	afterDownloads, err := clients.FileFolders.Get(ctx, downloads.ID)
	if err != nil {
		t.Fatal(err)
	}
	afterBrandPatches, afterBrandAsyncUpdates := fake.FileFolderWriteCounts(brand.ID)
	afterChildPatches, afterChildAsyncUpdates := fake.FileFolderWriteCounts(downloads.ID)
	if !reflect.DeepEqual(afterBrand, repairedBrand) || !reflect.DeepEqual(afterDownloads, repairedDownloads) || afterBrandPatches != brandPatches || afterBrandAsyncUpdates != brandAsyncUpdates || afterChildPatches != childPatches || afterChildAsyncUpdates != childAsyncUpdates {
		t.Fatalf("unsafe folder repair changed state or writes: parent=%#v child=%#v writes=%d/%d %d/%d", afterBrand, afterDownloads, afterBrandPatches, afterBrandAsyncUpdates, afterChildPatches, afterChildAsyncUpdates)
	}
	for _, id := range []string{privateFile.ID, publicFile.ID} {
		if err := clients.Files.Delete(ctx, id); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []string{downloads.ID, brand.ID} {
		if err := clients.FileFolders.Delete(ctx, id); err != nil {
			t.Fatal(err)
		}
	}
	record, err := execute(ctx, "verify-files-terminal", ids, clients)
	if err != nil || strings.Contains(record, brand.ID) || strings.Contains(record, publicFile.ID) || !strings.Contains(record, `"active_owned_files":0`) || !strings.Contains(record, `"active_owned_folders":0`) {
		t.Fatalf("terminal record = %q, %v", record, err)
	}
}

func TestExecuteRejectsFolderRepairWhenParentSearchOmitsExactChild(t *testing.T) {
	t.Setenv("HUBSPOT_NORTHSTAR_FILES_PREFIX", "ns_1a2b3c4d_o_")
	names, err := northstarFilesNamesFromEnvironment()
	if err != nil {
		t.Fatal(err)
	}
	fake := acceptance.NewFakeHubSpot("token", 123)
	blockedParentID := ""
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if blockedParentID != "" && request.Method == http.MethodGet && request.URL.Path == "/files/2026-03/folders/search" && request.URL.Query().Get("parentFolderId") == blockedParentID {
			writer.Header().Set("Content-Type", "application/json")
			if _, err := writer.Write([]byte(`{"results":[]}`)); err != nil {
				t.Error(err)
			}
			return
		}
		fake.ServeHTTP(writer, request)
	}))
	defer server.Close()
	origin, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	clients, err := hubspot.NewClientSet(hubspot.TransportConfig{BaseURL: origin, AccessToken: "token"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	brand, err := clients.FileFolders.Create(ctx, hubspot.FileFolderWrite{Name: names.BrandFolder})
	if err != nil {
		t.Fatal(err)
	}
	downloads, err := clients.FileFolders.Create(ctx, hubspot.FileFolderWrite{Name: names.DownloadsFolder, ParentFolderID: &brand.ID})
	if err != nil {
		t.Fatal(err)
	}
	privateFile, err := clients.Files.Upload(ctx, hubspot.FileUpload{Name: names.PrivateFile, FolderID: downloads.ID, Access: "PRIVATE", Bytes: []byte("Northstar private file\n")})
	if err != nil {
		t.Fatal(err)
	}
	publicFile, err := clients.Files.Upload(ctx, hubspot.FileUpload{Name: names.PublicFile, FolderID: downloads.ID, Access: "PUBLIC_NOT_INDEXABLE", Bytes: []byte("<svg xmlns=\"http://www.w3.org/2000/svg\" viewBox=\"0 0 1 1\"><path d=\"M0 0h1v1H0z\"/></svg>\n")})
	if err != nil {
		t.Fatal(err)
	}
	blockedParentID = brand.ID
	before, err := clients.FileFolders.Get(ctx, downloads.ID)
	if err != nil {
		t.Fatal(err)
	}
	beforeParent, err := clients.FileFolders.Get(ctx, brand.ID)
	if err != nil {
		t.Fatal(err)
	}
	parentPatches, parentAsyncUpdates := fake.FileFolderWriteCounts(brand.ID)
	childPatches, childAsyncUpdates := fake.FileFolderWriteCounts(downloads.ID)
	privateBefore, err := clients.Files.Get(ctx, privateFile.ID)
	if err != nil {
		t.Fatal(err)
	}
	publicBefore, err := clients.Files.Get(ctx, publicFile.ID)
	if err != nil {
		t.Fatal(err)
	}
	privatePatches, privateReplacements := fake.ManagedFileWriteCounts(privateFile.ID)
	publicPatches, publicReplacements := fake.ManagedFileWriteCounts(publicFile.ID)
	if _, err := execute(ctx, "repair-folder-path", []string{brand.ID, downloads.ID, privateFile.ID, publicFile.ID}, clients); err == nil || !strings.Contains(err.Error(), "identities") {
		t.Fatalf("folder repair without searched identity = %v", err)
	}
	after, err := clients.FileFolders.Get(ctx, downloads.ID)
	if err != nil {
		t.Fatal(err)
	}
	afterParent, err := clients.FileFolders.Get(ctx, brand.ID)
	if err != nil {
		t.Fatal(err)
	}
	afterParentPatches, afterParentAsyncUpdates := fake.FileFolderWriteCounts(brand.ID)
	afterChildPatches, afterChildAsyncUpdates := fake.FileFolderWriteCounts(downloads.ID)
	privateAfter, err := clients.Files.Get(ctx, privateFile.ID)
	if err != nil {
		t.Fatal(err)
	}
	publicAfter, err := clients.Files.Get(ctx, publicFile.ID)
	if err != nil {
		t.Fatal(err)
	}
	afterPrivatePatches, afterPrivateReplacements := fake.ManagedFileWriteCounts(privateFile.ID)
	afterPublicPatches, afterPublicReplacements := fake.ManagedFileWriteCounts(publicFile.ID)
	if !reflect.DeepEqual(after, before) || !reflect.DeepEqual(afterParent, beforeParent) || !reflect.DeepEqual(privateAfter, privateBefore) || !reflect.DeepEqual(publicAfter, publicBefore) || afterParentPatches != parentPatches || afterParentAsyncUpdates != parentAsyncUpdates || afterChildPatches != childPatches || afterChildAsyncUpdates != childAsyncUpdates || afterPrivatePatches != privatePatches || afterPrivateReplacements != privateReplacements || afterPublicPatches != publicPatches || afterPublicReplacements != publicReplacements {
		t.Fatalf("rejected folder repair changed state or writes: parent=%#v child=%#v writes=%d/%d %d/%d", afterParent, after, afterParentPatches, afterParentAsyncUpdates, afterChildPatches, afterChildAsyncUpdates)
	}
}

func TestExecuteRejectsUnconvergedFileStagingBeforeFolderDrift(t *testing.T) {
	t.Setenv("HUBSPOT_NORTHSTAR_FILES_PREFIX", "ns_1a2b3c4d_o_")
	names, err := northstarFilesNamesFromEnvironment()
	if err != nil {
		t.Fatal(err)
	}
	fake := acceptance.NewFakeHubSpot("token", 123)
	server := httptest.NewServer(fake)
	defer server.Close()
	origin, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	clients, err := hubspot.NewClientSet(hubspot.TransportConfig{BaseURL: origin, AccessToken: "token"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	brand, err := clients.FileFolders.Create(ctx, hubspot.FileFolderWrite{Name: names.BrandFolder})
	if err != nil {
		t.Fatal(err)
	}
	downloads, err := clients.FileFolders.Create(ctx, hubspot.FileFolderWrite{Name: names.DownloadsFolder, ParentFolderID: &brand.ID})
	if err != nil {
		t.Fatal(err)
	}
	privateFile, err := clients.Files.Upload(ctx, hubspot.FileUpload{
		Name: names.PrivateFile, FolderID: brand.ID, Access: "PRIVATE", Bytes: []byte("Northstar private file\n"),
	})
	if err != nil {
		t.Fatal(err)
	}
	originalReadDelays := northstarFilesConvergenceDelays
	northstarFilesConvergenceDelays = []time.Duration{0}
	t.Cleanup(func() { northstarFilesConvergenceDelays = originalReadDelays })
	fake.LagNextManagedFileMoveVisibility(2, 0)
	if _, err := execute(ctx, "stage-file-for-folder-rename", []string{privateFile.ID, brand.ID, downloads.ID}, clients); err == nil || !strings.Contains(err.Error(), "could not be verified") {
		t.Fatalf("unconverged staging error = %v", err)
	}
	currentBrand, err := clients.FileFolders.Get(ctx, brand.ID)
	if err != nil || currentBrand.Name != names.BrandFolder {
		t.Fatalf("failed staging changed the parent folder = %#v, %v", currentBrand, err)
	}
}

func TestNorthstarFilesPrefixRejectsUnboundedNames(t *testing.T) {
	t.Setenv("HUBSPOT_NORTHSTAR_FILES_PREFIX", "unsafe-prefix")
	if _, err := northstarFilesNamesFromEnvironment(); err == nil {
		t.Fatal("unsafe Northstar Files prefix accepted")
	}
}

func TestExecuteRejectsInvalidFolderRepairIdentitiesBeforeMutation(t *testing.T) {
	testCases := map[string]func(northstarFilesIDs, string) []string{
		"invalid parent": func(ids northstarFilesIDs, _ string) []string {
			return []string{"99999", ids.DownloadsFolder, ids.PrivateFile, ids.PublicFile}
		},
		"invalid child": func(ids northstarFilesIDs, _ string) []string {
			return []string{ids.BrandFolder, "99999", ids.PrivateFile, ids.PublicFile}
		},
		"invalid private file": func(ids northstarFilesIDs, foreign string) []string {
			return []string{ids.BrandFolder, ids.DownloadsFolder, foreign, ids.PublicFile}
		},
		"invalid public file": func(ids northstarFilesIDs, foreign string) []string {
			return []string{ids.BrandFolder, ids.DownloadsFolder, ids.PrivateFile, foreign}
		},
		"swapped folders": func(ids northstarFilesIDs, _ string) []string {
			return []string{ids.DownloadsFolder, ids.BrandFolder, ids.PrivateFile, ids.PublicFile}
		},
		"swapped files": func(ids northstarFilesIDs, _ string) []string {
			return []string{ids.BrandFolder, ids.DownloadsFolder, ids.PublicFile, ids.PrivateFile}
		},
		"wrong arity": func(ids northstarFilesIDs, _ string) []string {
			return []string{ids.BrandFolder, ids.DownloadsFolder, ids.PrivateFile}
		},
		"excess arity": func(ids northstarFilesIDs, foreign string) []string {
			return []string{ids.BrandFolder, ids.DownloadsFolder, ids.PrivateFile, ids.PublicFile, foreign}
		},
	}
	for name, inputs := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Setenv("HUBSPOT_NORTHSTAR_FILES_PREFIX", "ns_1a2b3c4d_o_")
			names, err := northstarFilesNamesFromEnvironment()
			if err != nil {
				t.Fatal(err)
			}
			fake := acceptance.NewFakeHubSpot("token", 123)
			server := httptest.NewServer(fake)
			defer server.Close()
			origin, err := url.Parse(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			clients, err := hubspot.NewClientSet(hubspot.TransportConfig{BaseURL: origin, AccessToken: "token"})
			if err != nil {
				t.Fatal(err)
			}
			ctx := context.Background()
			brand, err := clients.FileFolders.Create(ctx, hubspot.FileFolderWrite{Name: names.BrandFolder})
			if err != nil {
				t.Fatal(err)
			}
			downloads, err := clients.FileFolders.Create(ctx, hubspot.FileFolderWrite{Name: names.DownloadsFolder, ParentFolderID: &brand.ID})
			if err != nil {
				t.Fatal(err)
			}
			privateFile, err := clients.Files.Upload(ctx, hubspot.FileUpload{Name: names.PrivateFile, FolderID: downloads.ID, Access: "PRIVATE", Bytes: []byte("Northstar private file\n")})
			if err != nil {
				t.Fatal(err)
			}
			publicFile, err := clients.Files.Upload(ctx, hubspot.FileUpload{Name: names.PublicFile, FolderID: downloads.ID, Access: "PUBLIC_NOT_INDEXABLE", Bytes: []byte("<svg xmlns=\"http://www.w3.org/2000/svg\" viewBox=\"0 0 1 1\"><path d=\"M0 0h1v1H0z\"/></svg>\n")})
			if err != nil {
				t.Fatal(err)
			}
			foreign, err := clients.Files.Upload(ctx, hubspot.FileUpload{Name: "foreign.txt", FolderID: downloads.ID, Access: "PRIVATE", Bytes: []byte("foreign\n")})
			if err != nil {
				t.Fatal(err)
			}
			ids := northstarFilesIDs{BrandFolder: brand.ID, DownloadsFolder: downloads.ID, PrivateFile: privateFile.ID, PublicFile: publicFile.ID}
			before := []any{brand, downloads, privateFile, publicFile, foreign}
			parentPatches, parentAsync := fake.FileFolderWriteCounts(brand.ID)
			childPatches, childAsync := fake.FileFolderWriteCounts(downloads.ID)
			privatePatches, privateReplacements := fake.ManagedFileWriteCounts(privateFile.ID)
			publicPatches, publicReplacements := fake.ManagedFileWriteCounts(publicFile.ID)
			foreignPatches, foreignReplacements := fake.ManagedFileWriteCounts(foreign.ID)
			if _, err := execute(ctx, "repair-folder-path", inputs(ids, foreign.ID), clients); err == nil {
				t.Fatal("invalid folder-repair identities were accepted")
			}
			afterBrand, brandErr := clients.FileFolders.Get(ctx, brand.ID)
			afterDownloads, downloadsErr := clients.FileFolders.Get(ctx, downloads.ID)
			afterPrivate, privateErr := clients.Files.Get(ctx, privateFile.ID)
			afterPublic, publicErr := clients.Files.Get(ctx, publicFile.ID)
			afterForeign, foreignErr := clients.Files.Get(ctx, foreign.ID)
			if brandErr != nil || downloadsErr != nil || privateErr != nil || publicErr != nil || foreignErr != nil || !reflect.DeepEqual([]any{afterBrand, afterDownloads, afterPrivate, afterPublic, afterForeign}, before) {
				t.Fatalf("rejected repair changed objects: %#v %#v %#v %#v %#v", afterBrand, afterDownloads, afterPrivate, afterPublic, afterForeign)
			}
			afterParentPatches, afterParentAsync := fake.FileFolderWriteCounts(brand.ID)
			afterChildPatches, afterChildAsync := fake.FileFolderWriteCounts(downloads.ID)
			afterPrivatePatches, afterPrivateReplacements := fake.ManagedFileWriteCounts(privateFile.ID)
			afterPublicPatches, afterPublicReplacements := fake.ManagedFileWriteCounts(publicFile.ID)
			afterForeignPatches, afterForeignReplacements := fake.ManagedFileWriteCounts(foreign.ID)
			if afterParentPatches != parentPatches || afterParentAsync != parentAsync || afterChildPatches != childPatches || afterChildAsync != childAsync || afterPrivatePatches != privatePatches || afterPrivateReplacements != privateReplacements || afterPublicPatches != publicPatches || afterPublicReplacements != publicReplacements || afterForeignPatches != foreignPatches || afterForeignReplacements != foreignReplacements {
				t.Fatal("rejected repair sent a write")
			}
		})
	}
}

func TestExecuteRejectsUnexpectedFolderRepairTopologyBeforeMutation(t *testing.T) {
	testCases := map[string]func(context.Context, *hubspot.ClientSet, hubspot.FileFolder, hubspot.FileFolder, northstarFilesNames) (hubspot.FileFolder, error){
		"sibling": func(ctx context.Context, clients *hubspot.ClientSet, brand, _ hubspot.FileFolder, names northstarFilesNames) (hubspot.FileFolder, error) {
			return clients.FileFolders.Create(ctx, hubspot.FileFolderWrite{Name: names.DownloadsFolderRepair, ParentFolderID: &brand.ID})
		},
		"descendant": func(ctx context.Context, clients *hubspot.ClientSet, _, downloads hubspot.FileFolder, _ northstarFilesNames) (hubspot.FileFolder, error) {
			return clients.FileFolders.Create(ctx, hubspot.FileFolderWrite{Name: "unowned", ParentFolderID: &downloads.ID})
		},
	}
	for name, addUnexpected := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Setenv("HUBSPOT_NORTHSTAR_FILES_PREFIX", "ns_1a2b3c4d_o_")
			names, err := northstarFilesNamesFromEnvironment()
			if err != nil {
				t.Fatal(err)
			}
			fake := acceptance.NewFakeHubSpot("token", 123)
			server := httptest.NewServer(fake)
			defer server.Close()
			origin, err := url.Parse(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			clients, err := hubspot.NewClientSet(hubspot.TransportConfig{BaseURL: origin, AccessToken: "token"})
			if err != nil {
				t.Fatal(err)
			}
			ctx := context.Background()
			brand, err := clients.FileFolders.Create(ctx, hubspot.FileFolderWrite{Name: names.BrandFolder})
			if err != nil {
				t.Fatal(err)
			}
			downloads, err := clients.FileFolders.Create(ctx, hubspot.FileFolderWrite{Name: names.DownloadsFolder, ParentFolderID: &brand.ID})
			if err != nil {
				t.Fatal(err)
			}
			privateFile, err := clients.Files.Upload(ctx, hubspot.FileUpload{Name: names.PrivateFile, FolderID: downloads.ID, Access: "PRIVATE", Bytes: []byte("Northstar private file\n")})
			if err != nil {
				t.Fatal(err)
			}
			publicFile, err := clients.Files.Upload(ctx, hubspot.FileUpload{Name: names.PublicFile, FolderID: downloads.ID, Access: "PUBLIC_NOT_INDEXABLE", Bytes: []byte("<svg xmlns=\"http://www.w3.org/2000/svg\" viewBox=\"0 0 1 1\"><path d=\"M0 0h1v1H0z\"/></svg>\n")})
			if err != nil {
				t.Fatal(err)
			}
			unexpected, err := addUnexpected(ctx, clients, brand, downloads, names)
			if err != nil {
				t.Fatal(err)
			}
			beforeUnexpected := unexpected
			beforeBrand, beforeDownloads := brand, downloads
			parentPatches, parentAsync := fake.FileFolderWriteCounts(brand.ID)
			childPatches, childAsync := fake.FileFolderWriteCounts(downloads.ID)
			privatePatches, privateReplacements := fake.ManagedFileWriteCounts(privateFile.ID)
			publicPatches, publicReplacements := fake.ManagedFileWriteCounts(publicFile.ID)
			if _, err := execute(ctx, "repair-folder-path", []string{brand.ID, downloads.ID, privateFile.ID, publicFile.ID}, clients); err == nil {
				t.Fatal("unexpected folder-repair topology was accepted")
			}
			afterUnexpected, err := clients.FileFolders.Get(ctx, unexpected.ID)
			if err != nil || !reflect.DeepEqual(afterUnexpected, beforeUnexpected) {
				t.Fatalf("rejected repair changed unexpected folder = %#v, %v", afterUnexpected, err)
			}
			afterBrand, brandErr := clients.FileFolders.Get(ctx, brand.ID)
			afterDownloads, downloadsErr := clients.FileFolders.Get(ctx, downloads.ID)
			if brandErr != nil || downloadsErr != nil || !reflect.DeepEqual(afterBrand, beforeBrand) || !reflect.DeepEqual(afterDownloads, beforeDownloads) {
				t.Fatalf("rejected repair changed owned folders = %#v %#v", afterBrand, afterDownloads)
			}
			afterParentPatches, afterParentAsync := fake.FileFolderWriteCounts(brand.ID)
			afterChildPatches, afterChildAsync := fake.FileFolderWriteCounts(downloads.ID)
			afterPrivatePatches, afterPrivateReplacements := fake.ManagedFileWriteCounts(privateFile.ID)
			afterPublicPatches, afterPublicReplacements := fake.ManagedFileWriteCounts(publicFile.ID)
			if afterParentPatches != parentPatches || afterParentAsync != parentAsync || afterChildPatches != childPatches || afterChildAsync != childAsync || afterPrivatePatches != privatePatches || afterPrivateReplacements != privateReplacements || afterPublicPatches != publicPatches || afterPublicReplacements != publicReplacements {
				t.Fatal("rejected topology sent a write")
			}
		})
	}
}

func TestNorthstarFilesRunNamesFitSearchLimit(t *testing.T) {
	t.Setenv("HUBSPOT_NORTHSTAR_FILES_PREFIX", "ns_1a2b3c4d_o_")
	names, err := northstarFilesNamesFromEnvironment()
	if err != nil {
		t.Fatal(err)
	}
	for label, name := range map[string]string{
		"brand": names.BrandFolder, "brand refresh": names.BrandFolderRefresh, "downloads": names.DownloadsFolder, "downloads repair": names.DownloadsFolderRepair,
		"private file": names.PrivateFile, "public file": names.PublicFile, "drifted public file": names.PublicFileDrift,
	} {
		if len(name) > 19 {
			t.Fatalf("%s name %q exceeds the live Files search limit", label, name)
		}
	}
}

func TestWaitForNorthstarDescendantPathConvergence(t *testing.T) {
	var totalDelay time.Duration
	for _, delay := range northstarDescendantPathConvergenceDelays {
		totalDelay += delay
	}
	if totalDelay <= 87*time.Second {
		t.Fatalf("descendant path convergence delay = %s, want more than live-observed 87s", totalDelay)
	}
	originalDelays := northstarDescendantPathConvergenceDelays
	northstarDescendantPathConvergenceDelays = make([]time.Duration, len(originalDelays))
	t.Cleanup(func() { northstarDescendantPathConvergenceDelays = originalDelays })

	attempts := 0
	searches := 0
	folder, err := waitForNorthstarDescendantPath(context.Background(), func(context.Context, string) (hubspot.FileFolder, error) {
		attempts++
		path := "/old/child"
		if attempts == len(northstarDescendantPathConvergenceDelays) {
			path = "/current/child"
		}
		return hubspot.FileFolder{ID: "11", Path: path}, nil
	}, func(context.Context, *string, string) ([]hubspot.FileFolder, error) {
		searches++
		return nil, nil
	}, "10", "11", func(folder hubspot.FileFolder) bool {
		return folder.Path == "/current/child"
	})
	if err != nil || attempts != len(northstarDescendantPathConvergenceDelays) || searches != attempts-1 || folder.Path != "/current/child" {
		t.Fatalf("folder convergence = %#v after %d reads and %d searches, %v", folder, attempts, searches, err)
	}

	attempts = 0
	searches = 0
	_, err = waitForNorthstarDescendantPath(context.Background(), func(context.Context, string) (hubspot.FileFolder, error) {
		attempts++
		return hubspot.FileFolder{ID: "11", Path: "/old/child"}, nil
	}, func(_ context.Context, parentID *string, _ string) ([]hubspot.FileFolder, error) {
		searches++
		if parentID == nil || *parentID != "10" {
			t.Fatalf("search parent = %v", parentID)
		}
		return []hubspot.FileFolder{{ID: "12", Path: "/current/child"}, {ID: "11", Path: "/old/child"}}, nil
	}, "10", "11", func(folder hubspot.FileFolder) bool {
		return folder.Path == "/current/child"
	})
	if !errors.Is(err, errNorthstarFolderReadBack) || attempts != len(northstarDescendantPathConvergenceDelays) || searches != attempts {
		t.Fatalf("folder exhaustion after %d reads and %d searches = %v", attempts, searches, err)
	}

	attempts = 0
	searches = 0
	folder, err = waitForNorthstarDescendantPath(context.Background(), func(context.Context, string) (hubspot.FileFolder, error) {
		attempts++
		return hubspot.FileFolder{ID: "11", Path: "/old/child"}, nil
	}, func(context.Context, *string, string) ([]hubspot.FileFolder, error) {
		searches++
		return []hubspot.FileFolder{{ID: "11", Path: "/current/child"}}, nil
	}, "10", "11", func(folder hubspot.FileFolder) bool {
		return folder.Path == "/current/child"
	})
	if err != nil || attempts != 1 || searches != 1 || folder.ID != "11" || folder.Path != "/current/child" {
		t.Fatalf("search read-back = %#v after %d reads and %d searches, %v", folder, attempts, searches, err)
	}

	searches = 0
	_, err = waitForNorthstarDescendantPath(context.Background(), func(context.Context, string) (hubspot.FileFolder, error) {
		return hubspot.FileFolder{ID: "12", Path: "/old/child"}, nil
	}, func(context.Context, *string, string) ([]hubspot.FileFolder, error) {
		searches++
		return []hubspot.FileFolder{{ID: "11", Path: "/current/child"}}, nil
	}, "10", "11", func(folder hubspot.FileFolder) bool {
		return folder.Path == "/current/child"
	})
	if err == nil || !strings.Contains(err.Error(), "identity") || searches != 0 {
		t.Fatalf("mismatched GET identity with %d searches = %v", searches, err)
	}
}

func northstarFormFixture() hubspot.FormDefinitionWrite {
	return hubspot.FormDefinitionWrite{
		FormType: "hubspot", Name: "ns_contact_us",
		FieldGroups: []hubspot.FormFieldGroup{{GroupType: "default_group", RichTextType: "text", Fields: []hubspot.FormField{{
			ObjectTypeID: "0-1", Name: "email", DependentFields: []hubspot.FormDependentField{}, Label: "Work email",
			FieldType: "email", Required: true, Description: "Contact email", Placeholder: "you@company.example",
			Validation: hubspot.FormFieldValidation{BlockedEmailDomains: []string{}, UseDefaultBlockList: true},
		}}}},
		Configuration: hubspot.FormConfiguration{Editable: true, Language: "en", Cloneable: true, RecaptchaEnabled: true, Archivable: true,
			PostSubmitAction: hubspot.FormPostSubmitAction{Type: "thank_you", Value: "Thank you"}, NotifyRecipients: []string{}},
		DisplayOptions: hubspot.FormDisplayOptions{Theme: "default_style", SubmitButtonText: "Contact Northstar", Style: hubspot.FormStyle{
			LabelTextSize: "13px", LabelTextColor: "#33475b", LegalConsentTextSize: "12px", LegalConsentTextColor: "#33475b",
			HelpTextSize: "11px", HelpTextColor: "#516f90", FontFamily: "Arial, sans-serif", BackgroundWidth: "100%",
			SubmitFontColor: "#ffffff", SubmitAlignment: "center", SubmitSize: "12px 24px", SubmitColor: "#00a4bd",
		}},
		LegalConsentOptions: hubspot.FormLegalConsentOptions{Type: "none"},
	}
}
