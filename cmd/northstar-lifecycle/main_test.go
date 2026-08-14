// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

//go:build acceptance

package main

import (
	"context"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/jackemcpherson/terraform-provider-hubspot/internal/acceptance"
	"github.com/jackemcpherson/terraform-provider-hubspot/internal/hubspot"
)

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

func TestExecuteCleansInterruptedNorthstarLifecycle(t *testing.T) {
	t.Setenv("HUBSPOT_NORTHSTAR_FILES_PREFIX", "ns_1a2b3c4d_o_")
	names, err := northstarFilesNamesFromEnvironment()
	if err != nil {
		t.Fatal(err)
	}
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
	if err := verifyNorthstarCleanup(ctx, clients, names); err != nil {
		t.Fatal(err)
	}
}

func TestExecuteCleanupRejectsUnexpectedPrefixBeforeMutation(t *testing.T) {
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

func TestExecuteManagesNorthstarFilesLifecycle(t *testing.T) {
	t.Setenv("HUBSPOT_NORTHSTAR_FILES_PREFIX", "ns_1a2b3c4d_o_")
	names, err := northstarFilesNamesFromEnvironment()
	if err != nil {
		t.Fatal(err)
	}
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
	if _, err := execute(ctx, "stage-file-for-folder-rename", []string{privateFile.ID, brand.ID, downloads.ID}, clients); err != nil {
		t.Fatal(err)
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

func TestWaitForNorthstarFolderConvergence(t *testing.T) {
	attempts := 0
	folder, err := waitForNorthstarFolder(context.Background(), func(context.Context, string) (hubspot.FileFolder, error) {
		attempts++
		path := "/old/child"
		if attempts == 3 {
			path = "/current/child"
		}
		return hubspot.FileFolder{ID: "11", Path: path}, nil
	}, "11", func(folder hubspot.FileFolder) bool {
		return folder.Path == "/current/child"
	}, []time.Duration{0, 0, 0})
	if err != nil || attempts != 3 || folder.Path != "/current/child" {
		t.Fatalf("folder convergence = %#v after %d attempts, %v", folder, attempts, err)
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
