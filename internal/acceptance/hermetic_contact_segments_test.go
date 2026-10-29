// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

//go:build acceptance

package acceptance_test

import (
	"fmt"
	"net/http/httptest"
	"os/exec"
	"testing"

	"github.com/jackemcpherson/terraform-provider-hubspot/internal/acceptance"
	"github.com/jackemcpherson/terraform-provider-hubspot/internal/hubspot"
)

func TestHermeticContactSegmentLifecycle(t *testing.T) {
	runHermeticContactSegmentLifecycle(t, acceptance.OpenTofu, "registry.opentofu.org/jackemcpherson/hubspot")
}

func TestHermeticContactSegmentLifecycleTerraformParity(t *testing.T) {
	runHermeticContactSegmentLifecycle(t, acceptance.Terraform, "registry.terraform.io/jackemcpherson/hubspot")
}

func TestHermeticContactSegmentRecovery(t *testing.T) {
	runHermeticContactSegmentRecovery(t, acceptance.OpenTofu, "registry.opentofu.org/jackemcpherson/hubspot")
}

func TestHermeticContactSegmentRecoveryTerraformParity(t *testing.T) {
	runHermeticContactSegmentRecovery(t, acceptance.Terraform, "registry.terraform.io/jackemcpherson/hubspot")
}

func TestHermeticContactSegmentFaults(t *testing.T) {
	runHermeticContactSegmentFaults(t, acceptance.OpenTofu, "registry.opentofu.org/jackemcpherson/hubspot")
}

func TestHermeticContactSegmentFaultsTerraformParity(t *testing.T) {
	runHermeticContactSegmentFaults(t, acceptance.Terraform, "registry.terraform.io/jackemcpherson/hubspot")
}

// runHermeticContactSegmentFaults drives unknown creates, unsupported remote
// definitions, tombstone restoration outcomes, and disappearing deletes
// through the engine with exact per-identity request counts.
func runHermeticContactSegmentFaults(t *testing.T, engine acceptance.Engine, providerSource string) {
	t.Helper()
	if _, err := exec.LookPath(string(engine)); err != nil {
		t.Skipf("pinned %s executable is not installed", engine)
	}
	fake := acceptance.NewFakeHubSpot(hermeticToken, 666000671)
	server := httptest.NewServer(fake)
	t.Cleanup(server.Close)
	t.Setenv("HUBSPOT_ACCESS_TOKEN", hermeticToken)
	t.Setenv("HUBSPOT_CONTACT_SEGMENT_TEST_POLL_INTERVAL", "1ms")
	t.Setenv("HUBSPOT_CONTACT_SEGMENT_TEST_POLL_TIMEOUT", "2s")
	const address = "hubspot_contact_segment.dynamic"
	config := hermeticContactSegmentSingleConfig(server.URL, providerSource, "Faults", "active")
	acceptance.Run(t, acceptance.Options{
		Engine: engine, Shard: acceptance.ContactSegments,
		Prefix: "tf_acc_hermetic_segments_recovery_", LedgerPath: t.TempDir() + "/cleanup.jsonl", ProbeBaseURL: server.URL,
	}, func(session *acceptance.Session) {
		requireCounts := func(id, phase string, want [6]int) {
			t.Helper()
			creates, reads, names, filters, deletes, restores := fake.ContactSegmentRequestCounts(id)
			if got := [6]int{creates, reads, names, filters, deletes, restores}; got != want {
				t.Fatalf("%s requests creates/reads/names/filters/deletes/restores = %v, want %v", phase, got, want)
			}
		}

		// An unknown create retains no state and never searches by name; the
		// orphan is recoverable only by exact-ID import.
		fake.FailNextContactSegmentOperation(acceptance.ContactSegmentFaultCreateUnknown)
		session.RequireApplyFailure(config)
		session.RequireStateAbsent(address)
		orphans := fake.ActiveContactSegmentIDs()
		if len(orphans) != 1 {
			t.Fatalf("unknown create left %d active identities, want 1", len(orphans))
		}
		id := orphans[0]
		requireCounts(id, "unknown create", [6]int{1, 0, 0, 0, 0, 0})
		session.Import(address, id)
		session.RequireEmptyPlan(config)
		session.RequireStateString(address, "id", id)

		// Unsupported remote meaning fails refresh without dropping state and
		// fails exact-ID import without adoption.
		if !fake.SetContactSegmentUnsupported(id, true) {
			t.Fatal("could not inject an unsupported remote filter")
		}
		session.RequirePlanFailure(config, "Contact segment refresh failed")
		session.RequireStateString(address, "id", id)
		session.RemoveState(address)
		session.RequireImportFailure(config, address, id, "Contact segment import failed")
		session.RequireStateAbsent(address)
		if !fake.SetContactSegmentUnsupported(id, false) {
			t.Fatal("could not clear the unsupported remote filter")
		}
		session.Import(address, id)
		session.RequireEmptyPlan(config)
		_, _, names, filters, deletes, restores := fake.ContactSegmentRequestCounts(id)
		if names != 0 || filters != 0 || deletes != 0 || restores != 0 {
			t.Fatal("unsupported refresh and import sent a remote write")
		}

		// A tombstone restored out of band after refresh needs no restore.
		if !fake.DeleteContactSegment(id) {
			t.Fatal("could not tombstone the definition out of band")
		}
		session.Refresh(config)
		session.RequireStateString(address, "id", id)
		if !fake.RestoreContactSegment(id) {
			t.Fatal("could not restore the definition out of band")
		}
		session.ApplyWithoutRefresh(config)
		session.RequireStateString(address, "id", id)
		session.RequireEmptyPlan(config)
		if _, _, names, filters, _, restores := fake.ContactSegmentRequestCounts(id); names != 0 || filters != 0 || restores != 0 {
			t.Fatalf("already active definition sent names/filters/restores = %d/%d/%d, want 0/0/0", names, filters, restores)
		}

		// An ambiguous restore succeeds only after exact-ID read-back.
		if !fake.DeleteContactSegment(id) {
			t.Fatal("could not tombstone the definition before an ambiguous restore")
		}
		session.Refresh(config)
		fake.FailNextContactSegmentOperation(acceptance.ContactSegmentFaultRestoreApplied)
		session.ApplyWithWarning(config, acceptance.ContactSegmentAmbiguousWrite)
		session.RequireStateString(address, "id", id)
		session.RequireEmptyPlan(config)
		if _, _, names, filters, _, restores := fake.ContactSegmentRequestCounts(id); names != 0 || filters != 0 || restores != 1 {
			t.Fatalf("ambiguous restore sent names/filters/restores = %d/%d/%d, want 0/0/1", names, filters, restores)
		}

		// A restore observation without the retained definition keeps polling
		// instead of rewriting the name.
		if !fake.DeleteContactSegment(id) {
			t.Fatal("could not tombstone the definition before an incomplete restore")
		}
		session.Refresh(config)
		fake.FailNextContactSegmentOperation(acceptance.ContactSegmentFaultRestoreIncomplete)
		session.Apply(config)
		session.RequireStateString(address, "id", id)
		session.RequireEmptyPlan(config)
		if _, _, names, filters, _, restores := fake.ContactSegmentRequestCounts(id); names != 0 || filters != 0 || restores != 2 {
			t.Fatalf("incomplete restore sent names/filters/restores = %d/%d/%d, want 0/0/2", names, filters, restores)
		}

		// A definition that disappears during delete completes destroy.
		fake.FailNextContactSegmentOperation(acceptance.ContactSegmentFaultDeleteDisappears)
		session.Destroy(config)
		session.RequireStateAbsent(address)
		if _, ok := fake.ContactSegmentSnapshot(id); ok {
			t.Fatal("disappearing delete retained the fake identity")
		}
	})
}

func runHermeticContactSegmentRecovery(t *testing.T, engine acceptance.Engine, providerSource string) {
	t.Helper()
	if _, err := exec.LookPath(string(engine)); err != nil {
		t.Skipf("pinned %s executable is not installed", engine)
	}
	fake := acceptance.NewFakeHubSpot(hermeticToken, 666000669)
	server := httptest.NewServer(fake)
	t.Cleanup(server.Close)
	t.Setenv("HUBSPOT_ACCESS_TOKEN", hermeticToken)
	t.Setenv("HUBSPOT_CONTACT_SEGMENT_TEST_POLL_INTERVAL", "1ms")
	t.Setenv("HUBSPOT_CONTACT_SEGMENT_TEST_POLL_TIMEOUT", "30ms")
	config := hermeticContactSegmentSingleConfig(server.URL, providerSource, "Recovery", "active")
	acceptance.Run(t, acceptance.Options{
		Engine: engine, Shard: acceptance.ContactSegments,
		Prefix: "tf_acc_hermetic_segments_recovery_", LedgerPath: t.TempDir() + "/cleanup.jsonl", ProbeBaseURL: server.URL,
	}, func(session *acceptance.Session) {
		fake.FailNextContactSegmentOperation(acceptance.ContactSegmentFaultCreateKnown)
		session.Apply(config)
		id := session.OpaqueStateString("hubspot_contact_segment.dynamic", "id")
		session.RequireEmptyPlan(config)

		fake.FailNextContactSegmentOperation(acceptance.ContactSegmentFaultUpdatePending)
		pending := hermeticContactSegmentSingleConfig(server.URL, providerSource, "Recovery", "pending")
		session.Apply(pending)
		session.RequireStateString("hubspot_contact_segment.dynamic", "id", id)

		fake.FailNextContactSegmentOperation(acceptance.ContactSegmentFaultUpdateStale)
		stale := hermeticContactSegmentSingleConfig(server.URL, providerSource, "Recovery", "stale")
		session.Apply(stale)
		session.RequireStateString("hubspot_contact_segment.dynamic", "id", id)

		fake.FailNextContactSegmentOperation(acceptance.ContactSegmentFaultUpdateApplied)
		updated := hermeticContactSegmentSingleConfig(server.URL, providerSource, "Recovery", "ready")
		session.Apply(updated)
		session.RequireStateString("hubspot_contact_segment.dynamic", "id", id)
		_, filters, _, _ := fake.ContactSegmentWriteCounts(id)
		if filters != 3 {
			t.Fatalf("pending, stale, and ambiguous dynamic filter writes = %d, want 3", filters)
		}

		fake.FailNextContactSegmentOperation(acceptance.ContactSegmentFaultUpdateRejected)
		rejected := hermeticContactSegmentSingleConfig(server.URL, providerSource, "Recovery", "rejected")
		session.RequireApplyFailure(rejected)
		session.RequireEmptyPlan(updated)

		fake.FailNextContactSegmentOperation(acceptance.ContactSegmentFaultUpdateTimeout)
		timedOut := hermeticContactSegmentSingleConfig(server.URL, providerSource, "Recovery", "timeout")
		session.RequireApplyFailure(timedOut)
		if !fake.CompleteContactSegment(id) {
			t.Fatal("could not complete timed-out fake Contact segment")
		}
		session.Apply(updated)
		session.RequireStateString("hubspot_contact_segment.dynamic", "id", id)

		fake.FailNextContactSegmentOperation(acceptance.ContactSegmentFaultProcessingFailed)
		failed := hermeticContactSegmentSingleConfig(server.URL, providerSource, "Recovery", "failed")
		session.RequireApplyFailure(failed)
		if !fake.DriftContactSegmentFilters(id, []hubspot.ContactSegmentFilterGroup{{Filters: []hubspot.ContactSegmentFilter{{Property: "ns_onboarding_status", Operator: "is_equal_to", PropertyKind: "text", Value: "ready"}}}}) {
			t.Fatal("could not recover failed fake processing state")
		}
		if snapshot, ok := fake.ContactSegmentSnapshot(id); ok && snapshot.ProcessingStatus == "FAILED" {
			// A subsequent supported external completion restores the observed
			// status so cleanup can exercise the provider's terminal path.
			fake.PurgeContactSegment(id)
			session.Refresh(updated)
			session.Apply(updated)
			id = session.OpaqueStateString("hubspot_contact_segment.dynamic", "id")
		}

		fake.FailNextContactSegmentOperation(acceptance.ContactSegmentFaultDeleteRejected)
		session.RequireDestroyFailure(updated)
		fake.FailNextContactSegmentOperation(acceptance.ContactSegmentFaultDeleteApplied)
		session.Destroy(updated)
		terminal, ok := fake.ContactSegmentSnapshot(id)
		if !ok || !terminal.Deleted || terminal.ID != id {
			t.Fatalf("terminal Contact segment = %#v, %v", terminal, ok)
		}
	})
}

func runHermeticContactSegmentLifecycle(t *testing.T, engine acceptance.Engine, providerSource string) {
	t.Helper()
	if _, err := exec.LookPath(string(engine)); err != nil {
		t.Skipf("pinned %s executable is not installed", engine)
	}
	fake := acceptance.NewFakeHubSpot(hermeticToken, 666000669)
	server := httptest.NewServer(fake)
	t.Cleanup(server.Close)
	t.Setenv("HUBSPOT_ACCESS_TOKEN", hermeticToken)
	initial := hermeticContactSegmentConfig(server.URL, providerSource, "Initial", "text", "active", "enterprise")
	acceptance.Run(t, acceptance.Options{
		Engine: engine, Shard: acceptance.ContactSegments,
		Prefix: "tf_acc_hermetic_segments_", LedgerPath: t.TempDir() + "/cleanup.jsonl", ProbeBaseURL: server.URL,
	}, func(session *acceptance.Session) {
		session.Apply(initial)
		manualID := session.OpaqueStateString("hubspot_contact_segment.manual", "id")
		dynamicID := session.OpaqueStateString("hubspot_contact_segment.dynamic", "id")
		snapshotID := session.OpaqueStateString("hubspot_contact_segment.snapshot", "id")
		for _, id := range []string{manualID, dynamicID, snapshotID} {
			creates, _, _, _, _, _ := fake.ContactSegmentRequestCounts(id)
			if creates != 1 {
				t.Fatalf("Contact segment %s create writes = %d, want 1", id, creates)
			}
		}
		session.RequireEmptyPlan(initial)

		renamed := hermeticContactSegmentConfig(server.URL, providerSource, "Renamed", "text", "active", "enterprise")
		session.Apply(renamed)
		for address, id := range map[string]string{
			"hubspot_contact_segment.manual": manualID, "hubspot_contact_segment.dynamic": dynamicID, "hubspot_contact_segment.snapshot": snapshotID,
		} {
			session.RequireStateString(address, "id", id)
		}
		for _, id := range []string{manualID, dynamicID, snapshotID} {
			_, _, names, _, _, _ := fake.ContactSegmentRequestCounts(id)
			if names != 1 {
				t.Fatalf("Contact segment %s name writes = %d, want 1", id, names)
			}
		}

		drift := []hubspot.ContactSegmentFilterGroup{{Filters: []hubspot.ContactSegmentFilter{{
			Property: "ns_onboarding_status", Operator: "is_equal_to", PropertyKind: "text", Value: "drifted",
		}}}}
		if !fake.DriftContactSegmentFilters(dynamicID, drift) {
			t.Fatal("could not inject dynamic Contact segment drift")
		}
		session.RequirePlanDiffAttributes(renamed, "hubspot_contact_segment.dynamic", "filter_groups")
		session.Apply(renamed)
		if _, filters, _, _ := fake.ContactSegmentWriteCounts(dynamicID); filters != 1 {
			t.Fatalf("dynamic drift repair filter writes = %d, want 1", filters)
		}

		kindChanged := hermeticContactSegmentConfig(server.URL, providerSource, "Renamed", "select", "active", "enterprise")
		session.Apply(kindChanged)
		session.RequireStateString("hubspot_contact_segment.dynamic", "id", dynamicID)
		if _, filters, _, _ := fake.ContactSegmentWriteCounts(dynamicID); filters != 2 {
			t.Fatalf("dynamic property-kind update writes = %d, want 2", filters)
		}

		replaced := hermeticContactSegmentConfig(server.URL, providerSource, "Renamed", "select", "active", "growth")
		session.RequirePlanDiff(replaced)
		session.Apply(replaced)
		replacementID := session.OpaqueStateString("hubspot_contact_segment.snapshot", "id")
		if replacementID == snapshotID {
			t.Fatal("snapshot filter change retained the old generated identity")
		}
		if old, ok := fake.ContactSegmentSnapshot(snapshotID); !ok || !old.Deleted {
			t.Fatal("snapshot replacement did not retain the old exact tombstone")
		}
		if creates, _, _, _, deletes, _ := fake.ContactSegmentRequestCounts(snapshotID); creates != 1 || deletes != 1 {
			t.Fatalf("replaced snapshot request counts = creates:%d deletes:%d", creates, deletes)
		}
		if creates, _, _, _, _, _ := fake.ContactSegmentRequestCounts(replacementID); creates != 1 {
			t.Fatalf("replacement snapshot create writes = %d, want 1", creates)
		}

		session.RemoveState("hubspot_contact_segment.dynamic")
		for _, invalidID := range []string{"0", "01", "dynamic", "name:Renamed dynamic"} {
			session.RequireImportFailure(replaced, "hubspot_contact_segment.dynamic", invalidID, "Invalid Contact segment import ID")
		}
		session.Import("hubspot_contact_segment.dynamic", dynamicID)
		session.Apply(replaced)
		session.RequireStateString("hubspot_contact_segment.dynamic", "id", dynamicID)

		if !fake.DeleteContactSegment(replacementID) {
			t.Fatal("could not create tombstoned import fixture")
		}
		session.RemoveState("hubspot_contact_segment.snapshot")
		session.Import("hubspot_contact_segment.snapshot", replacementID)
		session.Apply(replaced)
		session.RequireStateString("hubspot_contact_segment.snapshot", "id", replacementID)
		if _, _, _, restores := fake.ContactSegmentWriteCounts(replacementID); restores != 1 {
			t.Fatalf("tombstoned import restore writes = %d, want 1", restores)
		}

		if !fake.DeleteContactSegment(manualID) {
			t.Fatal("could not tombstone manual Contact segment out of band")
		}
		session.Refresh(replaced)
		session.RequireStateString("hubspot_contact_segment.manual", "id", manualID)
		session.RequirePlanDiff(replaced)
		session.Apply(replaced)
		session.RequireStateString("hubspot_contact_segment.manual", "id", manualID)
		if _, _, _, _, _, restores := fake.ContactSegmentRequestCounts(manualID); restores != 1 {
			t.Fatalf("out-of-band manual tombstone restore writes = %d, want 1", restores)
		}

		if !fake.PurgeContactSegment(dynamicID) {
			t.Fatal("could not purge dynamic Contact segment")
		}
		session.Refresh(replaced)
		session.RequireStateAbsent("hubspot_contact_segment.dynamic")
		session.Apply(replaced)
		newDynamicID := session.OpaqueStateString("hubspot_contact_segment.dynamic", "id")
		if newDynamicID == dynamicID {
			t.Fatal("purged Contact segment reused its generated identity")
		}

		session.Destroy(replaced)
		if active := fake.ActiveContactSegmentIDs(); len(active) != 0 {
			t.Fatalf("Contact segment destroy left active identities: %#v", active)
		}
		for id, wantDeletes := range map[string]int{manualID: 2, replacementID: 2, newDynamicID: 1} {
			_, _, _, _, deletes, _ := fake.ContactSegmentRequestCounts(id)
			if deletes != wantDeletes {
				t.Fatalf("Contact segment %s delete writes = %d, want %d", id, deletes, wantDeletes)
			}
		}
	})
}

func hermeticContactSegmentSingleConfig(baseURL, providerSource, suffix, dynamicValue string) string {
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

resource "hubspot_contact_segment" "dynamic" {
  name            = "tf_acc_hermetic_segments_recovery_%s"
  processing_type = "dynamic"
  filter_groups = [{
    filters = [{
      property      = "ns_onboarding_status"
      operator      = "is_equal_to"
      property_kind = "text"
      value         = %q
    }]
  }]
}
`, providerSource, baseURL, suffix, dynamicValue)
}

func hermeticContactSegmentConfig(baseURL, providerSource, suffix, dynamicKind, dynamicValue, snapshotValue string) string {
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

resource "hubspot_contact_segment" "manual" {
  name            = "tf_acc_hermetic_segments_%s manual"
  processing_type = "manual"
}

resource "hubspot_contact_segment" "dynamic" {
  name            = "tf_acc_hermetic_segments_%s dynamic"
  processing_type = "dynamic"
  filter_groups = [{
    filters = [{
      property      = "ns_onboarding_status"
      operator      = "is_equal_to"
      property_kind = %q
      value         = %q
    }, {
      property = "email"
      operator = "is_known"
    }]
  }]
}

resource "hubspot_contact_segment" "snapshot" {
  name            = "tf_acc_hermetic_segments_%s snapshot"
  processing_type = "snapshot"
  filter_groups = [{
    filters = [{
      property      = "ns_customer_tier"
      operator      = "is_equal_to"
      property_kind = "select"
      value         = %q
    }]
  }]
}
`, providerSource, baseURL, suffix, suffix, dynamicKind, dynamicValue, suffix, snapshotValue)
}

func TestHermeticContactSegmentModuleLifecycle(t *testing.T) {
	runHermeticContactSegmentModuleLifecycle(t, acceptance.OpenTofu, "registry.opentofu.org/jackemcpherson/hubspot")
}

func TestHermeticContactSegmentModuleLifecycleTerraformParity(t *testing.T) {
	runHermeticContactSegmentModuleLifecycle(t, acceptance.Terraform, "registry.terraform.io/jackemcpherson/hubspot")
}

// runHermeticContactSegmentModuleLifecycle drives the cumulative demo's
// stable-keyed module through the journey that the Northstar demo runs live.
func runHermeticContactSegmentModuleLifecycle(t *testing.T, engine acceptance.Engine, providerSource string) {
	t.Helper()
	if _, err := exec.LookPath(string(engine)); err != nil {
		t.Skipf("pinned %s executable is not installed", engine)
	}
	moduleSource := consumerModuleSource(t, "contact-segment-definition")
	fake := acceptance.NewFakeHubSpot(hermeticToken, 666000670)
	server := httptest.NewServer(fake)
	t.Cleanup(server.Close)
	t.Setenv("HUBSPOT_ACCESS_TOKEN", hermeticToken)
	config := func(label, snapshotEmail string, extra bool) string {
		return hermeticContactSegmentModuleConfig(server.URL, providerSource, moduleSource, label, snapshotEmail, extra)
	}
	baseline := config("baseline", "suppressed@example.com", false)
	revised := config("revised", "suppressed-revised@example.com", false)
	const manualAddress = `module.segments.hubspot_contact_segment.this["all_contacts"]`
	acceptance.Run(t, acceptance.Options{
		Engine: engine, Shard: acceptance.ContactSegments,
		Prefix: "tf_acc_hermetic_segments_module_", LedgerPath: t.TempDir() + "/cleanup.jsonl", ProbeBaseURL: server.URL,
	}, func(session *acceptance.Session) {
		session.GetModules(baseline)
		session.Apply(baseline)
		ids := session.OpaqueOutputStringMap("segment_ids")
		manualID, dynamicID, snapshotID := ids["all_contacts"], ids["completed_onboarding"], ids["contactable_snapshot"]
		if len(ids) != 3 || manualID == "" || dynamicID == "" || snapshotID == "" {
			t.Fatalf("module exposed %d stable-keyed IDs, want 3", len(ids))
		}
		session.RequireEmptyPlan(baseline)

		drift := []hubspot.ContactSegmentFilterGroup{{Filters: []hubspot.ContactSegmentFilter{{
			Property: "ns_onboarding_status", Operator: "is_equal_to", PropertyKind: "select", Value: "in_progress",
		}}}}
		if !fake.DriftContactSegmentFilters(dynamicID, drift) {
			t.Fatal("could not inject dynamic Contact segment drift")
		}
		session.RequirePlanDiffAttributes(baseline, `module.segments.hubspot_contact_segment.this["completed_onboarding"]`, "filter_groups")
		session.Apply(baseline)
		if _, filters, _, _ := fake.ContactSegmentWriteCounts(dynamicID); filters != 1 {
			t.Fatalf("module dynamic drift repair sent %d filter writes, want 1", filters)
		}
		session.RequireEmptyPlan(baseline)

		session.Apply(revised)
		revisedIDs := session.OpaqueOutputStringMap("segment_ids")
		if revisedIDs["all_contacts"] != manualID || revisedIDs["completed_onboarding"] != dynamicID {
			t.Fatal("module renames did not preserve the manual and dynamic identities")
		}
		replacementID := revisedIDs["contactable_snapshot"]
		if replacementID == snapshotID {
			t.Fatal("module snapshot filter change retained the old generated identity")
		}
		if old, ok := fake.ContactSegmentSnapshot(snapshotID); !ok || !old.Deleted {
			t.Fatal("module snapshot replacement did not leave the old exact tombstone")
		}
		for _, id := range []string{manualID, dynamicID} {
			if _, _, names, _, _, _ := fake.ContactSegmentRequestCounts(id); names != 1 {
				t.Fatalf("module rename sent %d name writes, want 1", names)
			}
		}

		session.Apply(config("revised", "suppressed-revised@example.com", true))
		extendedIDs := session.OpaqueOutputStringMap("segment_ids")
		if len(extendedIDs) != 4 || extendedIDs["all_contacts"] != manualID || extendedIDs["completed_onboarding"] != dynamicID || extendedIDs["contactable_snapshot"] != replacementID {
			t.Fatal("adding a stable key re-addressed existing Contact segments")
		}
		session.Apply(revised)
		if remaining := session.OpaqueOutputStringMap("segment_ids"); len(remaining) != 3 || remaining["all_contacts"] != manualID {
			t.Fatal("removing a stable key re-addressed existing Contact segments")
		}

		if !fake.DeleteContactSegment(manualID) {
			t.Fatal("could not tombstone the manual Contact segment")
		}
		session.RemoveState(manualAddress)
		session.Import(manualAddress, manualID)
		session.RequirePlanDiff(revised)
		session.Apply(revised)
		if restored := session.OpaqueOutputStringMap("segment_ids"); restored["all_contacts"] != manualID {
			t.Fatal("tombstoned module import did not restore the same exact identity")
		}
		if _, _, _, _, _, restores := fake.ContactSegmentRequestCounts(manualID); restores != 1 {
			t.Fatalf("tombstoned module import sent %d restore writes, want 1", restores)
		}
		session.RequireEmptyPlan(revised)

		session.Destroy(revised)
		if active := fake.ActiveContactSegmentIDs(); len(active) != 0 {
			t.Fatalf("module destroy left %d active Contact segments", len(active))
		}
		for _, id := range []string{manualID, dynamicID, replacementID} {
			if terminal, ok := fake.ContactSegmentSnapshot(id); !ok || !terminal.Deleted || terminal.ID != id {
				t.Fatal("module destroy did not leave the exact tombstone")
			}
		}
	})
}

func hermeticContactSegmentModuleConfig(baseURL, providerSource, moduleSource, label, snapshotEmail string, extra bool) string {
	extraDefinition := ""
	if extra {
		extraDefinition = `
    champions = {
      name            = "tf_acc_hermetic_segments_module_champions"
      processing_type = "dynamic"
      filter_groups = [{
        filters = [{
          property      = "ns_buyer_role"
          operator      = "is_equal_to"
          property_kind = "select"
          value         = "champion"
        }, {
          property = "email"
          operator = "is_known"
        }]
      }]
    }`
	}
	return fmt.Sprintf(`
terraform {
  required_providers {
    hubspot = {
      source = %[1]q
    }
  }
}

provider "hubspot" {
  api_base_url = %[2]q
}

module "segments" {
  source = %[3]q

  definitions = {
    all_contacts = {
      name            = "tf_acc_hermetic_segments_module_all_%[4]s"
      processing_type = "manual"
    }
    completed_onboarding = {
      name            = "tf_acc_hermetic_segments_module_onboarded_%[4]s"
      processing_type = "dynamic"
      filter_groups = [{
        filters = [{
          property      = "ns_onboarding_status"
          operator      = "is_equal_to"
          property_kind = "select"
          value         = "complete"
        }]
      }]
    }
    contactable_snapshot = {
      name            = "tf_acc_hermetic_segments_module_snapshot_%[4]s"
      processing_type = "snapshot"
      filter_groups = [{
        filters = [{
          property      = "email"
          operator      = "is_not_equal_to"
          property_kind = "text"
          value         = %[5]q
        }]
      }]
    }%[6]s
  }
}

output "segment_ids" {
  value = module.segments.ids
}
`, providerSource, baseURL, moduleSource, label, snapshotEmail, extraDefinition)
}
