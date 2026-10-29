// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

//go:build acceptance

package acceptance

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/jackemcpherson/terraform-provider-hubspot/internal/hubspot"
)

func TestFakeHubSpotContactSegmentsPreserveExactTombstoneIdentity(t *testing.T) {
	fake := NewFakeHubSpot("token", 123)
	clients := fakeContactSegmentClients(t, fake)
	ctx := context.Background()
	created, err := clients.ContactSegments.Create(ctx, hubspot.ContactSegmentWrite{
		Name: "tf_acc_owned_dynamic", ProcessingType: "dynamic",
		FilterGroups: []hubspot.ContactSegmentFilterGroup{{Filters: []hubspot.ContactSegmentFilter{
			{Property: "ns_onboarding_status", Operator: "is_equal_to", PropertyKind: "text", Value: "active"},
			{Property: "email", Operator: "is_known"},
		}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	read, err := clients.ContactSegments.Get(ctx, created.ID)
	if err != nil || read.ID != created.ID || read.Deleted || read.Version != 1 {
		t.Fatalf("created/read = %#v/%#v, %v", created, read, err)
	}
	updated, err := clients.ContactSegments.UpdateFilters(ctx, created.ID, []hubspot.ContactSegmentFilterGroup{{Filters: []hubspot.ContactSegmentFilter{{
		Property: "ns_onboarding_status", Operator: "is_not_equal_to", PropertyKind: "text", Value: "inactive",
	}}}})
	if err != nil || updated.ID != created.ID || updated.Version != 2 {
		t.Fatalf("updated = %#v, %v", updated, err)
	}
	if err := clients.ContactSegments.Delete(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
	tombstone, err := clients.ContactSegments.Get(ctx, created.ID)
	if err != nil || tombstone.ID != created.ID || !tombstone.Deleted || tombstone.Version != 3 {
		t.Fatalf("tombstone = %#v, %v", tombstone, err)
	}
	if err := clients.ContactSegments.Restore(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
	restored, err := clients.ContactSegments.Get(ctx, created.ID)
	if err != nil || restored.ID != created.ID || restored.Deleted || restored.Version != 4 {
		t.Fatalf("restored = %#v, %v", restored, err)
	}
	_, filters, deletes, restores := fake.ContactSegmentWriteCounts(created.ID)
	if filters != 1 || deletes != 1 || restores != 1 {
		t.Fatalf("writes = filters:%d deletes:%d restores:%d", filters, deletes, restores)
	}
}

func TestFakeHubSpotContactSegmentFaultsAreAppliedOnce(t *testing.T) {
	fake := NewFakeHubSpot("token", 123)
	clients := fakeContactSegmentClients(t, fake)
	ctx := context.Background()
	fake.FailNextContactSegmentOperation(ContactSegmentFaultCreateKnown)
	created, err := clients.ContactSegments.Create(ctx, hubspot.ContactSegmentWrite{Name: "tf_acc_known", ProcessingType: "manual"})
	if created.ID == "" || err == nil {
		t.Fatalf("known ambiguous create = %#v, %v", created, err)
	}
	read, readErr := clients.ContactSegments.Get(ctx, created.ID)
	if readErr != nil || read.ID != created.ID {
		t.Fatalf("known ambiguous read-back = %#v, %v", read, readErr)
	}
	clean, err := clients.ContactSegments.Create(ctx, hubspot.ContactSegmentWrite{Name: "tf_acc_clean", ProcessingType: "manual"})
	if err != nil || clean.ID == "" {
		t.Fatalf("one-shot fault leaked = %#v, %v", clean, err)
	}
	fake.FailNextContactSegmentOperation(ContactSegmentFaultCreateUnknown)
	unknown, err := clients.ContactSegments.Create(ctx, hubspot.ContactSegmentWrite{Name: "tf_acc_unknown", ProcessingType: "manual"})
	var apiError *hubspot.Error
	if unknown.ID != "" || !errors.As(err, &apiError) || !apiError.Ambiguous {
		t.Fatalf("unknown ambiguous create = %#v, %#v", unknown, err)
	}
	if ids := fake.ContactSegmentIDsByNamePrefix("tf_acc_unknown"); len(ids) != 1 {
		t.Fatalf("unknown ambiguous create identities = %#v", ids)
	}
}

func TestFakeHubSpotContactSegmentModelsPendingStaleAndTimeoutReads(t *testing.T) {
	fake := NewFakeHubSpot("token", 123)
	clients := fakeContactSegmentClients(t, fake)
	ctx := context.Background()
	created, err := clients.ContactSegments.Create(ctx, hubspot.ContactSegmentWrite{
		Name: "tf_acc_async", ProcessingType: "dynamic",
		FilterGroups: contactSegmentFiltersForFakeTest("active"),
	})
	if err != nil {
		t.Fatal(err)
	}

	fake.FailNextContactSegmentOperation(ContactSegmentFaultUpdatePending)
	if _, err := clients.ContactSegments.UpdateFilters(ctx, created.ID, contactSegmentFiltersForFakeTest("pending")); err != nil {
		t.Fatal(err)
	}
	for read := 0; read < 3; read++ {
		segment, err := clients.ContactSegments.Get(ctx, created.ID)
		if err != nil {
			t.Fatal(err)
		}
		want := "PROCESSING"
		if read == 2 {
			want = "COMPLETE"
		}
		if segment.ProcessingStatus != want {
			t.Fatalf("pending read %d status = %q, want %q", read+1, segment.ProcessingStatus, want)
		}
	}

	fake.FailNextContactSegmentOperation(ContactSegmentFaultUpdateStale)
	if _, err := clients.ContactSegments.UpdateFilters(ctx, created.ID, contactSegmentFiltersForFakeTest("current")); err != nil {
		t.Fatal(err)
	}
	for read := 0; read < 3; read++ {
		segment, err := clients.ContactSegments.Get(ctx, created.ID)
		if err != nil {
			t.Fatal(err)
		}
		want := "pending"
		if read == 2 {
			want = "current"
		}
		if got := segment.FilterGroups[0].Filters[0].Value; got != want {
			t.Fatalf("stale read %d value = %q, want %q", read+1, got, want)
		}
	}

	fake.FailNextContactSegmentOperation(ContactSegmentFaultUpdateTimeout)
	if _, err := clients.ContactSegments.UpdateFilters(ctx, created.ID, contactSegmentFiltersForFakeTest("timeout")); err != nil {
		t.Fatal(err)
	}
	for read := 0; read < 3; read++ {
		segment, err := clients.ContactSegments.Get(ctx, created.ID)
		if err != nil || segment.ProcessingStatus != "PROCESSING" {
			t.Fatalf("timeout read %d = %#v, %v", read+1, segment, err)
		}
	}
	creates, reads, names, filters, deletes, restores := fake.ContactSegmentRequestCounts(created.ID)
	if creates != 1 || reads != 9 || names != 0 || filters != 3 || deletes != 0 || restores != 0 {
		t.Fatalf("request counts = %d/%d/%d/%d/%d/%d", creates, reads, names, filters, deletes, restores)
	}
}

func TestFakeHubSpotContactSegmentRecoveryHooks(t *testing.T) {
	fake := NewFakeHubSpot("token", 123)
	clients := fakeContactSegmentClients(t, fake)
	ctx := context.Background()
	created, err := clients.ContactSegments.Create(ctx, hubspot.ContactSegmentWrite{
		Name: "tf_acc_hooks", ProcessingType: "dynamic", FilterGroups: contactSegmentFiltersForFakeTest("active"),
	})
	if err != nil {
		t.Fatal(err)
	}

	if !fake.SetContactSegmentUnsupported(created.ID, true) {
		t.Fatal("could not mark the dynamic Contact segment unsupported")
	}
	if _, err := clients.ContactSegments.Get(ctx, created.ID); err == nil {
		t.Fatal("unsupported remote filter was accepted by exact-ID read")
	}
	if !fake.SetContactSegmentUnsupported(created.ID, false) {
		t.Fatal("could not clear the unsupported Contact segment filter")
	}
	if _, err := clients.ContactSegments.Get(ctx, created.ID); err != nil {
		t.Fatalf("supported read after clearing = %v", err)
	}

	if !fake.DeleteContactSegment(created.ID) || !fake.RestoreContactSegment(created.ID) {
		t.Fatal("could not tombstone and restore out of band")
	}
	restored, err := clients.ContactSegments.Get(ctx, created.ID)
	if err != nil || restored.Deleted || restored.Version != 3 {
		t.Fatalf("out-of-band restore = %#v, %v", restored, err)
	}
	if _, _, _, restores := fake.ContactSegmentWriteCounts(created.ID); restores != 0 {
		t.Fatalf("out-of-band restore counted %d provider restore writes", restores)
	}

	if !fake.DeleteContactSegment(created.ID) {
		t.Fatal("could not tombstone before incomplete restore")
	}
	fake.FailNextContactSegmentOperation(ContactSegmentFaultRestoreIncomplete)
	if err := clients.ContactSegments.Restore(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		incomplete, err := clients.ContactSegments.Get(ctx, created.ID)
		if err != nil || incomplete.Deleted || incomplete.Name == "tf_acc_hooks" || incomplete.Version != 5 {
			t.Fatalf("incomplete restore observation = %#v, %v", incomplete, err)
		}
	}
	complete, err := clients.ContactSegments.Get(ctx, created.ID)
	if err != nil || complete.Name != "tf_acc_hooks" || complete.Version != 5 {
		t.Fatalf("complete restore observation = %#v, %v", complete, err)
	}

	fake.FailNextContactSegmentOperation(ContactSegmentFaultDeleteDisappears)
	var apiError *hubspot.Error
	if err := clients.ContactSegments.Delete(ctx, created.ID); !errors.As(err, &apiError) || apiError.Status != http.StatusNotFound {
		t.Fatalf("disappearing delete = %v", err)
	}
	if _, ok := fake.ContactSegmentSnapshot(created.ID); ok {
		t.Fatal("disappearing delete retained the fake identity")
	}
}

func contactSegmentFiltersForFakeTest(value string) []hubspot.ContactSegmentFilterGroup {
	return []hubspot.ContactSegmentFilterGroup{{Filters: []hubspot.ContactSegmentFilter{{
		Property: "ns_onboarding_status", Operator: "is_equal_to", PropertyKind: "text", Value: value,
	}}}}
}

type fakeContactSegmentRoundTrip func(*http.Request) (*http.Response, error)

func (fn fakeContactSegmentRoundTrip) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func fakeContactSegmentClients(t *testing.T, fake *FakeHubSpot) *hubspot.ClientSet {
	t.Helper()
	origin, err := url.Parse("https://fake-hubspot.invalid")
	if err != nil {
		t.Fatal(err)
	}
	httpClient := &http.Client{Transport: fakeContactSegmentRoundTrip(func(request *http.Request) (*http.Response, error) {
		recorder := httptest.NewRecorder()
		fake.ServeHTTP(recorder, request)
		return recorder.Result(), nil
	})}
	clients, err := hubspot.NewClientSet(hubspot.TransportConfig{BaseURL: origin, AccessToken: "token", HTTPClient: httpClient})
	if err != nil {
		t.Fatal(err)
	}
	return clients
}
