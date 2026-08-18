// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

//go:build acceptance

package acceptance_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackemcpherson/terraform-provider-hubspot/internal/acceptance"
	"github.com/jackemcpherson/terraform-provider-hubspot/internal/hubspot"
)

const contactSegmentPath = "/crm/lists/2026-03"

type contactSegmentProbe struct {
	transport *hubspot.Transport
	ownedIDs  *[]string
}

type contactSegmentValueOperation struct {
	operationType    string
	equalOperator    string
	notEqualOperator string
	usesSingleValue  bool
}

type contactSegmentEnvelope struct {
	List contactSegmentWire `json:"list"`
}

type updatedContactSegmentEnvelope struct {
	UpdatedList contactSegmentWire `json:"updatedList"`
}

type contactSegmentWire struct {
	ListID           string                      `json:"listId"`
	ListVersion      int64                       `json:"listVersion"`
	Name             string                      `json:"name"`
	ObjectTypeID     string                      `json:"objectTypeId"`
	ProcessingStatus string                      `json:"processingStatus"`
	ProcessingType   string                      `json:"processingType"`
	DeletedAt        *string                     `json:"deletedAt"`
	FilterBranch     *contactSegmentFilterBranch `json:"filterBranch"`
}

type contactSegmentCreate struct {
	Name           string                      `json:"name"`
	ObjectTypeID   string                      `json:"objectTypeId"`
	ProcessingType string                      `json:"processingType"`
	FilterBranch   *contactSegmentFilterBranch `json:"filterBranch,omitempty"`
}

type contactSegmentFilterUpdate struct {
	FilterBranch contactSegmentFilterBranch `json:"filterBranch"`
}

type contactSegmentFilterBranch struct {
	FilterBranchType     string                       `json:"filterBranchType"`
	FilterBranchOperator string                       `json:"filterBranchOperator"`
	FilterBranches       []contactSegmentFilterBranch `json:"filterBranches"`
	Filters              []contactSegmentFilter       `json:"filters"`
}

type contactSegmentFilter struct {
	FilterType string                        `json:"filterType"`
	Property   string                        `json:"property"`
	Operation  contactSegmentFilterOperation `json:"operation"`
}

type contactSegmentFilterOperation struct {
	OperationType                string          `json:"operationType"`
	Operator                     string          `json:"operator"`
	Value                        json.RawMessage `json:"value,omitempty"`
	Values                       []string        `json:"values,omitempty"`
	IncludeObjectsWithNoValueSet *bool           `json:"includeObjectsWithNoValueSet"`
}

func TestAcc_contact_segments_ContractPreflight(t *testing.T) {
	requireAcceptanceEnabled(t)
	token := requiredEnvironment(t, "HUBSPOT_ACCESS_TOKEN")
	prefix := requiredEnvironment(t, "HUBSPOT_ACCEPTANCE_PREFIX")
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
	defer cancel()

	createdIDs := make([]string, 0, 3)
	probe := newContactSegmentProbe(t, ctx, token, &createdIDs)
	suffix := strings.ReplaceAll(time.Now().UTC().Format("20060102T150405.000000000"), ".", "")
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cleanupCancel()
		for _, id := range createdIDs {
			if err := probe.ensureDeleted(cleanupCtx, id); err != nil {
				t.Errorf("Contact segment contract cleanup failed: %s", acceptance.SanitizedHubSpotError(err))
			}
		}
	}()
	valueOperation := probe.mustFindUniversalValueOperation(t, ctx, prefix, suffix)

	manual := probe.mustCreate(t, ctx, contactSegmentCreate{
		Name: prefix + "manual_" + suffix, ObjectTypeID: "0-1", ProcessingType: "MANUAL",
	})
	manual = probe.mustConverge(t, ctx, manual.ListID, "MANUAL", nil, false)

	dynamicFilters := contactSegmentOR(
		contactSegmentAND(
			contactSegmentValueFilter(valueOperation, "firstname", true, "contract-probe"),
			contactSegmentValueFilter(valueOperation, "lastname", false, "contract-probe"),
			contactSegmentPresenceFilter("email", "IS_KNOWN"),
			contactSegmentPresenceFilter("phone", "IS_UNKNOWN"),
		),
	)
	dynamic := probe.mustCreate(t, ctx, contactSegmentCreate{
		Name: prefix + "dynamic_" + suffix, ObjectTypeID: "0-1", ProcessingType: "DYNAMIC",
		FilterBranch: &dynamicFilters,
	})
	dynamic = probe.mustConverge(t, ctx, dynamic.ListID, "DYNAMIC", &dynamicFilters, false)

	snapshotFilters := contactSegmentOR(
		contactSegmentAND(contactSegmentValueFilter(valueOperation, "lifecyclestage", true, "lead")),
	)
	snapshot := probe.mustCreate(t, ctx, contactSegmentCreate{
		Name: prefix + "snapshot_" + suffix, ObjectTypeID: "0-1", ProcessingType: "SNAPSHOT",
		FilterBranch: &snapshotFilters,
	})
	snapshot = probe.mustConverge(t, ctx, snapshot.ListID, "SNAPSHOT", &snapshotFilters, false)
	t.Logf("Contact segment create transitions: MANUAL=%s/v%d DYNAMIC=%s/v%d SNAPSHOT=%s/v%d", manual.ProcessingStatus, manual.ListVersion, dynamic.ProcessingStatus, dynamic.ListVersion, snapshot.ProcessingStatus, snapshot.ListVersion)

	manual = probe.mustRename(t, ctx, manual, manual.Name+"_renamed")
	dynamic = probe.mustRename(t, ctx, dynamic, dynamic.Name+"_renamed")
	snapshot = probe.mustRename(t, ctx, snapshot, snapshot.Name+"_renamed")
	probe.mustRejectDuplicateName(t, ctx, manual.ListID, dynamic.Name)

	updatedDynamicFilters := contactSegmentOR(
		contactSegmentAND(
			contactSegmentValueFilter(valueOperation, "hs_lead_status", true, "NEW"),
			contactSegmentPresenceFilter("firstname", "IS_KNOWN"),
		),
	)
	probe.mustUpdateFilters(t, ctx, dynamic.ListID, updatedDynamicFilters)
	dynamic = probe.mustConverge(t, ctx, dynamic.ListID, "DYNAMIC", &updatedDynamicFilters, false)
	probe.mustRejectFilterUpdate(t, ctx, snapshot.ListID, updatedDynamicFilters, "SNAPSHOT")
	probe.mustRejectFilterUpdate(t, ctx, manual.ListID, updatedDynamicFilters, "MANUAL")

	probe.mustDelete(t, ctx, dynamic.ListID)
	tombstone := probe.mustConverge(t, ctx, dynamic.ListID, "DYNAMIC", &updatedDynamicFilters, true)
	if tombstone.Name != dynamic.Name {
		t.Fatal("Contact segment tombstone did not preserve the exact supported definition")
	}
	repeatedDelete := probe.mustClassifyRepeatedDelete(t, ctx, dynamic.ListID)
	probe.mustRestore(t, ctx, dynamic.ListID)
	restored := probe.mustConverge(t, ctx, dynamic.ListID, "DYNAMIC", &updatedDynamicFilters, false)
	if restored.ListID != dynamic.ListID || restored.Name != dynamic.Name {
		t.Fatal("Contact segment restore did not preserve the exact generated identity and name")
	}
	repeatedRestore := probe.mustClassifyRepeatedRestore(t, ctx, dynamic.ListID)
	probe.mustDelete(t, ctx, dynamic.ListID)
	probe.mustConverge(t, ctx, dynamic.ListID, "DYNAMIC", &updatedDynamicFilters, true)
	t.Logf("Contact segment delete/restore transitions: active=v%d tombstone=%s/v%d restored=%s/v%d repeated-delete=%s repeated-restore=%s", dynamic.ListVersion, tombstone.ProcessingStatus, tombstone.ListVersion, restored.ProcessingStatus, restored.ListVersion, repeatedDelete, repeatedRestore)

	probe.mustVerifyPermanentAbsence(t, ctx, "9223372036854775807")
	t.Log("Contact segment contract preflight proved MANUAL, DYNAMIC, and SNAPSHOT creation; text/select/presence round-trip; dynamic replacement; snapshot/manual immutability; and exact-ID delete/read/restore on the protected Free portal")
}

func TestContactSegmentProbeNormalizesSupportedValueShapes(t *testing.T) {
	includeUnset := false
	stringValue, err := json.Marshal("lead")
	if err != nil {
		t.Fatal("encode test value")
	}
	stringBranch := contactSegmentOR(contactSegmentAND(contactSegmentFilter{
		FilterType: "PROPERTY", Property: "lifecyclestage",
		Operation: contactSegmentFilterOperation{
			OperationType: "STRING", Operator: "IS_EQUAL_TO", Value: stringValue,
			IncludeObjectsWithNoValueSet: &includeUnset,
		},
	}))
	enumerationBranch := contactSegmentOR(contactSegmentAND(contactSegmentFilter{
		FilterType: "PROPERTY", Property: "lifecyclestage",
		Operation: contactSegmentFilterOperation{
			OperationType: "ENUMERATION", Operator: "IS_ANY_OF", Values: []string{"lead"},
			IncludeObjectsWithNoValueSet: &includeUnset,
		},
	}))
	if strings.Join(contactSegmentFilterFingerprint(stringBranch), "\n") != strings.Join(contactSegmentFilterFingerprint(enumerationBranch), "\n") {
		t.Fatal("supported string and enumeration equality shapes did not normalize to one public predicate")
	}
	if contactSegmentPublicOperator("IS_UNKNOWN") != "is_not_known" || contactSegmentPublicOperator("CONTAINS") != "" {
		t.Fatal("Contact segment operator normalization accepted an unsupported mapping")
	}
}

func newContactSegmentProbe(t *testing.T, ctx context.Context, token string, ownedIDs *[]string) contactSegmentProbe {
	t.Helper()
	if _, err := acceptance.NewRealPortalClientSet(ctx, token, "terraform-provider-hubspot/contact-segments-contract-preflight"); err != nil {
		t.Fatalf("Contact segment capability preflight failed: %s", acceptance.SanitizedHubSpotError(err))
	}
	origin, err := url.Parse(acceptance.RealPortalOrigin)
	if err != nil {
		t.Fatal("parse HubSpot API origin")
	}
	transport, err := hubspot.NewTransport(hubspot.TransportConfig{
		BaseURL: origin, AccessToken: token,
		UserAgent: "terraform-provider-hubspot/contact-segments-contract-preflight",
	})
	if err != nil {
		t.Fatal("configure Contact segment contract transport")
	}
	return contactSegmentProbe{transport: transport, ownedIDs: ownedIDs}
}

func (p contactSegmentProbe) mustCreate(t *testing.T, ctx context.Context, input contactSegmentCreate) contactSegmentWire {
	t.Helper()
	response, err := p.create(ctx, input)
	if err != nil || response.ListID == "" {
		t.Fatalf("Contact segment create probe failed: %s", acceptance.SanitizedHubSpotError(err))
	}
	if response.ObjectTypeID != input.ObjectTypeID || response.ProcessingType != input.ProcessingType || response.Name != input.Name {
		t.Fatal("Contact segment create probe returned a different supported definition")
	}
	return response
}

func (p contactSegmentProbe) create(ctx context.Context, input contactSegmentCreate) (contactSegmentWire, error) {
	body, err := json.Marshal(input)
	if err != nil {
		return contactSegmentWire{}, fmt.Errorf("encode Contact segment contract create: %w", err)
	}
	var response contactSegmentEnvelope
	err = p.transport.Do(ctx, hubspot.Operation{
		Name: "contact-segment-contract-create", Method: http.MethodPost,
		Path: contactSegmentPath, Replay: hubspot.ReplayNever,
	}, bytes.NewReader(body), &response)
	if response.List.ListID != "" {
		*p.ownedIDs = append(*p.ownedIDs, response.List.ListID)
		if idErr := validateContactSegmentProbeID(response.List.ListID); idErr != nil {
			return response.List, idErr
		}
	}
	if err != nil {
		return response.List, err
	}
	if response.List.ListID == "" {
		return response.List, errors.New("HubSpot Contact segment create response omitted listId")
	}
	return response.List, nil
}

func (p contactSegmentProbe) mustFindUniversalValueOperation(t *testing.T, ctx context.Context, prefix, suffix string) contactSegmentValueOperation {
	t.Helper()
	candidates := []contactSegmentValueOperation{
		{operationType: "MULTISTRING", equalOperator: "IS_EQUAL_TO", notEqualOperator: "IS_NOT_EQUAL_TO"},
		{operationType: "STRING", equalOperator: "IS_EQUAL_TO", notEqualOperator: "IS_NOT_EQUAL_TO", usesSingleValue: true},
		{operationType: "ENUMERATION", equalOperator: "IS_ANY_OF", notEqualOperator: "IS_NONE_OF"},
	}
	for _, candidate := range candidates {
		textAccepted := p.tryValueOperation(t, ctx, prefix+"matrix_"+strings.ToLower(candidate.operationType)+"_text_"+suffix, "firstname", "contract-probe", candidate)
		selectAccepted := p.tryValueOperation(t, ctx, prefix+"matrix_"+strings.ToLower(candidate.operationType)+"_select_"+suffix, "lifecyclestage", "lead", candidate)
		t.Logf("Contact segment operation matrix: %s text=%t select=%t", candidate.operationType, textAccepted, selectAccepted)
		if textAccepted && selectAccepted {
			return candidate
		}
	}
	t.Fatal("Contact segment text and select equality require different wire operation types; the three-field filter and lists-only scope contracts are incompatible")
	return contactSegmentValueOperation{}
}

func (p contactSegmentProbe) tryValueOperation(t *testing.T, ctx context.Context, name, property, value string, candidate contactSegmentValueOperation) bool {
	t.Helper()
	filters := contactSegmentOR(contactSegmentAND(contactSegmentValueFilter(candidate, property, true, value)))
	created, err := p.create(ctx, contactSegmentCreate{
		Name: name, ObjectTypeID: "0-1", ProcessingType: "DYNAMIC", FilterBranch: &filters,
	})
	if err != nil {
		if contactSegmentProbeRejected(err) && created.ListID == "" {
			return false
		}
		t.Fatalf("Contact segment operation-matrix create could not be classified: %s", acceptance.SanitizedHubSpotError(err))
	}
	p.mustConverge(t, ctx, created.ListID, "DYNAMIC", &filters, false)
	if err := p.ensureDeleted(ctx, created.ListID); err != nil {
		t.Fatalf("Contact segment operation-matrix cleanup failed: %s", acceptance.SanitizedHubSpotError(err))
	}
	return true
}

func (p contactSegmentProbe) mustRead(t *testing.T, ctx context.Context, id string) contactSegmentWire {
	t.Helper()
	segment, err := p.read(ctx, id)
	if err != nil {
		t.Fatalf("Contact segment exact-ID read probe failed: %s", acceptance.SanitizedHubSpotError(err))
	}
	return segment
}

func (p contactSegmentProbe) read(ctx context.Context, id string) (contactSegmentWire, error) {
	var response contactSegmentEnvelope
	err := p.transport.Do(ctx, hubspot.Operation{
		Name: "contact-segment-contract-read", Method: http.MethodGet,
		Path: contactSegmentPath + "/" + url.PathEscape(id) + "?includeFilters=true", Replay: hubspot.ReplaySafe,
	}, nil, &response)
	if err != nil {
		return contactSegmentWire{}, err
	}
	if response.List.ListID == "" {
		return contactSegmentWire{}, errors.New("HubSpot Contact segment response omitted listId")
	}
	if response.List.ListID != id {
		return contactSegmentWire{}, errors.New("HubSpot Contact segment response returned a different listId")
	}
	return response.List, nil
}

func (p contactSegmentProbe) mustConverge(t *testing.T, ctx context.Context, id, processingType string, filters *contactSegmentFilterBranch, deleted bool) contactSegmentWire {
	t.Helper()
	deadline := time.Now().Add(5 * time.Minute)
	for {
		segment, err := p.read(ctx, id)
		if err != nil {
			if contactSegmentProbeNotFound(err) && time.Now().Before(deadline) {
				time.Sleep(2 * time.Second)
				continue
			}
			t.Fatalf("Contact segment exact-ID convergence probe failed: %s", acceptance.SanitizedHubSpotError(err))
		}
		if segment.ObjectTypeID != "0-1" || segment.ProcessingType != processingType || segment.ListVersion <= 0 || segment.ProcessingStatus == "" {
			t.Fatal("Contact segment read-back omitted required definition fields")
		}
		if deleted != (segment.DeletedAt != nil && strings.TrimSpace(*segment.DeletedAt) != "") {
			if time.Now().Before(deadline) {
				time.Sleep(2 * time.Second)
				continue
			}
			t.Fatal("Contact segment deletion state did not converge by exact-ID read")
		}
		if filters == nil {
			if segment.FilterBranch != nil {
				t.Fatal("Manual Contact segment unexpectedly returned a filter definition")
			}
		} else {
			assertContactSegmentFilters(t, *filters, segment.FilterBranch)
		}
		status := strings.ToUpper(segment.ProcessingStatus)
		switch status {
		case "COMPLETE":
			return segment
		case "FAILED", "ERROR", "CANCELED", "CANCELLED", "REJECTED":
			t.Fatal("Contact segment processing reached a terminal failure state")
		default:
			if time.Now().After(deadline) {
				t.Fatal("Contact segment processing did not complete within five minutes")
			}
			time.Sleep(2 * time.Second)
		}
	}
}

func (p contactSegmentProbe) mustRename(t *testing.T, ctx context.Context, current contactSegmentWire, name string) contactSegmentWire {
	t.Helper()
	query := url.Values{"includeFilters": []string{"true"}, "listName": []string{name}}
	var response updatedContactSegmentEnvelope
	if err := p.transport.Do(ctx, hubspot.Operation{
		Name: "contact-segment-contract-rename", Method: http.MethodPut,
		Path:   contactSegmentPath + "/" + url.PathEscape(current.ListID) + "/update-list-name?" + query.Encode(),
		Replay: hubspot.ReplayNever,
	}, nil, &response); err != nil {
		t.Fatalf("Contact segment name update probe failed: %s", acceptance.SanitizedHubSpotError(err))
	}
	updated := p.mustRead(t, ctx, current.ListID)
	if updated.Name != name || updated.ListID != current.ListID || updated.ProcessingType != current.ProcessingType {
		t.Fatal("Contact segment name update did not preserve identity and processing type")
	}
	return updated
}

func (p contactSegmentProbe) mustRejectDuplicateName(t *testing.T, ctx context.Context, id, name string) {
	t.Helper()
	query := url.Values{"listName": []string{name}}
	err := p.transport.Do(ctx, hubspot.Operation{
		Name: "contact-segment-contract-duplicate-name", Method: http.MethodPut,
		Path:   contactSegmentPath + "/" + url.PathEscape(id) + "/update-list-name?" + query.Encode(),
		Replay: hubspot.ReplayNever,
	}, nil, nil)
	if !contactSegmentProbeRejected(err) {
		t.Fatal("Contact segment duplicate-name probe was not authoritatively rejected")
	}
}

func (p contactSegmentProbe) mustUpdateFilters(t *testing.T, ctx context.Context, id string, filters contactSegmentFilterBranch) {
	t.Helper()
	body, err := json.Marshal(contactSegmentFilterUpdate{FilterBranch: filters})
	if err != nil {
		t.Fatal("encode Contact segment filter update")
	}
	var response updatedContactSegmentEnvelope
	if err := p.transport.Do(ctx, hubspot.Operation{
		Name: "contact-segment-contract-filter-update", Method: http.MethodPut,
		Path: contactSegmentPath + "/" + url.PathEscape(id) + "/update-list-filters", Replay: hubspot.ReplayNever,
	}, bytes.NewReader(body), &response); err != nil {
		t.Fatalf("Dynamic Contact segment filter update probe failed: %s", acceptance.SanitizedHubSpotError(err))
	}
}

func (p contactSegmentProbe) mustRejectFilterUpdate(t *testing.T, ctx context.Context, id string, filters contactSegmentFilterBranch, processingType string) {
	t.Helper()
	body, err := json.Marshal(contactSegmentFilterUpdate{FilterBranch: filters})
	if err != nil {
		t.Fatal("encode rejected Contact segment filter update")
	}
	err = p.transport.Do(ctx, hubspot.Operation{
		Name: "contact-segment-contract-filter-update-rejected", Method: http.MethodPut,
		Path: contactSegmentPath + "/" + url.PathEscape(id) + "/update-list-filters", Replay: hubspot.ReplayNever,
	}, bytes.NewReader(body), nil)
	if !contactSegmentProbeRejected(err) {
		t.Fatalf("%s Contact segment filter update was not authoritatively rejected", processingType)
	}
}

func (p contactSegmentProbe) mustDelete(t *testing.T, ctx context.Context, id string) {
	t.Helper()
	if err := p.transport.Do(ctx, hubspot.Operation{
		Name: "contact-segment-contract-delete", Method: http.MethodDelete,
		Path: contactSegmentPath + "/" + url.PathEscape(id), Replay: hubspot.ReplayNever,
	}, nil, nil); err != nil {
		t.Fatalf("Contact segment delete probe failed: %s", acceptance.SanitizedHubSpotError(err))
	}
}

func (p contactSegmentProbe) mustClassifyRepeatedDelete(t *testing.T, ctx context.Context, id string) string {
	t.Helper()
	err := p.transport.Do(ctx, hubspot.Operation{
		Name: "contact-segment-contract-delete-repeated", Method: http.MethodDelete,
		Path: contactSegmentPath + "/" + url.PathEscape(id), Replay: hubspot.ReplayNever,
	}, nil, nil)
	if err == nil {
		return "complete"
	}
	if !contactSegmentProbeRejected(err) {
		t.Fatal("Repeated Contact segment delete did not return a classified outcome")
	}
	return "rejected"
}

func (p contactSegmentProbe) mustRestore(t *testing.T, ctx context.Context, id string) {
	t.Helper()
	if err := p.transport.Do(ctx, hubspot.Operation{
		Name: "contact-segment-contract-restore", Method: http.MethodPut,
		Path: contactSegmentPath + "/" + url.PathEscape(id) + "/restore", Replay: hubspot.ReplayNever,
	}, nil, nil); err != nil {
		t.Fatalf("Contact segment restore probe failed: %s", acceptance.SanitizedHubSpotError(err))
	}
}

func (p contactSegmentProbe) mustClassifyRepeatedRestore(t *testing.T, ctx context.Context, id string) string {
	t.Helper()
	err := p.transport.Do(ctx, hubspot.Operation{
		Name: "contact-segment-contract-restore-repeated", Method: http.MethodPut,
		Path: contactSegmentPath + "/" + url.PathEscape(id) + "/restore", Replay: hubspot.ReplayNever,
	}, nil, nil)
	if err == nil {
		return "complete"
	}
	if !contactSegmentProbeRejected(err) {
		t.Fatal("Repeated Contact segment restore did not return a classified outcome")
	}
	return "rejected"
}

func (p contactSegmentProbe) mustVerifyPermanentAbsence(t *testing.T, ctx context.Context, id string) {
	t.Helper()
	_, readErr := p.read(ctx, id)
	deleteErr := p.transport.Do(ctx, hubspot.Operation{
		Name: "contact-segment-contract-delete-absent", Method: http.MethodDelete,
		Path: contactSegmentPath + "/" + url.PathEscape(id), Replay: hubspot.ReplayNever,
	}, nil, nil)
	restoreErr := p.transport.Do(ctx, hubspot.Operation{
		Name: "contact-segment-contract-restore-absent", Method: http.MethodPut,
		Path: contactSegmentPath + "/" + url.PathEscape(id) + "/restore", Replay: hubspot.ReplayNever,
	}, nil, nil)
	deleteClassified := deleteErr == nil || contactSegmentProbeRejected(deleteErr)
	if !contactSegmentProbeNotFound(readErr) || !deleteClassified || !contactSegmentProbeRejected(restoreErr) {
		t.Fatal("Permanently absent Contact segment did not return the expected exact-ID classifications")
	}
}

func (p contactSegmentProbe) ensureDeleted(ctx context.Context, id string) error {
	current, err := p.read(ctx, id)
	if err != nil {
		if contactSegmentProbeNotFound(err) {
			return nil
		}
		return err
	}
	if current.DeletedAt != nil && strings.TrimSpace(*current.DeletedAt) != "" {
		return nil
	}
	err = p.transport.Do(ctx, hubspot.Operation{
		Name: "contact-segment-contract-cleanup", Method: http.MethodDelete,
		Path: contactSegmentPath + "/" + url.PathEscape(id), Replay: hubspot.ReplayNever,
	}, nil, nil)
	if err != nil && !contactSegmentProbeRejected(err) {
		return err
	}
	deadline := time.Now().Add(time.Minute)
	for {
		deleted, readErr := p.read(ctx, id)
		if readErr == nil && deleted.DeletedAt != nil && strings.TrimSpace(*deleted.DeletedAt) != "" {
			return nil
		}
		if readErr != nil && !contactSegmentProbeNotFound(readErr) {
			return readErr
		}
		if time.Now().After(deadline) {
			return errors.New("Contact segment cleanup did not verify the exact tombstone")
		}
		time.Sleep(2 * time.Second)
	}
}

func contactSegmentOR(groups ...contactSegmentFilterBranch) contactSegmentFilterBranch {
	return contactSegmentFilterBranch{
		FilterBranchType: "OR", FilterBranchOperator: "OR",
		FilterBranches: groups, Filters: []contactSegmentFilter{},
	}
}

func contactSegmentAND(filters ...contactSegmentFilter) contactSegmentFilterBranch {
	return contactSegmentFilterBranch{
		FilterBranchType: "AND", FilterBranchOperator: "AND",
		FilterBranches: []contactSegmentFilterBranch{}, Filters: filters,
	}
}

func contactSegmentValueFilter(candidate contactSegmentValueOperation, property string, equal bool, value string) contactSegmentFilter {
	includeUnset := false
	operator := candidate.notEqualOperator
	if equal {
		operator = candidate.equalOperator
	}
	operation := contactSegmentFilterOperation{
		OperationType: candidate.operationType, Operator: operator,
		IncludeObjectsWithNoValueSet: &includeUnset,
	}
	if candidate.usesSingleValue {
		encoded, err := json.Marshal(value)
		if err != nil {
			panic("encode static Contact segment filter value")
		}
		operation.Value = encoded
	} else {
		operation.Values = []string{value}
	}
	return contactSegmentFilter{
		FilterType: "PROPERTY", Property: property,
		Operation: operation,
	}
}

func contactSegmentPresenceFilter(property, operator string) contactSegmentFilter {
	includeUnset := false
	return contactSegmentFilter{
		FilterType: "PROPERTY", Property: property,
		Operation: contactSegmentFilterOperation{
			OperationType: "ALL_PROPERTY", Operator: operator,
			IncludeObjectsWithNoValueSet: &includeUnset,
		},
	}
}

func assertContactSegmentFilters(t *testing.T, expected contactSegmentFilterBranch, actual *contactSegmentFilterBranch) {
	t.Helper()
	if actual == nil {
		t.Fatal("Contact segment exact-ID read omitted requested filters")
	}
	if actual.FilterBranchType != "OR" || actual.FilterBranchOperator != "OR" || len(actual.Filters) != 0 || len(actual.FilterBranches) == 0 {
		t.Fatal("Contact segment read-back did not preserve the root OR branch contract")
	}
	for _, group := range actual.FilterBranches {
		if group.FilterBranchType != "AND" || group.FilterBranchOperator != "AND" || len(group.FilterBranches) != 0 || len(group.Filters) == 0 {
			t.Fatal("Contact segment read-back did not preserve nested nonempty AND groups")
		}
		for _, filter := range group.Filters {
			if filter.FilterType != "PROPERTY" || filter.Property == "" || filter.Operation.OperationType == "" || filter.Operation.Operator == "" || filter.Operation.IncludeObjectsWithNoValueSet == nil {
				t.Fatal("Contact segment read-back omitted required property-operation fields")
			}
			if *filter.Operation.IncludeObjectsWithNoValueSet {
				t.Fatal("Contact segment equality or presence filter unexpectedly included unset properties")
			}
			if contactSegmentPublicOperator(filter.Operation.Operator) == "" {
				t.Fatal("Contact segment read-back returned an unsupported property operator")
			}
			switch filter.Operation.OperationType {
			case "STRING":
				var value string
				if len(filter.Operation.Values) != 0 || json.Unmarshal(filter.Operation.Value, &value) != nil || strings.TrimSpace(value) == "" {
					t.Fatal("Contact segment STRING predicate did not round-trip with one value")
				}
			case "MULTISTRING", "ENUMERATION":
				if len(filter.Operation.Values) != 1 || len(filter.Operation.Value) != 0 {
					t.Fatal("Contact segment multi-value predicate did not round-trip with exactly one value")
				}
			case "ALL_PROPERTY":
				if filter.Operation.Operator != "IS_KNOWN" && filter.Operation.Operator != "IS_UNKNOWN" {
					t.Fatal("Contact segment presence predicate returned an unsupported operator")
				}
				if filter.Operation.OperationType != "ALL_PROPERTY" || len(filter.Operation.Values) != 0 || len(filter.Operation.Value) != 0 {
					t.Fatal("Contact segment presence predicate did not round-trip as ALL_PROPERTY without a value")
				}
			default:
				t.Fatal("Contact segment read-back returned an unsupported property operation type")
			}
		}
	}
	expectedFingerprint := contactSegmentFilterFingerprint(expected)
	actualFingerprint := contactSegmentFilterFingerprint(*actual)
	if strings.Join(expectedFingerprint, "\n") != strings.Join(actualFingerprint, "\n") {
		t.Fatal("Contact segment exact-ID read changed the requested filter semantics")
	}
}

func contactSegmentFilterFingerprint(branch contactSegmentFilterBranch) []string {
	result := make([]string, 0)
	for groupIndex, group := range branch.FilterBranches {
		filters := make([]string, 0, len(group.Filters))
		for _, filter := range group.Filters {
			value := ""
			if len(filter.Operation.Values) == 1 {
				value = filter.Operation.Values[0]
			} else if len(filter.Operation.Value) != 0 {
				_ = json.Unmarshal(filter.Operation.Value, &value)
			}
			filters = append(filters, fmt.Sprintf("%s|%s|%s", filter.Property, contactSegmentPublicOperator(filter.Operation.Operator), value))
		}
		sort.Strings(filters)
		result = append(result, fmt.Sprintf("%d:%s", groupIndex, strings.Join(filters, ",")))
	}
	sort.Strings(result)
	return result
}

func contactSegmentPublicOperator(operator string) string {
	switch operator {
	case "IS_EQUAL_TO", "IS_ANY_OF":
		return "is_equal_to"
	case "IS_NOT_EQUAL_TO", "IS_NONE_OF":
		return "is_not_equal_to"
	case "IS_KNOWN":
		return "is_known"
	case "IS_UNKNOWN":
		return "is_not_known"
	default:
		return ""
	}
}

func validateContactSegmentProbeID(id string) error {
	value, ok := new(big.Int).SetString(id, 10)
	if !ok || value.Sign() <= 0 || value.String() != id {
		return errors.New("HubSpot Contact segment listId was not a canonical positive decimal string")
	}
	return nil
}

func contactSegmentProbeRejected(err error) bool {
	var apiError *hubspot.Error
	return errors.As(err, &apiError) && apiError.Status >= 400 && apiError.Status < 500
}

func contactSegmentProbeNotFound(err error) bool {
	var apiError *hubspot.Error
	return errors.As(err, &apiError) && apiError.Status == http.StatusNotFound
}
