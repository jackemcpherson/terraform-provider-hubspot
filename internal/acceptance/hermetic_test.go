// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

//go:build acceptance

// This file drives the canonical hermetic lifecycle (create with state
// assertions, empty-plan re-apply, in-place update, import round-trip, and
// archival-aware destroy) against FakeHubSpot exclusively through
// Options.ProbeBaseURL and the provider's api_base_url attribute, using the
// real tofu/terraform CLIs. It needs no HubSpot credentials: run it with
//
//	go test -tags=acceptance ./internal/acceptance -run '^TestHermetic'
//
// Pipeline lifecycle coverage is intentionally not driven through Terraform
// here: hubspot_pipeline is implemented (internal/provider/pipeline_resource.go)
// but is not registered in Provider.Resources(), so no built provider binary
// recognizes it (see TestProviderRegistersOnlyFreeTierTypes in
// internal/provider). Registering it would be a provider schema shape
// change, which is out of scope for this version. FakeHubSpot still serves
// pipelines/stages faithfully (see fake_hubspot.go and fake_hubspot_test.go),
// consistent with the existing dormant //go:build deferred pipeline harness
// tests in this package, so the fake is ready once pipelines ship.
package acceptance_test

import (
	"context"
	"fmt"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackemcpherson/terraform-provider-hubspot/internal/acceptance"
)

const hermeticToken = "hermetic-sentinel"

func hermeticServer(t *testing.T, portalID int64) string {
	t.Helper()
	server := httptest.NewServer(acceptance.NewFakeHubSpot(hermeticToken, portalID))
	t.Cleanup(server.Close)
	return server.URL
}

func TestHermeticConsumerModuleLifecycle(t *testing.T) {
	runHermeticConsumerModuleLifecycle(t, acceptance.OpenTofu, "registry.opentofu.org/jackemcpherson/hubspot")
}

func TestHermeticConsumerModuleLifecycleTerraformParity(t *testing.T) {
	runHermeticConsumerModuleLifecycle(t, acceptance.Terraform, "registry.terraform.io/jackemcpherson/hubspot")
}

func TestHermeticV016StateUpgradeCompatibility(t *testing.T) {
	runHermeticV016StateUpgradeCompatibility(t, acceptance.OpenTofu, "registry.opentofu.org/jackemcpherson/hubspot")
}

func TestHermeticV016StateUpgradeCompatibilityTerraformParity(t *testing.T) {
	runHermeticV016StateUpgradeCompatibility(t, acceptance.Terraform, "registry.terraform.io/jackemcpherson/hubspot")
}

func runHermeticV016StateUpgradeCompatibility(t *testing.T, engine acceptance.Engine, providerSource string) {
	t.Helper()
	if _, err := exec.LookPath(string(engine)); err != nil {
		t.Skipf("pinned %s executable is not installed", engine)
	}
	server := hermeticServer(t, 444000444)
	fixture, err := os.ReadFile(filepath.Join("testdata", "v0.1.6-property-state.json"))
	if err != nil {
		t.Fatal(err)
	}
	state := strings.ReplaceAll(string(fixture), "__PROVIDER_SOURCE__", providerSource)
	config := hermeticV016CompatibilityConfig(server, providerSource)
	t.Setenv("HUBSPOT_ACCESS_TOKEN", hermeticToken)
	acceptance.Run(t, acceptance.Options{
		Engine: engine, Shard: acceptance.FreeProperties,
		Prefix: "tf_acc_hermetic_v016_", LedgerPath: t.TempDir() + "/cleanup.jsonl", ProbeBaseURL: server,
	}, func(session *acceptance.Session) {
		session.Apply(config)
		session.PushState(state)
		session.RequireEmptyPlan(config)
		session.Destroy(config)
	})
}

// TestCaptureV016PropertyState regenerates the committed state fixture when
// run explicitly with a released v0.1.6 provider binary.
func TestCaptureV016PropertyState(t *testing.T) {
	path := os.Getenv("HUBSPOT_V016_STATE_CAPTURE")
	if path == "" {
		t.Skip("state capture is an explicit maintenance operation")
	}
	server := hermeticServer(t, 444000444)
	t.Setenv("HUBSPOT_ACCESS_TOKEN", hermeticToken)
	config := hermeticV016CompatibilityConfig(server, "registry.terraform.io/jackemcpherson/hubspot")
	acceptance.Run(t, acceptance.Options{
		Engine: acceptance.Terraform, Shard: acceptance.FreeProperties,
		Prefix: "tf_acc_hermetic_v016_", LedgerPath: t.TempDir() + "/cleanup.jsonl", ProbeBaseURL: server,
	}, func(session *acceptance.Session) {
		session.Apply(config)
		state := strings.ReplaceAll(session.PullState(), "registry.terraform.io/jackemcpherson/hubspot", "__PROVIDER_SOURCE__")
		if err := os.WriteFile(path, []byte(state), 0o600); err != nil {
			t.Fatal(err)
		}
		session.Destroy(config)
	})
}

func runHermeticConsumerModuleLifecycle(t *testing.T, engine acceptance.Engine, providerSource string) {
	t.Helper()
	if _, err := exec.LookPath(string(engine)); err != nil {
		t.Skipf("pinned %s executable is not installed", engine)
	}
	demoRepo := os.Getenv("HUBSPOT_DEMO_REPO")
	if demoRepo == "" {
		demoRepo = filepath.Join("..", "..", "..", "terraform-hubspot-demo")
	}
	moduleSource, err := filepath.Abs(filepath.Join(demoRepo, "modules", "crm-schema"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(moduleSource, "main.tf")); err != nil {
		t.Fatalf("consumer crm-schema module is required: %v", err)
	}

	server := hermeticServer(t, 333000333)
	t.Setenv("HUBSPOT_ACCESS_TOKEN", hermeticToken)
	ledger := t.TempDir() + "/cleanup.jsonl"
	acceptance.Run(t, acceptance.Options{
		Engine: engine, Shard: acceptance.FreeProperties,
		Prefix: "tf_acc_hermetic_", LedgerPath: ledger, ProbeBaseURL: server,
	}, func(session *acceptance.Session) {
		initial := hermeticConsumerModuleConfig(server, providerSource, moduleSource, false)
		session.GetModules(initial)
		session.Apply(initial)
		session.RequireEmptyPlan(initial)

		updated := hermeticConsumerModuleConfig(server, providerSource, moduleSource, true)
		session.RequirePlanWarning(updated, acceptance.PropertyOptionValuesChanged)
		session.Apply(updated)
		session.RequireEmptyPlan(updated)
		session.Destroy(updated)
		session.RequirePropertyAbsent("contacts", "tf_acc_hermetic_module_text")
		session.RequirePropertyAbsent("contacts", "tf_acc_hermetic_module_select")
		session.RequirePropertyGroupAbsent("contacts", "tf_acc_hermetic_module_group")
	})
}

// --- property group lifecycle ---

func TestHermeticPropertyGroupLifecycle(t *testing.T) {
	runHermeticPropertyGroupLifecycle(t, acceptance.OpenTofu, "registry.opentofu.org/jackemcpherson/hubspot")
}

func TestHermeticPropertyGroupLifecycleTerraformParity(t *testing.T) {
	runHermeticPropertyGroupLifecycle(t, acceptance.Terraform, "registry.terraform.io/jackemcpherson/hubspot")
}

// --- form definition lifecycle ---

func TestHermeticFormDefinitionLifecycle(t *testing.T) {
	runHermeticFormDefinitionLifecycle(t, acceptance.OpenTofu, "registry.opentofu.org/jackemcpherson/hubspot")
}

func TestHermeticFormDefinitionLifecycleTerraformParity(t *testing.T) {
	runHermeticFormDefinitionLifecycle(t, acceptance.Terraform, "registry.terraform.io/jackemcpherson/hubspot")
}

func TestHermeticFormDefinitionRecovery(t *testing.T) {
	runHermeticFormDefinitionRecovery(t, acceptance.OpenTofu, "registry.opentofu.org/jackemcpherson/hubspot")
}

func TestHermeticFormDefinitionRecoveryTerraformParity(t *testing.T) {
	runHermeticFormDefinitionRecovery(t, acceptance.Terraform, "registry.terraform.io/jackemcpherson/hubspot")
}

func runHermeticFormDefinitionRecovery(t *testing.T, engine acceptance.Engine, providerSource string) {
	t.Helper()
	if _, err := exec.LookPath(string(engine)); err != nil {
		t.Skipf("pinned %s executable is not installed", engine)
	}
	fake := acceptance.NewFakeHubSpot(hermeticToken, 555000556)
	server := httptest.NewServer(fake)
	t.Cleanup(server.Close)
	clients := newFakeHubSpotClients(t, fake, hermeticToken)
	t.Setenv("HUBSPOT_ACCESS_TOKEN", hermeticToken)
	initial := hermeticFormDefinitionConfig(server.URL, providerSource, false)
	updated := hermeticFormDefinitionConfig(server.URL, providerSource, true)
	const address = "hubspot_form_definition.test"
	acceptance.Run(t, acceptance.Options{
		Engine: engine, Shard: acceptance.FreeProperties,
		Prefix: "tf_acc_hermetic_form_recovery_", LedgerPath: t.TempDir() + "/cleanup.jsonl", ProbeBaseURL: server.URL,
	}, func(session *acceptance.Session) {
		fake.FailNextFormOperation(acceptance.FormFaultCreateUnknown)
		session.RequireApplyFailure(initial)
		if fake.FormCreateCount() != 1 {
			t.Fatalf("ambiguous create was sent %d times, want exactly once", fake.FormCreateCount())
		}
		unknownIDs := fake.ActiveFormIDs()
		if len(unknownIDs) != 1 {
			t.Fatalf("ambiguous create produced active IDs %v", unknownIDs)
		}
		session.RequireStateAbsent(address)
		session.Import(address, unknownIDs[0])
		session.RequireEmptyPlan(initial)

		fake.FailNextFormOperation(acceptance.FormFaultUpdateApplied)
		session.Apply(updated)
		if got := fake.FormPatchCount(unknownIDs[0]); got != 1 {
			t.Fatalf("applied ambiguous update sent %d PATCH requests, want 1", got)
		}
		session.RequireEmptyPlan(updated)

		fake.FailNextFormOperation(acceptance.FormFaultUpdateNotApplied)
		session.RequireApplyFailure(initial)
		session.RequireStateString(address, "name", "Hermetic managed form updated")
		if got := fake.FormPatchCount(unknownIDs[0]); got != 2 {
			t.Fatalf("unapplied ambiguous update sent %d PATCH requests, want 2 total", got)
		}
		session.Apply(initial)
		if got := fake.FormPatchCount(unknownIDs[0]); got != 3 {
			t.Fatalf("explicit update retry sent %d PATCH requests, want 3 total", got)
		}

		fake.FailNextFormOperation(acceptance.FormFaultArchiveNotApplied)
		session.RequireDestroyFailure(initial)
		session.RequireStateString(address, "id", unknownIDs[0])
		if got := fake.FormDeleteCount(unknownIDs[0]); got != 1 {
			t.Fatalf("unapplied ambiguous archive sent %d DELETE requests, want 1", got)
		}
		fake.FailNextFormOperation(acceptance.FormFaultArchiveApplied)
		session.Destroy(initial)
		session.RequireStateAbsent(address)
		if got := fake.FormDeleteCount(unknownIDs[0]); got != 2 {
			t.Fatalf("explicit archive retry sent %d DELETE requests, want 2 total", got)
		}

		fake.FailNextFormOperation(acceptance.FormFaultCreateKnown)
		session.Apply(initial)
		knownID := session.OpaqueStateString(address, "id")
		if fake.FormCreateCount() != 2 || knownID == unknownIDs[0] {
			t.Fatal("known-ID ambiguous create did not recover exactly once")
		}
		session.RequireEmptyPlan(initial)
		if err := clients.Forms.Archive(context.Background(), knownID); err != nil {
			t.Fatal(err)
		}
		session.Destroy(initial)
		if fake.FormDeleteCount(knownID) != 1 {
			t.Fatal("destroy replayed DELETE for an already archived form")
		}

		fake.FailNextFormOperation(acceptance.FormFaultCreateUnverifiable)
		session.RequireApplyFailure(initial)
		if fake.FormCreateCount() != 3 {
			t.Fatalf("unverifiable create was sent %d times, want exactly three total creates", fake.FormCreateCount())
		}
		session.RequireStateStringPrefix(address, "id", "00000000-0000-4000-8000-")
		unverifiableID := session.OpaqueStateString(address, "id")
		session.RemoveState(address)
		session.Import(address, unverifiableID)
		session.RequireEmptyPlan(initial)
		if !fake.DisappearForm(unverifiableID) {
			t.Fatal("remove recovered form from both views")
		}
		session.Destroy(initial)
		session.Destroy(initial)
		if fake.FormDeleteCount(unverifiableID) != 0 {
			t.Fatal("destroy sent DELETE for a permanently absent form")
		}
		if active := fake.ActiveFormIDs(); len(active) != 0 {
			t.Fatalf("active forms remained after recovery cleanup: %v", active)
		}
	})
}

func runHermeticFormDefinitionLifecycle(t *testing.T, engine acceptance.Engine, providerSource string) {
	t.Helper()
	if _, err := exec.LookPath(string(engine)); err != nil {
		t.Skipf("pinned %s executable is not installed", engine)
	}
	fake := acceptance.NewFakeHubSpot(hermeticToken, 555000555)
	server := httptest.NewServer(fake)
	t.Cleanup(server.Close)
	clients := newFakeHubSpotClients(t, fake, hermeticToken)
	t.Setenv("HUBSPOT_ACCESS_TOKEN", hermeticToken)
	initial := hermeticFormDefinitionConfig(server.URL, providerSource, false)
	acceptance.Run(t, acceptance.Options{
		Engine: engine, Shard: acceptance.FreeProperties,
		Prefix: "tf_acc_hermetic_form_", LedgerPath: t.TempDir() + "/cleanup.jsonl", ProbeBaseURL: server.URL,
	}, func(session *acceptance.Session) {
		session.Apply(initial)
		session.RequireStateStringPrefix("hubspot_form_definition.test", "id", "00000000-0000-4000-8000-")
		id := session.OpaqueStateString("hubspot_form_definition.test", "id")
		session.RequireEmptyPlan(initial)
		if got := fake.FormPatchCount(id); got != 0 {
			t.Fatalf("semantic no-op sent %d PATCH requests", got)
		}

		invalid := []struct {
			old, replacement, title string
		}{
			{`name = "Hermetic managed form"`, `name = " "`, "Invalid form presentation"},
			{`blocked_email_domains  = []`, `blocked_email_domains  = ["https://example.com"]`, "Invalid blocked email domain"},
			{`language                         = "en"`, `language                         = "fr"`, "Unsupported form language"},
			{`submit_alignment         = "left"`, `submit_alignment         = "right"`, "Unsupported submit alignment"},
			{`submit_color             = "#ff7a59"`, `submit_color             = "#fff"`, "Invalid form color"},
			{`font_family              = "Arial, sans-serif"`, `font_family              = "Arial; color: red"`, "Invalid form font family"},
			{`background_width         = "100%"`, `background_width         = "0%"`, "Invalid form percentage width"},
			{`label_text_size          = "13px"`, `label_text_size          = "0px"`, "Invalid form pixel size"},
			{`submit_size              = "12px 24px"`, `submit_size              = "12px"`, "Invalid form submit size"},
		}
		for _, test := range invalid {
			session.RequirePlanFailure(strings.Replace(initial, test.old, test.replacement, 1), test.title)
		}

		updated := hermeticFormDefinitionConfig(server.URL, providerSource, true)
		session.Apply(updated)
		session.RequireEmptyPlan(updated)
		if got := fake.FormPatchCount(id); got != 1 {
			t.Fatalf("bounded managed update sent %d PATCH requests, want 1", got)
		}

		if !fake.DriftFormPresentation(id) {
			t.Fatal("inject supported form drift")
		}
		session.RequirePlanDiffAttributes(updated, "hubspot_form_definition.test", "configuration", "display_options", "field_groups", "name")
		session.Apply(updated)
		session.RequireEmptyPlan(updated)
		if got := fake.FormPatchCount(id); got != 2 {
			t.Fatalf("drift repair sent %d total PATCH requests, want 2", got)
		}

		if !fake.AddFormUnknownMetadata(id) {
			t.Fatal("inject harmless form metadata")
		}
		session.RequireEmptyPlan(updated)
		if got := fake.FormPatchCount(id); got != 2 {
			t.Fatalf("harmless metadata caused a PATCH; count = %d", got)
		}

		if !fake.InjectUnsupportedFormStructure(id) {
			t.Fatal("inject unsupported form structure")
		}
		session.RequirePlanFailure(updated, "Unsupported HubSpot form definition")
		session.RequireStateString("hubspot_form_definition.test", "name", "Hermetic managed form updated")
		if got := fake.FormPatchCount(id); got != 2 {
			t.Fatalf("unsupported drift caused a PATCH; count = %d", got)
		}
		if !fake.ClearUnsupportedFormStructure(id) {
			t.Fatal("clear unsupported form structure")
		}

		const address = "hubspot_form_definition.test"
		session.RemoveState(address)
		session.Import(address, id)
		session.RequireEmptyPlan(updated)
		session.RemoveState(address)
		for _, invalidID := range []string{
			"Hermetic managed form updated",
			id + "/presentation",
			server.URL + "/marketing/v3/forms/" + id,
			"ABCDEFAB-CDEF-ABCD-EFAB-CDEFABCDEFAB",
		} {
			session.RequireImportFailure(updated, address, invalidID, "Invalid form definition import ID")
		}

		ctx := context.Background()
		duplicateOne, err := clients.Forms.Create(ctx, fakeFormWrite())
		if err != nil {
			t.Fatal(err)
		}
		duplicateTwo, err := clients.Forms.Create(ctx, fakeFormWrite())
		if err != nil {
			t.Fatal(err)
		}
		if duplicateOne.ID == duplicateTwo.ID || duplicateOne.Name != duplicateTwo.Name {
			t.Fatal("duplicate active form names did not retain distinct generated identities")
		}
		archivedFixture, err := clients.Forms.Create(ctx, fakeFormWrite())
		if err != nil {
			t.Fatal(err)
		}
		if err := clients.Forms.Archive(ctx, archivedFixture.ID); err != nil {
			t.Fatal(err)
		}
		unsupportedFixture, err := clients.Forms.Create(ctx, fakeFormWrite())
		if err != nil || !fake.InjectUnsupportedFormStructure(unsupportedFixture.ID) {
			t.Fatal("create unsupported import fixture")
		}
		nonHubSpotFixture, err := clients.Forms.Create(ctx, fakeFormWrite())
		if err != nil || !fake.InjectNonHubSpotForm(nonHubSpotFixture.ID) {
			t.Fatal("create non-HubSpot import fixture")
		}
		createCount := fake.FormCreateCount()
		session.RequireImportFailure(updated, address, archivedFixture.ID, "Archived form definition cannot be imported")
		session.RequireImportFailure(updated, address, unsupportedFixture.ID, "Unsupported HubSpot form definition")
		session.RequireImportFailure(updated, address, nonHubSpotFixture.ID, "Unsupported HubSpot form definition")
		if fake.FormCreateCount() != createCount || fake.FormPatchCount(unsupportedFixture.ID) != 0 || fake.FormPatchCount(nonHubSpotFixture.ID) != 0 {
			t.Fatal("failed import mutated remote form definitions")
		}
		session.Import(address, id)
		session.RequireEmptyPlan(updated)
		for _, fixtureID := range []string{duplicateOne.ID, duplicateTwo.ID, unsupportedFixture.ID, nonHubSpotFixture.ID} {
			if err := clients.Forms.Archive(ctx, fixtureID); err != nil {
				t.Fatalf("archive import fixture %s: %v", fixtureID, err)
			}
		}

		if err := clients.Forms.Archive(ctx, id); err != nil {
			t.Fatal(err)
		}
		session.Refresh(updated)
		session.RequireStateAbsent(address)
		session.Apply(updated)
		recreatedAfterArchive := session.OpaqueStateString(address, "id")
		if recreatedAfterArchive == id {
			t.Fatal("external archival reused the prior generated form ID")
		}
		if !fake.DisappearForm(recreatedAfterArchive) {
			t.Fatal("remove form from both active and archived views")
		}
		session.Refresh(updated)
		session.RequireStateAbsent(address)
		session.Apply(updated)
		recreatedAfterAbsence := session.OpaqueStateString(address, "id")
		if recreatedAfterAbsence == recreatedAfterArchive {
			t.Fatal("complete disappearance reused the missing generated form ID")
		}
		session.Destroy(updated)
		session.Destroy(updated)
		session.RequireStateAbsent(address)
		session.RequireFormArchived(recreatedAfterAbsence)
		if active := fake.ActiveFormIDs(); len(active) != 0 {
			t.Fatalf("active forms remained after cleanup: %v", active)
		}
	})
}

func runHermeticPropertyGroupLifecycle(t *testing.T, engine acceptance.Engine, providerSource string) {
	t.Helper()
	if _, err := exec.LookPath(string(engine)); err != nil {
		t.Skipf("pinned %s executable is not installed", engine)
	}

	server := hermeticServer(t, 111000111)
	t.Setenv("HUBSPOT_ACCESS_TOKEN", hermeticToken)
	ledger := t.TempDir() + "/cleanup.jsonl"

	acceptance.Run(t, acceptance.Options{
		Engine:       engine,
		Shard:        acceptance.FreeProperties,
		Prefix:       "tf_acc_hermetic_",
		LedgerPath:   ledger,
		ProbeBaseURL: server,
	}, func(session *acceptance.Session) {
		name := "tf_acc_hermetic_group"
		initial := hermeticPropertyGroupConfig(server, providerSource, name, "Initial label", 10)
		session.Apply(initial)
		session.RequireStateString("hubspot_property_group.test", "id", "contacts/"+name)
		session.RequireStateString("hubspot_property_group.test", "label", "Initial label")
		session.RequireEmptyPlan(initial)

		updated := hermeticPropertyGroupConfig(server, providerSource, name, "Updated label", 20)
		session.Apply(updated)
		session.RequireStateString("hubspot_property_group.test", "label", "Updated label")
		session.RequireEmptyPlan(updated)

		// Drift via out-of-band fake mutation.
		driftOrder := int64(30)
		session.MutatePropertyGroup("contacts", name, "Out-of-band label", &driftOrder)
		session.RequirePlanDiffAttributes(updated, "hubspot_property_group.test", "display_order", "label")
		session.Apply(updated)
		session.RequireEmptyPlan(updated)

		// Import round-trip.
		session.RemoveState("hubspot_property_group.test")
		session.Import("hubspot_property_group.test", "contacts/"+name)
		session.RequireEmptyPlan(updated)

		// Removed-resource recovery: the group vanishes out of band and the
		// next refresh clears state; a subsequent apply recreates it.
		session.ArchivePropertyGroup("contacts", name)
		session.Refresh(updated)
		session.RequireStateAbsent("hubspot_property_group.test")
		session.Apply(updated)
		session.RequireEmptyPlan(updated)

		// Destroy with archival-aware verification.
		session.Destroy(updated)
		session.RequirePropertyGroupAbsent("contacts", name)
		// Archived group names are not reserved: recreating it must succeed.
		session.RequirePropertyGroupReusable("contacts", name)
	})
}

func TestHermeticPropertyGroupBlockedDestroy(t *testing.T) {
	runHermeticPropertyGroupBlockedDestroy(t, acceptance.OpenTofu, "registry.opentofu.org/jackemcpherson/hubspot")
}

func TestHermeticPropertyGroupBlockedDestroyTerraformParity(t *testing.T) {
	runHermeticPropertyGroupBlockedDestroy(t, acceptance.Terraform, "registry.terraform.io/jackemcpherson/hubspot")
}

func runHermeticPropertyGroupBlockedDestroy(t *testing.T, engine acceptance.Engine, providerSource string) {
	t.Helper()
	if _, err := exec.LookPath(string(engine)); err != nil {
		t.Skipf("pinned %s executable is not installed", engine)
	}

	server := hermeticServer(t, 111000222)
	t.Setenv("HUBSPOT_ACCESS_TOKEN", hermeticToken)
	ledger := t.TempDir() + "/cleanup.jsonl"

	acceptance.Run(t, acceptance.Options{
		Engine:       engine,
		Shard:        acceptance.FreeProperties,
		Prefix:       "tf_acc_hermetic_",
		LedgerPath:   ledger,
		ProbeBaseURL: server,
	}, func(session *acceptance.Session) {
		groupName := "tf_acc_hermetic_blocked_group"
		propertyName := "tf_acc_hermetic_blocked_property"
		initial := hermeticBlockedGroupConfig(server, providerSource, groupName, propertyName, true)
		session.Apply(initial)
		// Group-deletion-with-active-properties failure: removing the group
		// from config while its property remains active must fail with the
		// harness's canonical failure identity, and retain state.
		session.RequireApplyFailureWithStatus(hermeticBlockedGroupConfig(server, providerSource, groupName, propertyName, false), acceptance.PropertyGroupHasActiveProperties)
		session.RequireStateString("hubspot_property_group.blocked", "label", "Hermetic blocked group")
		session.Destroy(initial)
	})
}

// --- property definition lifecycle ---

func TestHermeticPropertyDefinitionLifecycle(t *testing.T) {
	runHermeticPropertyDefinitionLifecycle(t, acceptance.OpenTofu, "registry.opentofu.org/jackemcpherson/hubspot")
}

func TestHermeticPropertyDefinitionLifecycleTerraformParity(t *testing.T) {
	runHermeticPropertyDefinitionLifecycle(t, acceptance.Terraform, "registry.terraform.io/jackemcpherson/hubspot")
}

func runHermeticPropertyDefinitionLifecycle(t *testing.T, engine acceptance.Engine, providerSource string) {
	t.Helper()
	if _, err := exec.LookPath(string(engine)); err != nil {
		t.Skipf("pinned %s executable is not installed", engine)
	}

	server := hermeticServer(t, 222000222)
	t.Setenv("HUBSPOT_ACCESS_TOKEN", hermeticToken)
	ledger := t.TempDir() + "/cleanup.jsonl"

	acceptance.Run(t, acceptance.Options{
		Engine:       engine,
		Shard:        acceptance.FreeProperties,
		Prefix:       "tf_acc_hermetic_",
		LedgerPath:   ledger,
		ProbeBaseURL: server,
	}, func(session *acceptance.Session) {
		for _, objectType := range []string{"contacts", "companies", "deals", "tickets"} {
			groupName := "tf_acc_hermetic_" + objectType + "_group"
			textName := "tf_acc_hermetic_" + objectType + "_text"
			selectName := "tf_acc_hermetic_" + objectType + "_select"
			initial := hermeticObjectSchemaConfig(server, providerSource, objectType, groupName, textName, selectName, false)
			session.Apply(initial)
			session.RequireStateString("hubspot_property.text", "id", objectType+"/"+textName)
			session.RequireStateString("hubspot_property.text", "type", "string")
			session.RequireStateString("hubspot_property.select", "field_type", "select")
			session.RequireEmptyPlan(initial)

			updated := hermeticObjectSchemaConfig(server, providerSource, objectType, groupName, textName, selectName, true)
			session.RequirePlanWarning(updated, acceptance.PropertyOptionValuesChanged)
			session.Apply(updated)
			session.RequireEmptyPlan(updated)

			// Drift via out-of-band fake mutation.
			session.MutatePropertyLabel(objectType, textName, "Out-of-band label")
			session.RequirePlanDiffAttributes(updated, "hubspot_property.text", "label")
			session.Apply(updated)
			session.RequireEmptyPlan(updated)

			// Direct group and property import round-trips use canonical IDs.
			session.RemoveState("hubspot_property_group.test")
			session.Import("hubspot_property_group.test", objectType+"/"+groupName)
			session.RemoveState("hubspot_property.text")
			session.Import("hubspot_property.text", objectType+"/"+textName)
			session.RequireEmptyPlan(updated)

			// Refresh removes an out-of-band archived property, and apply
			// immediately reuses its immutable name while retaining tombstone visibility.
			session.ArchiveProperty(objectType, textName)
			session.Refresh(updated)
			session.RequireStateAbsent("hubspot_property.text")
			session.Apply(updated)
			session.RequirePropertyArchived(objectType, textName)
			session.RequireEmptyPlan(updated)

			session.Destroy(updated)
			session.RequirePropertyAbsent(objectType, textName)
			session.RequirePropertyAbsent(objectType, selectName)
			session.RequirePropertyArchived(objectType, textName)
			session.RequirePropertyArchived(objectType, selectName)
			session.RequirePropertyGroupAbsent(objectType, groupName)
			session.RequirePropertyGroupReusable(objectType, groupName)
		}
	})
}

// --- shared config builders ---

func hermeticFormDefinitionConfig(apiBaseURL, providerSource string, updated bool) string {
	name := "Hermetic managed form"
	label := "Email address"
	description := "Contact email"
	placeholder := "name@example.com"
	required := true
	blockedDomains := "[]"
	useDefaultBlockList := true
	allowReset := false
	prePopulate := false
	recaptcha := true
	thankYou := "Thank you"
	submitText := "Submit"
	labelSize := "13px"
	labelColor := "#33475b"
	legalSize := "12px"
	legalColor := "#33475b"
	helpSize := "11px"
	helpColor := "#516f90"
	fontFamily := "Arial, sans-serif"
	backgroundWidth := "100%"
	submitFontColor := "#ffffff"
	submitAlignment := "left"
	submitSize := "12px 24px"
	submitColor := "#ff7a59"
	if updated {
		name = "Hermetic managed form updated"
		label = "Work email"
		description = "Updated contact email"
		placeholder = "work@example.com"
		required = false
		blockedDomains = `["example.com"]`
		useDefaultBlockList = false
		allowReset = true
		prePopulate = true
		recaptcha = false
		thankYou = "Updated thank you"
		submitText = "Send"
		labelSize = "14px"
		labelColor = "#123456"
		legalSize = "13px"
		legalColor = "#234567"
		helpSize = "12px"
		helpColor = "#345678"
		fontFamily = "Helvetica Neue, sans-serif"
		backgroundWidth = "95.5%"
		submitFontColor = "#456789"
		submitAlignment = "center"
		submitSize = "10px 20px"
		submitColor = "#00a4bd"
	}
	return fmt.Sprintf(`
terraform {
  required_providers {
    hubspot = {
      source = %q
    }
  }
}

provider "hubspot" {
  access_token = %q
  api_base_url = %q
}

resource "hubspot_form_definition" "test" {
  name = %q

  field_groups = [{
    fields = [{
      label                  = %q
      description            = %q
      placeholder            = %q
      required               = %t
      blocked_email_domains  = %s
      use_default_block_list = %t
    }]
  }]

  configuration = {
    language                         = "en"
    allow_link_to_reset_known_values = %t
    pre_populate_known_values        = %t
    recaptcha_enabled                = %t
    thank_you_text                   = %q
  }

  display_options = {
    submit_button_text = %q
    style = {
      label_text_size          = %q
      label_text_color         = %q
      legal_consent_text_size  = %q
      legal_consent_text_color = %q
      help_text_size           = %q
      help_text_color          = %q
      font_family              = %q
      background_width         = %q
      submit_font_color        = %q
      submit_alignment         = %q
      submit_size              = %q
      submit_color             = %q
    }
  }
}
`, providerSource, hermeticToken, apiBaseURL, name, label, description, placeholder, required, blockedDomains, useDefaultBlockList,
		allowReset, prePopulate, recaptcha, thankYou, submitText, labelSize, labelColor, legalSize, legalColor, helpSize, helpColor,
		fontFamily, backgroundWidth, submitFontColor, submitAlignment, submitSize, submitColor)
}

func hermeticConsumerModuleConfig(apiBaseURL, providerSource, moduleSource string, updated bool) string {
	groupLabel := "Hermetic module group"
	textDescription := ""
	options := `
      alpha = { label = "Alpha", display_order = 10 }
      beta  = { label = "Beta", display_order = 20 }
`
	if updated {
		groupLabel = "Updated hermetic module group"
		textDescription = "Updated through the consumer module"
		options = `
      alpha = { label = "Alpha updated", display_order = 10, hidden = true }
      gamma = { label = "Gamma", description = "Added option", display_order = 30 }
`
	}
	return fmt.Sprintf(`
terraform {
  required_providers {
    hubspot = {
      source = %q
    }
  }
}

provider "hubspot" {
  access_token = %q
  api_base_url = %q
}

module "schema" {
  source      = %q
  object_type = "contacts"
  groups = {
    tf_acc_hermetic_module_group = {
      label         = %q
      display_order = 10
    }
  }
  properties = {
    tf_acc_hermetic_module_text = {
      label         = "Hermetic module text"
      group         = "tf_acc_hermetic_module_group"
      description   = %q
      display_order = 20
    }
    tf_acc_hermetic_module_select = {
      label         = "Hermetic module select"
      group         = "tf_acc_hermetic_module_group"
      kind          = "select"
      display_order = 30
      options = {%s
      }
    }
  }
}
`, providerSource, hermeticToken, apiBaseURL, moduleSource, groupLabel, textDescription, options)
}

func hermeticV016CompatibilityConfig(apiBaseURL, providerSource string) string {
	return fmt.Sprintf(`
terraform {
  required_providers {
    hubspot = {
      source = %q
    }
  }
}

provider "hubspot" {
  access_token = %q
  api_base_url = %q
}

resource "hubspot_property_group" "v016" {
  object_type   = "contacts"
  name          = "tf_acc_hermetic_v016_group"
  label         = "v0.1.6 group"
  display_order = 10
}

resource "hubspot_property" "v016" {
  object_type = "contacts"
  name        = "tf_acc_hermetic_v016_property"
  label       = "v0.1.6 property"
  group_name  = hubspot_property_group.v016.name
  type        = "string"
  field_type  = "text"
}
`, providerSource, hermeticToken, apiBaseURL)
}

func hermeticPropertyGroupConfig(apiBaseURL, providerSource, name, label string, displayOrder int64) string {
	return fmt.Sprintf(`
terraform {
  required_providers {
    hubspot = {
      source = %q
    }
  }
}

provider "hubspot" {
  access_token = %q
  api_base_url = %q
}

resource "hubspot_property_group" "test" {
  object_type   = "contacts"
  name          = %q
  label         = %q
  display_order = %d
}
`, providerSource, hermeticToken, apiBaseURL, name, label, displayOrder)
}

func hermeticBlockedGroupConfig(apiBaseURL, providerSource, groupName, propertyName string, includeGroup bool) string {
	group := ""
	dependency := ""
	if includeGroup {
		group = fmt.Sprintf(`
resource "hubspot_property_group" "blocked" {
  object_type   = "contacts"
  name          = %q
  label         = "Hermetic blocked group"
  display_order = -1
}
`, groupName)
		dependency = "depends_on = [hubspot_property_group.blocked]"
	}
	return fmt.Sprintf(`
terraform {
  required_providers {
    hubspot = {
      source = %q
    }
  }
}

provider "hubspot" {
  access_token = %q
  api_base_url = %q
}
%s
resource "hubspot_property" "blocker" {
  object_type = "contacts"
  name        = %q
  label       = "Hermetic group deletion blocker"
  group_name  = %q
  type        = "string"
  field_type  = "text"
  %s
}
`, providerSource, hermeticToken, apiBaseURL, group, propertyName, groupName, dependency)
}

func hermeticObjectSchemaConfig(apiBaseURL, providerSource, objectType, groupName, textName, selectName string, updated bool) string {
	groupLabel := "Hermetic property schema"
	textLabel := "Hermetic text"
	textDescription := ""
	textOrder := int64(10)
	textHidden := false
	alphaLabel := "Alpha"
	alphaOrder := int64(20)
	options := `
    alpha = { label = %q, display_order = %d }
    beta  = { label = "Beta", display_order = 30 }
`
	if updated {
		groupLabel = "Updated hermetic property schema"
		textLabel = "Updated hermetic text"
		textDescription = "Updated through the provider"
		textOrder = 15
		textHidden = true
		alphaLabel = "Alpha updated"
		alphaOrder = 25
		options = `
    alpha = { label = %q, display_order = %d, hidden = true }
    gamma = { label = "Gamma", description = "Added option", display_order = 35 }
`
	}
	return fmt.Sprintf(`
terraform {
  required_providers {
    hubspot = {
      source = %q
    }
  }
}

provider "hubspot" {
  access_token = %q
  api_base_url = %q
}

resource "hubspot_property_group" "test" {
  object_type = %q
  name        = %q
  label       = %q
  display_order = 5
}

resource "hubspot_property" "text" {
  object_type = %q
  name        = %q
  label       = %q
  group_name  = hubspot_property_group.test.name
  type        = "string"
  field_type  = "text"
  description = %q
  display_order = %d
  hidden      = %t
}

resource "hubspot_property" "select" {
  object_type = %q
  name        = %q
  label       = "Hermetic select"
  group_name  = hubspot_property_group.test.name
  type        = "enumeration"
  field_type  = "select"

  depends_on = [hubspot_property.text]

  options = {%s
  }
}
`, providerSource, hermeticToken, apiBaseURL, objectType, groupName, groupLabel, objectType, textName, textLabel, textDescription, textOrder, textHidden, objectType, selectName, fmt.Sprintf(options, alphaLabel, alphaOrder))
}
