// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

//go:build acceptance

package acceptance_test

import (
	"fmt"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"

	"github.com/jackemcpherson/terraform-provider-hubspot/internal/acceptance"
)

func TestHermeticAccountMembershipLifecycle(t *testing.T) {
	runHermeticAccountMembershipLifecycle(t, acceptance.OpenTofu, "registry.opentofu.org/jackemcpherson/hubspot")
}

func TestHermeticAccountMembershipLifecycleTerraformParity(t *testing.T) {
	runHermeticAccountMembershipLifecycle(t, acceptance.Terraform, "registry.terraform.io/jackemcpherson/hubspot")
}

func runHermeticAccountMembershipLifecycle(t *testing.T, engine acceptance.Engine, providerSource string) {
	t.Helper()
	if _, err := exec.LookPath(string(engine)); err != nil {
		t.Skipf("pinned %s executable is not installed", engine)
	}
	fake := acceptance.NewFakeHubSpot(hermeticToken, 666000668)
	server := httptest.NewServer(fake)
	t.Cleanup(server.Close)
	t.Setenv("HUBSPOT_ACCESS_TOKEN", hermeticToken)
	email := "membership@example.com"
	initial := hermeticAccountMembershipDefaultGuardConfig(server.URL, providerSource, email, "First", "Member", false)
	removable := hermeticAccountMembershipConfig(server.URL, providerSource, email, "First", "Member", false, true)
	updated := hermeticAccountMembershipConfig(server.URL, providerSource, email, "Updated", "Member", false, true)

	acceptance.Run(t, acceptance.Options{
		Engine: engine, Shard: acceptance.AccountMemberships,
		Prefix: "tf_acc_hermetic_memberships_", LedgerPath: t.TempDir() + "/cleanup.jsonl", ProbeBaseURL: server.URL,
	}, func(session *acceptance.Session) {
		session.Apply(initial)
		id := session.OpaqueStateString("hubspot_account_membership.managed", "id")
		session.RequireStateString("hubspot_account_membership.managed", "email", email)
		session.RequireStateString("hubspot_account_membership.managed", "first_name", "First")
		session.RequireStateBool("hubspot_account_membership.managed", "send_welcome_email", false)
		session.RequireStateBool("hubspot_account_membership.managed", "allow_removal", false)
		session.RequireStateBool("hubspot_account_membership.managed", "super_admin", false)
		session.RequireEmptyPlan(initial)
		session.RequireDestroyFailure(initial)
		if creates, updates, deletes := fake.AccountMembershipWriteCounts(id); creates != 1 || updates != 0 || deletes != 0 {
			t.Fatalf("initial membership writes = %d/%d/%d", creates, updates, deletes)
		}

		if !fake.DriftAccountMembershipNames(id, "Drifted", "Identity") {
			t.Fatal("could not inject account membership name drift")
		}
		session.RequirePlanDiffAttributes(initial, "hubspot_account_membership.managed", "first_name", "last_name")
		session.Apply(initial)
		session.RequireEmptyPlan(initial)

		fake.SetAccountMembershipAssignments(id, true)
		session.RequireApplyFailure(updated)
		fake.SetAccountMembershipAssignments(id, false)
		fake.SetAccountMembershipTeamAssignment(id, true)
		session.RequireApplyFailure(updated)
		fake.SetAccountMembershipTeamAssignment(id, false)
		fake.SetAccountMembershipActivated(id, false)
		updatesBefore := fake.AccountMembershipUpdateAttempts(id)
		session.RequireApplyFailure(updated)
		if fake.AccountMembershipUpdateAttempts(id) != updatesBefore+1 {
			t.Fatal("activation failure was retried or not sent exactly once")
		}
		fake.SetAccountMembershipActivated(id, true)
		session.Apply(updated)
		session.RequireEmptyPlan(updated)

		current, ok := fake.AccountMembershipSnapshot(id)
		if !ok {
			t.Fatal("could not snapshot the account membership")
		}
		conflicting := current
		conflicting.Email = "conflicting@example.com"
		conflictingUpdate := hermeticAccountMembershipConfig(server.URL, providerSource, email, "Conflicting", "Member", false, true)
		updatesBefore = fake.AccountMembershipUpdateAttempts(id)
		fake.OverrideNextAccountMembershipEmailRead(email, conflicting)
		session.RequireApplyFailure(conflictingUpdate)
		if fake.AccountMembershipUpdateAttempts(id) != updatesBefore {
			t.Fatal("identity-conflicting name update sent a PUT")
		}
		_, _, deletesBefore := fake.AccountMembershipWriteCounts(id)
		fake.OverrideNextAccountMembershipEmailRead(email, conflicting)
		session.RequireDestroyFailure(updated)
		_, _, deletesAfter := fake.AccountMembershipWriteCounts(id)
		if deletesAfter != deletesBefore {
			t.Fatal("identity-conflicting destroy sent a DELETE")
		}
		emailSuperAdmin := current
		emailSuperAdmin.SuperAdmin = true
		fake.OverrideNextAccountMembershipEmailRead(email, emailSuperAdmin)
		session.RequireDestroyFailure(updated)
		_, _, deletesAfter = fake.AccountMembershipWriteCounts(id)
		if deletesAfter != deletesBefore {
			t.Fatal("email-reported Super Admin destroy sent a DELETE")
		}

		session.Apply(removable)
		session.RemoveState("hubspot_account_membership.managed")
		for _, invalidImport := range []struct {
			id    string
			title string
		}{
			{id: email, title: "Invalid account membership import ID"},
			{id: "0", title: "Invalid account membership import ID"},
			{id: "01", title: "Invalid account membership import ID"},
			{id: "email:", title: "Invalid account membership import email"},
			{id: "email: " + email, title: "Invalid account membership import email"},
		} {
			session.RequireImportFailure(removable, "hubspot_account_membership.managed", invalidImport.id, invalidImport.title)
		}
		current, ok = fake.AccountMembershipSnapshot(id)
		if !ok {
			t.Fatal("could not snapshot the account membership for import")
		}
		mismatchedEmail := current
		mismatchedEmail.Email = "mismatched@example.com"
		fake.OverrideNextAccountMembershipEmailRead(email, mismatchedEmail)
		session.RequireImportFailure(removable, "hubspot_account_membership.managed", "email:"+email, "Account membership import email mismatch")
		mismatchedID := current
		mismatchedID.ID = "999"
		fake.OverrideNextAccountMembershipIDRead(id, mismatchedID)
		session.RequireImportFailure(removable, "hubspot_account_membership.managed", id, "Account membership import identity mismatch")
		session.Import("hubspot_account_membership.managed", "email:"+email)
		session.RequireStateString("hubspot_account_membership.managed", "id", id)
		session.RequireStateBool("hubspot_account_membership.managed", "send_welcome_email", false)
		session.RequireStateBool("hubspot_account_membership.managed", "allow_removal", false)
		session.Apply(removable)
		session.RemoveState("hubspot_account_membership.managed")
		session.Import("hubspot_account_membership.managed", id)
		session.Apply(removable)

		duplicate := removable + strings.Replace(removable, `resource "hubspot_account_membership" "managed"`, `resource "hubspot_account_membership" "duplicate"`, 1)
		session.RequireApplyFailure(duplicate)
		session.RequireEmptyPlan(removable)

		fake.SetAccountMembershipSuperAdmin(id, true)
		session.Refresh(removable)
		session.RequireStateBool("hubspot_account_membership.managed", "super_admin", true)
		session.RequireDestroyFailure(removable)
		fake.SetAccountMembershipSuperAdmin(id, false)
		fake.LagNextAccountMembershipCollectionAfterDelete(2)
		session.Destroy(removable)
		if fake.ActiveAccountMembershipCount() != 0 {
			t.Fatal("destroy retained an active account membership")
		}

		session.Apply(removable)
		reusedID := session.OpaqueStateString("hubspot_account_membership.managed", "id")
		if reusedID != id {
			t.Fatal("same-email reprovision did not reuse the Settings user ID")
		}
		if !fake.DisappearAccountMembership(reusedID) {
			t.Fatal("could not inject account membership disappearance")
		}
		session.Refresh(removable)
		session.RequireStateAbsent("hubspot_account_membership.managed")
		session.Apply(removable)
		if session.OpaqueStateString("hubspot_account_membership.managed", "id") != id {
			t.Fatal("same-email recreation changed the Settings user ID")
		}

		welcome := hermeticAccountMembershipConfig(server.URL, providerSource, email, "First", "Member", true, true)
		session.Apply(welcome)
		if !fake.LastAccountMembershipWelcomeChoice(email) {
			t.Fatal("replacement create did not carry the explicit true welcome choice")
		}
		replacementEmail := "replacement@example.com"
		replacement := hermeticAccountMembershipConfig(server.URL, providerSource, replacementEmail, "First", "Member", false, true)
		session.Apply(replacement)
		replacementID := session.OpaqueStateString("hubspot_account_membership.managed", "id")
		if replacementID == id {
			t.Fatal("email replacement retained the prior Settings user ID")
		}
		session.Destroy(replacement)

		unnamed := hermeticAccountMembershipWithoutNamesConfig(server.URL, providerSource, email, false, true)
		session.Apply(unnamed)
		unnamedID := session.OpaqueStateString("hubspot_account_membership.managed", "id")
		updatesBefore = fake.AccountMembershipUpdateAttempts(unnamedID)
		if !fake.DriftAccountMembershipNames(unnamedID, "Observed", "Only") {
			t.Fatal("could not inject optional-name drift")
		}
		session.Refresh(unnamed)
		session.RequireStateString("hubspot_account_membership.managed", "first_name", "Observed")
		session.RequireStateString("hubspot_account_membership.managed", "last_name", "Only")
		session.RequireEmptyPlan(unnamed)
		if fake.AccountMembershipUpdateAttempts(unnamedID) != updatesBefore {
			t.Fatal("unconfigured optional-name drift sent a PUT")
		}
		session.Destroy(unnamed)

		unknownEmail := "unknown-outcome@example.com"
		unknown := hermeticAccountMembershipConfig(server.URL, providerSource, unknownEmail, "Unknown", "Outcome", false, true)
		fake.FailNextAccountMembershipOperation(acceptance.AccountMembershipFaultCreateUnknown)
		session.RequireApplyFailure(unknown)
		session.RequireStateAbsent("hubspot_account_membership.managed")
		unknownResidualID := fake.ActiveAccountMembershipID(unknownEmail)
		if unknownResidualID == "" {
			t.Fatal("ambiguous create did not leave the expected fake residual")
		}
		session.RequireApplyFailure(unknown)
		if !fake.DisappearAccountMembership(unknownResidualID) {
			t.Fatal("could not remove ambiguous fake residual")
		}
		session.Apply(unknown)
		session.Destroy(unknown)

		knownEmail := "known-id-outcome@example.com"
		known := hermeticAccountMembershipConfig(server.URL, providerSource, knownEmail, "Known", "Outcome", false, true)
		fake.FailNextAccountMembershipOperation(acceptance.AccountMembershipFaultCreateKnown)
		session.Apply(known)
		session.RequireEmptyPlan(known)
		session.Destroy(known)

		for _, mismatch := range []struct {
			email string
			fault acceptance.AccountMembershipFault
		}{
			{email: "known-id-email-mismatch@example.com", fault: acceptance.AccountMembershipFaultCreateKnownEmailMismatch},
			{email: "known-id-name-mismatch@example.com", fault: acceptance.AccountMembershipFaultCreateKnownNameMismatch},
		} {
			config := hermeticAccountMembershipConfig(server.URL, providerSource, mismatch.email, "Known", "Mismatch", false, true)
			fake.FailNextAccountMembershipOperation(mismatch.fault)
			session.RequireApplyFailure(config)
			residualID := fake.ActiveAccountMembershipID(mismatch.email)
			if residualID == "" {
				t.Fatal("known-ID mismatch did not leave the expected fake residual")
			}
			session.RequireStateString("hubspot_account_membership.managed", "id", residualID)
			if !fake.DisappearAccountMembership(residualID) {
				t.Fatal("could not remove known-ID mismatch fake residual")
			}
			session.Refresh(config)
			session.RequireStateAbsent("hubspot_account_membership.managed")
		}
	})
}

func hermeticAccountMembershipDefaultGuardConfig(baseURL, providerSource, email, firstName, lastName string, welcome bool) string {
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

resource "hubspot_account_membership" "managed" {
  email              = %q
  first_name         = %q
  last_name          = %q
  send_welcome_email = %t
}
`, providerSource, baseURL, email, firstName, lastName, welcome)
}

func hermeticAccountMembershipConfig(baseURL, providerSource, email, firstName, lastName string, welcome, allowRemoval bool) string {
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

resource "hubspot_account_membership" "managed" {
  email              = %q
  first_name         = %q
  last_name          = %q
  send_welcome_email = %t
  allow_removal      = %t
}
`, providerSource, baseURL, email, firstName, lastName, welcome, allowRemoval)
}

func hermeticAccountMembershipWithoutNamesConfig(baseURL, providerSource, email string, welcome, allowRemoval bool) string {
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

resource "hubspot_account_membership" "managed" {
  email              = %q
  send_welcome_email = %t
  allow_removal      = %t
}
`, providerSource, baseURL, email, welcome, allowRemoval)
}
