// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

//go:build acceptance

package main

import (
	"context"
	"errors"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/jackemcpherson/terraform-provider-hubspot/internal/acceptance"
	"github.com/jackemcpherson/terraform-provider-hubspot/internal/hubspot"
)

const northstarTestMembershipEmail = "tfhs-probe-16-20260802024807@example.com"

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
	if _, err := execute(ctx, "drift-folder-path", []string{brand.ID, downloads.ID}, clients); err != nil {
		t.Fatal(err)
	}
	driftedDownloads, err := clients.FileFolders.Get(ctx, downloads.ID)
	if err != nil || driftedDownloads.Path != "/"+names.BrandFolderRefresh+"/"+names.DownloadsFolder {
		t.Fatalf("drifted folder = %#v, %v", driftedDownloads, err)
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

func TestNorthstarFilesRunNamesFitSearchLimit(t *testing.T) {
	t.Setenv("HUBSPOT_NORTHSTAR_FILES_PREFIX", "ns_1a2b3c4d_o_")
	names, err := northstarFilesNamesFromEnvironment()
	if err != nil {
		t.Fatal(err)
	}
	for label, name := range map[string]string{
		"brand": names.BrandFolder, "brand refresh": names.BrandFolderRefresh, "downloads": names.DownloadsFolder,
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
	if totalDelay <= 32*time.Second {
		t.Fatalf("descendant path convergence delay = %s, want more than live-observed 32s", totalDelay)
	}
	originalDelays := northstarDescendantPathConvergenceDelays
	northstarDescendantPathConvergenceDelays = make([]time.Duration, len(originalDelays))
	t.Cleanup(func() { northstarDescendantPathConvergenceDelays = originalDelays })

	attempts := 0
	folder, err := waitForNorthstarDescendantPath(context.Background(), func(context.Context, string) (hubspot.FileFolder, error) {
		attempts++
		path := "/old/child"
		if attempts == len(northstarDescendantPathConvergenceDelays) {
			path = "/current/child"
		}
		return hubspot.FileFolder{ID: "11", Path: path}, nil
	}, "11", func(folder hubspot.FileFolder) bool {
		return folder.Path == "/current/child"
	})
	if err != nil || attempts != len(northstarDescendantPathConvergenceDelays) || folder.Path != "/current/child" {
		t.Fatalf("folder convergence = %#v after %d attempts, %v", folder, attempts, err)
	}

	attempts = 0
	_, err = waitForNorthstarDescendantPath(context.Background(), func(context.Context, string) (hubspot.FileFolder, error) {
		attempts++
		return hubspot.FileFolder{ID: "11", Path: "/old/child"}, nil
	}, "11", func(folder hubspot.FileFolder) bool {
		return folder.Path == "/current/child"
	})
	if !errors.Is(err, errNorthstarFolderReadBack) || attempts != len(northstarDescendantPathConvergenceDelays) {
		t.Fatalf("folder exhaustion after %d attempts = %v", attempts, err)
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
