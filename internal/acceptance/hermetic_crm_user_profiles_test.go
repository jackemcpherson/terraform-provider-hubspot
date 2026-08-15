// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

//go:build acceptance

package acceptance_test

import (
	"fmt"
	"net/http/httptest"
	"os/exec"
	"reflect"
	"testing"

	"github.com/jackemcpherson/terraform-provider-hubspot/internal/acceptance"
)

func TestHermeticCRMUserProfileLifecycle(t *testing.T) {
	runHermeticCRMUserProfileLifecycle(t, acceptance.OpenTofu, "registry.opentofu.org/jackemcpherson/hubspot")
}

func TestHermeticCRMUserProfileLifecycleTerraformParity(t *testing.T) {
	runHermeticCRMUserProfileLifecycle(t, acceptance.Terraform, "registry.terraform.io/jackemcpherson/hubspot")
}

func TestHermeticCRMUserProfileValidationAndAmbiguousJoin(t *testing.T) {
	if _, err := exec.LookPath(string(acceptance.OpenTofu)); err != nil {
		t.Skip("pinned OpenTofu executable is not installed")
	}
	fake := acceptance.NewFakeHubSpot(hermeticToken, 666000668)
	firstID := fake.SeedCRMUserProfile("101")
	secondID := fake.DuplicateCRMUserProfile("101")
	server := httptest.NewServer(fake)
	t.Cleanup(server.Close)
	t.Setenv("HUBSPOT_ACCESS_TOKEN", hermeticToken)

	acceptance.Run(t, acceptance.Options{
		Engine: acceptance.OpenTofu, Shard: acceptance.AccountMemberships,
		Prefix: "tf_acc_hermetic_crm_profile_guards_", LedgerPath: t.TempDir() + "/cleanup.jsonl", ProbeBaseURL: server.URL,
	}, func(session *acceptance.Session) {
		session.RequireValidationFailure(
			hermeticCRMUserProfileNoPropertiesConfig(server.URL, "registry.opentofu.org/jackemcpherson/hubspot", "101"),
			"CRM user profile manages no properties",
		)
		session.RequireApplyFailure(hermeticCRMUserProfileResolvedNullConfig(
			server.URL, "registry.opentofu.org/jackemcpherson/hubspot", "101",
		))
		if reads := fake.CRMUserProfileListReads(); reads != 0 {
			t.Fatalf("apply-time validation performed readiness reads: %d", reads)
		}
		session.RequireApplyFailure(hermeticCRMUserProfileConfig(
			server.URL, "registry.opentofu.org/jackemcpherson/hubspot", "101", "Engineer", "away",
		))
		if firstPatches, secondPatches := fake.CRMUserProfilePatchCount(firstID), fake.CRMUserProfilePatchCount(secondID); firstPatches != 0 || secondPatches != 0 {
			t.Fatalf("ambiguous identity sent profile patches: %d, %d", firstPatches, secondPatches)
		}
	})
}

func runHermeticCRMUserProfileLifecycle(t *testing.T, engine acceptance.Engine, providerSource string) {
	t.Helper()
	if _, err := exec.LookPath(string(engine)); err != nil {
		t.Skipf("pinned %s executable is not installed", engine)
	}
	fake := acceptance.NewFakeHubSpot(hermeticToken, 666000668)
	crmID := fake.SeedCRMUserProfile("101")
	fake.DelayCRMUserProfileMaterialization(crmID, 2)
	server := httptest.NewServer(fake)
	t.Cleanup(server.Close)
	t.Setenv("HUBSPOT_ACCESS_TOKEN", hermeticToken)
	config := hermeticCRMUserProfileConfig(server.URL, providerSource, "101", "Engineer", "away")

	acceptance.Run(t, acceptance.Options{
		Engine: engine, Shard: acceptance.AccountMemberships,
		Prefix: "tf_acc_hermetic_crm_profiles_", LedgerPath: t.TempDir() + "/cleanup.jsonl", ProbeBaseURL: server.URL,
	}, func(session *acceptance.Session) {
		session.Apply(config)
		session.RequireStateString("hubspot_crm_user_profile.managed", "id", crmID)
		session.RequireStateString("hubspot_crm_user_profile.managed", "account_membership_id", "101")
		session.RequireStateString("hubspot_crm_user_profile.managed", "job_title", "Engineer")
		session.RequireStateString("hubspot_crm_user_profile.managed", "availability_status", "away")
		session.RequireStateString("hubspot_crm_user_profile.managed", "time_zone", "Australia/Melbourne")
		session.RequireEmptyPlan(config)
		if patches := fake.CRMUserProfilePatchCount(crmID); patches != 2 {
			t.Fatalf("initial CRM user profile patches = %d, want 2", patches)
		}
		if history := fake.CRMUserProfilePatchHistory(crmID); !reflect.DeepEqual(history, [][]string{
			{"hs_availability_status", "hs_job_title", "hs_standard_time_zone"},
			{"hs_working_hours"},
		}) {
			t.Fatalf("initial CRM user profile patch ordering = %#v", history)
		}
		if reads := fake.CRMUserProfileListReads(); reads != 3 {
			t.Fatalf("readiness list reads = %d, want 3", reads)
		}

		if !fake.DriftCRMUserProfile(crmID, "Drifted", "available") {
			t.Fatal("could not inject CRM user profile drift")
		}
		session.RequirePlanDiffAttributes(config, "hubspot_crm_user_profile.managed", "job_title", "availability_status")
		fake.MalformNextCRMUserProfilePatchSuccess()
		session.Apply(config)
		session.RequireEmptyPlan(config)
		if patches := fake.CRMUserProfilePatchCount(crmID); patches != 3 {
			t.Fatalf("drift repair patches = %d, want 3", patches)
		}
		fake.RejectNextCRMUserProfilePatch()
		rejected := hermeticCRMUserProfileConfig(server.URL, providerSource, "101", "Principal Engineer", "away")
		session.RequireApplyFailure(rejected)
		if patches := fake.CRMUserProfilePatchCount(crmID); patches != 3 {
			t.Fatalf("rejected API write changed profile patch count: %d", patches)
		}

		session.RemoveState("hubspot_crm_user_profile.managed")
		for _, invalidImport := range []struct {
			id    string
			title string
		}{
			{id: "0", title: "Invalid CRM user profile import ID"},
			{id: "01", title: "Invalid CRM user profile import ID"},
			{id: "membership:", title: "Invalid CRM user profile membership import ID"},
			{id: "membership:01", title: "Invalid CRM user profile membership import ID"},
		} {
			session.RequireImportFailure(config, "hubspot_crm_user_profile.managed", invalidImport.id, invalidImport.title)
		}
		session.Import("hubspot_crm_user_profile.managed", crmID)
		session.RequireStateString("hubspot_crm_user_profile.managed", "id", crmID)
		session.RequireStateString("hubspot_crm_user_profile.managed", "account_membership_id", "101")
		session.Apply(config)
		session.RequireEmptyPlan(config)
		session.RemoveState("hubspot_crm_user_profile.managed")
		session.Import("hubspot_crm_user_profile.managed", "membership:101")
		session.RequireStateString("hubspot_crm_user_profile.managed", "id", crmID)
		session.Apply(config)
		session.RequireEmptyPlan(config)
		if patches := fake.CRMUserProfilePatchCount(crmID); patches != 3 {
			t.Fatalf("imports or semantic no-op wrote CRM profile: patches = %d", patches)
		}

		session.Destroy(config)
		if patches := fake.CRMUserProfilePatchCount(crmID); patches != 3 {
			t.Fatalf("destroy wrote CRM profile: patches = %d", patches)
		}
		profile, ok := fake.CRMUserProfileSnapshot(crmID)
		if !ok || profile.JobTitle != "Engineer" || profile.AvailabilityStatus != "away" {
			t.Fatalf("destroy did not retain CRM profile values: %#v, %v", profile, ok)
		}
	})
}

func hermeticCRMUserProfileConfig(baseURL, providerSource, membershipID, jobTitle, availability string) string {
	return fmt.Sprintf(`
terraform {
  required_providers {
    hubspot = {
      source = %q
    }
  }
}

provider "hubspot" {
  api_base_url = %q
}

resource "hubspot_crm_user_profile" "managed" {
  account_membership_id = %q
  job_title             = %q
  availability_status   = %q
  time_zone             = "Australia/Melbourne"
  working_hours = [
    {
      days         = "MONDAY_TO_FRIDAY"
      start_minute = 540
      end_minute   = 1020
    }
  ]
}
`, providerSource, baseURL, membershipID, jobTitle, availability)
}

func hermeticCRMUserProfileNoPropertiesConfig(baseURL, providerSource, membershipID string) string {
	return fmt.Sprintf(`
terraform {
  required_providers {
    hubspot = {
      source = %q
    }
  }
}

provider "hubspot" {
  api_base_url = %q
}

resource "hubspot_crm_user_profile" "managed" {
  account_membership_id = %q
}
`, providerSource, baseURL, membershipID)
}

func hermeticCRMUserProfileResolvedNullConfig(baseURL, providerSource, membershipID string) string {
	return fmt.Sprintf(`
terraform {
  required_providers {
    hubspot = {
      source = %q
    }
  }
}

provider "hubspot" {
  api_base_url = %q
}

resource "terraform_data" "resolved" {
  input = null
}

resource "hubspot_crm_user_profile" "managed" {
  account_membership_id = %q
  job_title             = terraform_data.resolved.output
}
`, providerSource, baseURL, membershipID)
}
