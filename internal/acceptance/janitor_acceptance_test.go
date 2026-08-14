// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

//go:build acceptance

package acceptance_test

import (
	"context"
	"errors"
	"fmt"
	"net/http/httptest"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackemcpherson/terraform-provider-hubspot/internal/acceptance"
	"github.com/jackemcpherson/terraform-provider-hubspot/internal/hubspot"
)

func TestAcc_JanitorReport(t *testing.T) {
	clients, shard := janitorClients(t)
	prefix := requiredEnvironment(t, "HUBSPOT_ACCEPTANCE_PREFIX")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	switch shard {
	case "free_properties":
		propertyCount, groupCount := countFreeOwnedConfiguration(t, ctx, clients, prefix)
		t.Logf("stale owned CRM configuration: property_definitions=%d property_groups=%d", propertyCount, groupCount)
	case "deal_pipelines":
		t.Logf("stale owned CRM configuration: deal_pipelines=%d", countOwnedPipelines(t, ctx, clients, "deals", prefix))
	case "ticket_pipelines":
		t.Logf("stale owned CRM configuration: ticket_pipelines=%d", countOwnedPipelines(t, ctx, clients, "tickets", prefix))
	case "form_definitions":
		active, archived, err := countOwnedForms(ctx, clients, prefix)
		if err != nil {
			t.Fatalf("report stale owned Form definitions: %s", acceptance.SanitizedHubSpotError(err))
		}
		t.Logf("stale owned HubSpot configuration: active_form_definitions=%d retained_archived_form_definitions=%d", active, archived)
	case "files_configuration":
		files, folders, err := acceptance.CountActiveFilesConfiguration(ctx, clients, prefix)
		if err != nil {
			t.Fatalf("report stale active Files configuration: %s", acceptance.SanitizedHubSpotError(err))
		}
		t.Logf("stale owned HubSpot configuration: active_managed_files=%d active_file_folders=%d", files, folders)
	case "account_memberships":
		memberships, err := countOwnedAccountMemberships(ctx, clients, prefix)
		if err != nil {
			t.Fatalf("report stale owned account memberships: %s", acceptance.SanitizedHubSpotError(err))
		}
		t.Logf("stale owned HubSpot configuration: active_account_memberships=%d", memberships)
	}
}

func TestAcc_ManualPrefixCleanup(t *testing.T) {
	clients, shard := janitorClients(t)
	prefix := requiredEnvironment(t, "HUBSPOT_ACCEPTANCE_PREFIX")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if shard == "account_memberships" {
		if err := deleteOwnedAccountMemberships(ctx, clients, prefix); err != nil {
			t.Fatalf("delete owned account memberships: %s", acceptance.SanitizedHubSpotError(err))
		}
		t.Log("account membership cleanup verified exact Settings ID and email absence")
		return
	}
	if shard == "files_configuration" {
		if err := deleteOwnedFilesConfiguration(ctx, clients, prefix); err != nil {
			t.Fatalf("delete owned Files configuration: %s", acceptance.SanitizedHubSpotError(err))
		}
		t.Log("Files cleanup verified zero active owned configuration; HubSpot-managed Trash retention is expected")
		return
	}
	if shard == "form_definitions" {
		archived, err := archiveOwnedForms(ctx, clients, prefix)
		if err != nil {
			t.Fatalf("archive owned Form definitions: %s", acceptance.SanitizedHubSpotError(err))
		}
		t.Logf("terminal cleanup retained Archived form definitions: %d", archived)
		return
	}

	if shard == "deal_pipelines" || shard == "ticket_pipelines" {
		objectType := "deals"
		if shard == "ticket_pipelines" {
			objectType = "tickets"
		}
		pipelines, err := clients.Pipelines.List(ctx, objectType)
		if err != nil {
			t.Fatalf("list pipelines for manual cleanup: %s", acceptance.SanitizedHubSpotError(err))
		}
		for _, pipeline := range pipelines {
			if !pipeline.Archived && strings.HasPrefix(pipeline.Label, prefix) {
				if err := clients.Pipelines.Archive(ctx, objectType, pipeline.ID); err != nil {
					t.Fatalf("archive owned pipeline during manual cleanup: %s", acceptance.SanitizedHubSpotError(err))
				}
			}
		}
		if countOwnedPipelines(t, ctx, clients, objectType, prefix) != 0 {
			t.Fatal("manual cleanup could not verify absence of all active prefixed pipelines")
		}
		return
	}

	properties, err := clients.Properties.List(ctx, "contacts", false, "non_sensitive", "")
	if err != nil {
		t.Fatalf("list property definitions for manual cleanup: %s", acceptance.SanitizedHubSpotError(err))
	}
	for _, property := range properties {
		if strings.HasPrefix(property.Name, prefix) {
			if err := clients.Properties.Archive(ctx, "contacts", property.Name); err != nil {
				t.Fatalf("archive owned property definition during manual cleanup: %s", acceptance.SanitizedHubSpotError(err))
			}
		}
	}

	groups, err := clients.PropertyGroups.List(ctx, "contacts")
	if err != nil {
		t.Fatalf("list property groups for manual cleanup: %s", acceptance.SanitizedHubSpotError(err))
	}
	for _, group := range groups {
		if strings.HasPrefix(group.Name, prefix) {
			if err := clients.PropertyGroups.Archive(ctx, "contacts", group.Name); err != nil {
				t.Fatalf("archive owned property group during manual cleanup: %s", acceptance.SanitizedHubSpotError(err))
			}
		}
	}

	propertyCount, groupCount := countFreeOwnedConfiguration(t, ctx, clients, prefix)
	if propertyCount != 0 || groupCount != 0 {
		t.Fatal("manual cleanup could not verify absence of all prefixed CRM configuration")
	}
}

func TestFilesJanitorReportsAndDeletesOwnedConfigurationLeafFirst(t *testing.T) {
	fake := acceptance.NewFakeHubSpot("token", 123)
	server := httptest.NewServer(fake)
	defer server.Close()
	origin, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	clients, err := hubspot.NewClientSet(hubspot.TransportConfig{BaseURL: origin, AccessToken: "token", UserAgent: "files-janitor-test"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	unowned, err := clients.FileFolders.Create(ctx, hubspot.FileFolderWrite{Name: "keep"})
	if err != nil {
		t.Fatal(err)
	}
	root, err := clients.FileFolders.Create(ctx, hubspot.FileFolderWrite{Name: "tf_acc_owned_root"})
	if err != nil {
		t.Fatal(err)
	}
	child, err := clients.FileFolders.Create(ctx, hubspot.FileFolderWrite{Name: "tf_acc_owned_child", ParentFolderID: &root.ID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := clients.Files.Upload(ctx, hubspot.FileUpload{Name: "tf_acc_owned_file.txt", FolderID: child.ID, Access: "PRIVATE", Bytes: []byte("owned")}); err != nil {
		t.Fatal(err)
	}
	if _, err := clients.Files.Upload(ctx, hubspot.FileUpload{Name: "keep.txt", FolderID: unowned.ID, Access: "PRIVATE", Bytes: []byte("unowned")}); err != nil {
		t.Fatal(err)
	}
	files, folders, err := acceptance.CountActiveFilesConfiguration(ctx, clients, "tf_acc_owned_")
	if err != nil || files != 1 || folders != 2 {
		t.Fatalf("owned Files report = files %d, folders %d, error %v", files, folders, err)
	}
	if err := deleteOwnedFilesConfiguration(ctx, clients, "tf_acc_owned_"); err != nil {
		t.Fatal(err)
	}
	files, folders, err = acceptance.CountActiveFilesConfiguration(ctx, clients, "tf_acc_owned_")
	if err != nil || files != 0 || folders != 0 {
		t.Fatalf("owned Files cleanup = files %d, folders %d, error %v", files, folders, err)
	}
	if _, err := clients.FileFolders.Get(ctx, unowned.ID); err != nil {
		t.Fatal("Files cleanup mutated an unowned folder")
	}
}

func TestFilesJanitorFailsClosedOnUnownedFolderContents(t *testing.T) {
	fake := acceptance.NewFakeHubSpot("token", 123)
	server := httptest.NewServer(fake)
	defer server.Close()
	origin, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	clients, err := hubspot.NewClientSet(hubspot.TransportConfig{BaseURL: origin, AccessToken: "token", UserAgent: "files-janitor-failure-test"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	root, err := clients.FileFolders.Create(ctx, hubspot.FileFolderWrite{Name: "tf_acc_owned_root"})
	if err != nil {
		t.Fatal(err)
	}
	ownedFile, err := clients.Files.Upload(ctx, hubspot.FileUpload{Name: "tf_acc_owned_file.txt", FolderID: root.ID, Access: "PRIVATE", Bytes: []byte("owned")})
	if err != nil {
		t.Fatal(err)
	}
	unownedFile, err := clients.Files.Upload(ctx, hubspot.FileUpload{Name: "keep.txt", FolderID: root.ID, Access: "PRIVATE", Bytes: []byte("unowned")})
	if err != nil {
		t.Fatal(err)
	}
	if err := deleteOwnedFilesConfiguration(ctx, clients, "tf_acc_owned_"); err == nil {
		t.Fatal("Files cleanup accepted an owned folder containing unowned configuration")
	}
	if _, err := clients.Files.Get(ctx, ownedFile.ID); err != nil {
		t.Fatal("failed Files cleanup partially mutated owned configuration")
	}
	if _, err := clients.Files.Get(ctx, unownedFile.ID); err != nil {
		t.Fatal("failed Files cleanup mutated unowned configuration")
	}
}

func TestAccountMembershipJanitorReportsAndDeletesOnlyExactOwnedIdentities(t *testing.T) {
	fake := acceptance.NewFakeHubSpot("token", 123)
	server := httptest.NewServer(fake)
	defer server.Close()
	origin, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	clients, err := hubspot.NewClientSet(hubspot.TransportConfig{BaseURL: origin, AccessToken: "token", UserAgent: "membership-janitor-test"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	owned, err := clients.AccountMemberships.Create(ctx, hubspot.AccountMembershipCreate{Email: "tf_acc_owned_operator@example.com", SendWelcomeEmail: false})
	if err != nil {
		t.Fatal(err)
	}
	unowned, err := clients.AccountMemberships.Create(ctx, hubspot.AccountMembershipCreate{Email: "keep@example.com", SendWelcomeEmail: false})
	if err != nil {
		t.Fatal(err)
	}
	count, err := countOwnedAccountMemberships(ctx, clients, "tf_acc_owned_")
	if err != nil || count != 1 {
		t.Fatalf("owned account membership report = %d, %v", count, err)
	}
	if err := deleteOwnedAccountMemberships(ctx, clients, "tf_acc_owned_"); err != nil {
		t.Fatal(err)
	}
	if _, err := clients.AccountMemberships.GetByID(ctx, owned.ID); !formJanitorNotFound(err) {
		t.Fatal("account membership cleanup retained the owned identity")
	}
	if _, err := clients.AccountMemberships.GetByID(ctx, unowned.ID); err != nil {
		t.Fatal("account membership cleanup mutated an unowned identity")
	}
}

func TestAccountMembershipJanitorRefusesSuperAdminBeforeMutation(t *testing.T) {
	fake := acceptance.NewFakeHubSpot("token", 123)
	server := httptest.NewServer(fake)
	defer server.Close()
	origin, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	clients, err := hubspot.NewClientSet(hubspot.TransportConfig{BaseURL: origin, AccessToken: "token", UserAgent: "membership-janitor-guard-test"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	ordinary, err := clients.AccountMemberships.Create(ctx, hubspot.AccountMembershipCreate{Email: "tf_acc_owned_ordinary@example.com", SendWelcomeEmail: false})
	if err != nil {
		t.Fatal(err)
	}
	admin, err := clients.AccountMemberships.Create(ctx, hubspot.AccountMembershipCreate{Email: "tf_acc_owned_admin@example.com", SendWelcomeEmail: false})
	if err != nil {
		t.Fatal(err)
	}
	fake.SetAccountMembershipSuperAdmin(admin.ID, true)
	if err := deleteOwnedAccountMemberships(ctx, clients, "tf_acc_owned_"); err == nil {
		t.Fatal("account membership cleanup accepted a Super Admin")
	}
	for _, id := range []string{ordinary.ID, admin.ID} {
		if _, err := clients.AccountMemberships.GetByID(ctx, id); err != nil {
			t.Fatal("failed account membership cleanup partially mutated an owned identity")
		}
	}
}

func TestAccountMembershipJanitorRefusesInvalidDomainBeforeMutation(t *testing.T) {
	fake := acceptance.NewFakeHubSpot("token", 123)
	server := httptest.NewServer(fake)
	defer server.Close()
	origin, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	clients, err := hubspot.NewClientSet(hubspot.TransportConfig{BaseURL: origin, AccessToken: "token", UserAgent: "membership-janitor-domain-test"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	ordinary, err := clients.AccountMemberships.Create(ctx, hubspot.AccountMembershipCreate{Email: "tf_acc_owned_ordinary@example.com", SendWelcomeEmail: false})
	if err != nil {
		t.Fatal(err)
	}
	invalid, err := clients.AccountMemberships.Create(ctx, hubspot.AccountMembershipCreate{Email: "tf_acc_owned_invalid@example.org", SendWelcomeEmail: false})
	if err != nil {
		t.Fatal(err)
	}
	if err := deleteOwnedAccountMemberships(ctx, clients, "tf_acc_owned_"); err == nil {
		t.Fatal("account membership cleanup accepted an invalid owned email domain")
	}
	for _, id := range []string{ordinary.ID, invalid.ID} {
		if _, err := clients.AccountMemberships.GetByID(ctx, id); err != nil {
			t.Fatal("failed account membership cleanup partially mutated an owned identity")
		}
	}
}

func TestAccountMembershipJanitorRefusesIdentityMismatchBeforeMutation(t *testing.T) {
	fake := acceptance.NewFakeHubSpot("token", 123)
	server := httptest.NewServer(fake)
	defer server.Close()
	origin, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	clients, err := hubspot.NewClientSet(hubspot.TransportConfig{BaseURL: origin, AccessToken: "token", UserAgent: "membership-janitor-identity-test"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	first, err := clients.AccountMemberships.Create(ctx, hubspot.AccountMembershipCreate{Email: "tf_acc_owned_first@example.com", SendWelcomeEmail: false})
	if err != nil {
		t.Fatal(err)
	}
	second, err := clients.AccountMemberships.Create(ctx, hubspot.AccountMembershipCreate{Email: "tf_acc_owned_second@example.com", SendWelcomeEmail: false})
	if err != nil {
		t.Fatal(err)
	}
	mismatch := first
	mismatch.ID = second.ID
	fake.OverrideNextAccountMembershipEmailRead(first.Email, mismatch)
	if err := deleteOwnedAccountMemberships(ctx, clients, "tf_acc_owned_"); err == nil {
		t.Fatal("account membership cleanup accepted an identity mismatch")
	}
	for _, id := range []string{first.ID, second.ID} {
		if _, err := clients.AccountMemberships.GetByID(ctx, id); err != nil {
			t.Fatal("failed account membership cleanup partially mutated an owned identity")
		}
	}
}

func janitorClients(t *testing.T) (*hubspot.ClientSet, string) {
	t.Helper()
	shard := requiredEnvironment(t, "CAPABILITY_SHARD")
	if shard != "free_properties" && shard != "form_definitions" && shard != "files_configuration" && shard != "account_memberships" && shard != "deal_pipelines" && shard != "ticket_pipelines" {
		t.Fatal("janitor implementation is unavailable for the selected capability shard")
	}
	token := requiredEnvironment(t, "HUBSPOT_ACCESS_TOKEN")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	clients, err := acceptance.NewRealPortalClientSet(ctx, token, "terraform-provider-hubspot/acceptance-janitor")
	if err != nil {
		t.Fatalf("configure HubSpot acceptance janitor: %v", err)
	}
	return clients, shard
}

var generatedDecimalJanitorID = regexp.MustCompile(`^[1-9][0-9]*$`)

func countOwnedAccountMemberships(ctx context.Context, clients *hubspot.ClientSet, prefix string) (int, error) {
	memberships, err := clients.AccountMemberships.List(ctx)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, membership := range memberships {
		if strings.HasPrefix(membership.Email, prefix) {
			count++
		}
	}
	return count, nil
}

func deleteOwnedAccountMemberships(ctx context.Context, clients *hubspot.ClientSet, prefix string) error {
	memberships, err := clients.AccountMemberships.List(ctx)
	if err != nil {
		return err
	}
	owned := make([]hubspot.AccountMembership, 0)
	for _, membership := range memberships {
		if !strings.HasPrefix(membership.Email, prefix) {
			continue
		}
		if !generatedDecimalJanitorID.MatchString(membership.ID) || !strings.HasSuffix(membership.Email, "@example.com") {
			return errors.New("prefix-owned account membership has an unsupported Settings identity or email domain")
		}
		byID, idErr := clients.AccountMemberships.GetByID(ctx, membership.ID)
		byEmail, emailErr := clients.AccountMemberships.GetByEmail(ctx, membership.Email)
		if idErr != nil || emailErr != nil || byID.ID != membership.ID || byEmail.ID != membership.ID || byID.Email != membership.Email || byEmail.Email != membership.Email {
			return errors.New("prefix-owned account membership failed exact Settings ID and email verification")
		}
		if byID.SuperAdmin || byEmail.SuperAdmin {
			return errors.New("refusing to delete a prefix-owned Super Admin account membership")
		}
		owned = append(owned, membership)
	}
	sort.Slice(owned, func(left, right int) bool { return owned[left].ID < owned[right].ID })
	for _, membership := range owned {
		deleteErr := clients.AccountMemberships.Delete(ctx, membership.ID)
		if deleteErr != nil && !formJanitorNotFound(deleteErr) {
			var apiError *hubspot.Error
			if !errors.As(deleteErr, &apiError) || !apiError.Ambiguous {
				return fmt.Errorf("delete prefix-owned account membership: %w", deleteErr)
			}
		}
		if err := clients.AccountMemberships.WaitForAbsence(ctx, membership.ID, membership.Email); err != nil {
			return fmt.Errorf("verify prefix-owned account membership absence: %w", err)
		}
	}
	remaining, err := countOwnedAccountMemberships(ctx, clients, prefix)
	if err != nil {
		return err
	}
	if remaining != 0 {
		return errors.New("manual cleanup could not verify zero active prefix-owned account memberships")
	}
	return nil
}

func deleteOwnedFilesConfiguration(ctx context.Context, clients *hubspot.ClientSet, prefix string) error {
	files, err := clients.Files.Search(ctx, nil, "")
	if err != nil {
		return err
	}
	folders, err := clients.FileFolders.Search(ctx, nil, "")
	if err != nil {
		return err
	}

	ownedFiles := make([]hubspot.ManagedFile, 0)
	for _, file := range files {
		if file.Archived || !strings.HasPrefix(file.Name, prefix) {
			continue
		}
		if !generatedDecimalJanitorID.MatchString(file.ID) || !generatedDecimalJanitorID.MatchString(file.FolderID) || !strings.HasSuffix(file.Path, "/"+file.Name) {
			return errors.New("prefix-owned Managed file has an unsupported identity or placement")
		}
		ownedFiles = append(ownedFiles, file)
	}

	ownedFolders := make([]hubspot.FileFolder, 0)
	ownedFolderIDs := make(map[string]struct{})
	for _, folder := range folders {
		if folder.Archived || !strings.HasPrefix(folder.Name, prefix) {
			continue
		}
		if !generatedDecimalJanitorID.MatchString(folder.ID) || !strings.HasSuffix(folder.Path, "/"+folder.Name) {
			return errors.New("prefix-owned File folder has an unsupported identity or path")
		}
		ownedFolders = append(ownedFolders, folder)
		ownedFolderIDs[folder.ID] = struct{}{}
	}
	for _, file := range files {
		if file.Archived {
			continue
		}
		if _, parentOwned := ownedFolderIDs[file.FolderID]; parentOwned && !strings.HasPrefix(file.Name, prefix) {
			return errors.New("prefix-owned File folder contains an unowned Managed file")
		}
	}
	for _, folder := range folders {
		if folder.Archived || folder.ParentFolderID == nil {
			continue
		}
		if _, parentOwned := ownedFolderIDs[*folder.ParentFolderID]; parentOwned && !strings.HasPrefix(folder.Name, prefix) {
			return errors.New("prefix-owned File folder contains an unowned child folder")
		}
	}

	sort.Slice(ownedFiles, func(left, right int) bool { return ownedFiles[left].ID < ownedFiles[right].ID })
	for _, file := range ownedFiles {
		if err := deleteManagedFileAndVerifyAbsent(ctx, clients, file.ID); err != nil {
			return fmt.Errorf("delete prefix-owned Managed file: %w", err)
		}
	}
	sort.Slice(ownedFolders, func(left, right int) bool {
		leftDepth := strings.Count(ownedFolders[left].Path, "/")
		rightDepth := strings.Count(ownedFolders[right].Path, "/")
		if leftDepth == rightDepth {
			return ownedFolders[left].Path > ownedFolders[right].Path
		}
		return leftDepth > rightDepth
	})
	for _, folder := range ownedFolders {
		if err := deleteFileFolderAndVerifyAbsent(ctx, clients, folder.ID); err != nil {
			return fmt.Errorf("delete prefix-owned File folder leaf-first: %w", err)
		}
	}

	filesRemaining, foldersRemaining, err := acceptance.CountActiveFilesConfiguration(ctx, clients, prefix)
	if err != nil {
		return err
	}
	if filesRemaining != 0 || foldersRemaining != 0 {
		return errors.New("manual cleanup could not verify zero active prefix-owned Files configuration")
	}
	return nil
}

func deleteManagedFileAndVerifyAbsent(ctx context.Context, clients *hubspot.ClientSet, id string) error {
	return deleteFilesConfigurationAndVerifyAbsent(ctx, clients.Files.Delete, func(ctx context.Context, id string) error {
		_, err := clients.Files.Get(ctx, id)
		return err
	}, id)
}

func deleteFileFolderAndVerifyAbsent(ctx context.Context, clients *hubspot.ClientSet, id string) error {
	return deleteFilesConfigurationAndVerifyAbsent(ctx, clients.FileFolders.Delete, func(ctx context.Context, id string) error {
		_, err := clients.FileFolders.Get(ctx, id)
		return err
	}, id)

}

func deleteFilesConfigurationAndVerifyAbsent(ctx context.Context, deleteByID func(context.Context, string) error, readByID func(context.Context, string) error, id string) error {
	deleteErr := deleteByID(ctx, id)
	if deleteErr != nil && !formJanitorNotFound(deleteErr) {
		var apiError *hubspot.Error
		if !errors.As(deleteErr, &apiError) || !apiError.Ambiguous {
			return deleteErr
		}
	}
	for {
		readErr := readByID(ctx, id)
		if formJanitorNotFound(readErr) {
			return nil
		}
		if readErr != nil {
			return readErr
		}
		select {
		case <-ctx.Done():
			return errors.New("Files active absence was not verified before the operation deadline")
		case <-time.After(250 * time.Millisecond):
		}
	}
}

func countOwnedForms(ctx context.Context, clients *hubspot.ClientSet, prefix string) (int, int, error) {
	active, err := clients.Forms.List(ctx, false)
	if err != nil {
		return 0, 0, err
	}
	archived, err := clients.Forms.List(ctx, true)
	if err != nil {
		return 0, 0, err
	}
	return countFormsWithPrefix(active, prefix), countFormsWithPrefix(archived, prefix), nil
}

func countFormsWithPrefix(forms []hubspot.FormDefinition, prefix string) int {
	count := 0
	for _, form := range forms {
		if strings.HasPrefix(form.Name, prefix) {
			count++
		}
	}
	return count
}

func archiveOwnedForms(ctx context.Context, clients *hubspot.ClientSet, prefix string) (int, error) {
	active, err := clients.Forms.List(ctx, false)
	if err != nil {
		return 0, err
	}
	for _, form := range active {
		if !strings.HasPrefix(form.Name, prefix) {
			continue
		}
		if form.FormType != "hubspot" || form.ID == "" {
			return 0, errors.New("prefix-owned Form definition has an unsupported identity or type")
		}
		archiveErr := clients.Forms.Archive(ctx, form.ID)
		if _, activeErr := clients.Forms.Get(ctx, form.ID); activeErr == nil {
			return 0, errors.New("prefix-owned Form definition remained active after archival")
		} else if !formJanitorNotFound(activeErr) {
			return 0, fmt.Errorf("verify active Form definition absence: %w", activeErr)
		}
		archived, archivedErr := clients.Forms.GetArchived(ctx, form.ID)
		if archivedErr != nil {
			if archiveErr != nil {
				return 0, fmt.Errorf("archive prefix-owned Form definition: %w", archiveErr)
			}
			return 0, fmt.Errorf("verify Archived form definition: %w", archivedErr)
		}
		if archived.ID != form.ID || !archived.Archived {
			return 0, errors.New("Archived form definition identity was not exact")
		}
	}
	remaining, retained, err := countOwnedForms(ctx, clients, prefix)
	if err != nil {
		return 0, err
	}
	if remaining != 0 {
		return 0, errors.New("manual cleanup could not verify zero active prefix-owned Form definitions")
	}
	return retained, nil
}

func formJanitorNotFound(err error) bool {
	var apiError *hubspot.Error
	return errors.As(err, &apiError) && apiError.Status == 404
}

func freeJanitorClients(t *testing.T) *hubspot.ClientSet {
	t.Helper()
	clients, shard := janitorClients(t)
	if shard != "free_properties" {
		t.Fatal("free janitor client used the wrong capability shard")
	}
	return clients
}

func countFreeOwnedConfiguration(t *testing.T, ctx context.Context, clients *hubspot.ClientSet, prefix string) (int, int) {
	t.Helper()
	properties, err := clients.Properties.List(ctx, "contacts", false, "non_sensitive", "")
	if err != nil {
		t.Fatalf("list property definitions for janitor verification: %s", acceptance.SanitizedHubSpotError(err))
	}
	groups, err := clients.PropertyGroups.List(ctx, "contacts")
	if err != nil {
		t.Fatalf("list property groups for janitor verification: %s", acceptance.SanitizedHubSpotError(err))
	}
	propertyCount := 0
	for _, property := range properties {
		if strings.HasPrefix(property.Name, prefix) {
			propertyCount++
		}
	}
	groupCount := 0
	for _, group := range groups {
		if strings.HasPrefix(group.Name, prefix) {
			groupCount++
		}
	}
	return propertyCount, groupCount
}

func requireFreeOwnedConfigurationAbsent(t *testing.T, prefix string) {
	t.Helper()
	clients, shard := janitorClients(t)
	if shard != "free_properties" {
		t.Fatal("free owned-configuration verification used the wrong capability shard")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	properties, groups := countFreeOwnedConfiguration(t, ctx, clients, prefix)
	if properties != 0 || groups != 0 {
		t.Fatal("independent cleanup verification found active owned CRM configuration")
	}
}

func requireFreeOwnedConfigurationAbsentForStandardObjectTypes(t *testing.T, prefix string) {
	t.Helper()
	clients := freeJanitorClients(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	for _, objectType := range []string{"contacts", "companies", "deals", "tickets"} {
		properties, err := clients.Properties.List(ctx, objectType, false, "non_sensitive", "")
		if err != nil {
			t.Fatalf("list %s property definitions for cleanup verification: %s", objectType, acceptance.SanitizedHubSpotError(err))
		}
		groups, err := clients.PropertyGroups.List(ctx, objectType)
		if err != nil {
			t.Fatalf("list %s property groups for cleanup verification: %s", objectType, acceptance.SanitizedHubSpotError(err))
		}
		for _, property := range properties {
			if strings.HasPrefix(property.Name, prefix) {
				t.Fatalf("cleanup left an active %s property definition: %s", objectType, property.Name)
			}
		}
		for _, group := range groups {
			if strings.HasPrefix(group.Name, prefix) {
				t.Fatalf("cleanup left an active %s property group: %s", objectType, group.Name)
			}
		}
	}
}

func countOwnedPipelines(t *testing.T, ctx context.Context, clients *hubspot.ClientSet, objectType, prefix string) int {
	t.Helper()
	pipelines, err := clients.Pipelines.List(ctx, objectType)
	if err != nil {
		t.Fatalf("list pipelines for janitor verification: %s", acceptance.SanitizedHubSpotError(err))
	}
	count := 0
	for _, pipeline := range pipelines {
		if !pipeline.Archived && strings.HasPrefix(pipeline.Label, prefix) {
			count++
		}
	}
	return count
}
