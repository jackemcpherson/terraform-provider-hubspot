// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

//go:build acceptance

package acceptance

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackemcpherson/terraform-provider-hubspot/internal/hubspot"
)

type fakeContactSegment struct {
	segment       hubspot.ContactSegment
	createWrites  int
	readWrites    int
	nameWrites    int
	filterWrites  int
	deleteWrites  int
	restoreWrites int
	pendingReads  int
	staleReads    int
	staleSegment  hubspot.ContactSegment
}

// ContactSegmentFault selects one deterministic one-shot Lists API outcome.
type ContactSegmentFault string

const (
	ContactSegmentFaultCreateKnown      ContactSegmentFault = "create_known"
	ContactSegmentFaultCreateUnknown    ContactSegmentFault = "create_unknown"
	ContactSegmentFaultUpdateApplied    ContactSegmentFault = "update_applied"
	ContactSegmentFaultUpdateRejected   ContactSegmentFault = "update_rejected"
	ContactSegmentFaultUpdatePending    ContactSegmentFault = "update_pending"
	ContactSegmentFaultUpdateStale      ContactSegmentFault = "update_stale"
	ContactSegmentFaultUpdateTimeout    ContactSegmentFault = "update_timeout"
	ContactSegmentFaultProcessingFailed ContactSegmentFault = "processing_failed"
	ContactSegmentFaultDeleteApplied    ContactSegmentFault = "delete_applied"
	ContactSegmentFaultDeleteRejected   ContactSegmentFault = "delete_rejected"
	ContactSegmentFaultRestoreApplied   ContactSegmentFault = "restore_applied"
)

type fakeContactSegmentBranch struct {
	FilterBranchType     string                     `json:"filterBranchType"`
	FilterBranchOperator string                     `json:"filterBranchOperator"`
	FilterBranches       []fakeContactSegmentBranch `json:"filterBranches"`
	Filters              []fakeContactSegmentFilter `json:"filters"`
}

type fakeContactSegmentFilter struct {
	FilterType string                      `json:"filterType"`
	Property   string                      `json:"property"`
	Operation  fakeContactSegmentOperation `json:"operation"`
}

type fakeContactSegmentOperation struct {
	OperationType                string   `json:"operationType"`
	Operator                     string   `json:"operator"`
	Value                        *string  `json:"value,omitempty"`
	Values                       []string `json:"values,omitempty"`
	IncludeObjectsWithNoValueSet *bool    `json:"includeObjectsWithNoValueSet"`
}

func (f *FakeHubSpot) handleContactSegments(response http.ResponseWriter, request *http.Request, rest []string) {
	switch len(rest) {
	case 0:
		f.handleContactSegmentCollection(response, request)
	case 1:
		f.handleContactSegmentItem(response, request, rest[0])
	case 2:
		f.handleContactSegmentAction(response, request, rest[0], rest[1])
	default:
		writeFakeError(response, http.StatusNotFound, "OBJECT_NOT_FOUND", "", "No route matched this request.")
	}
}

func (f *FakeHubSpot) handleContactSegmentCollection(response http.ResponseWriter, request *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if request.Method != http.MethodPost {
		writeFakeError(response, http.StatusMethodNotAllowed, "VALIDATION_ERROR", "", "Unsupported method.")
		return
	}
	var body struct {
		Name           string                    `json:"name"`
		ObjectTypeID   string                    `json:"objectTypeId"`
		ProcessingType string                    `json:"processingType"`
		FilterBranch   *fakeContactSegmentBranch `json:"filterBranch"`
	}
	if !decodeFakeBody(response, request, &body) {
		return
	}
	processingType := strings.ToLower(body.ProcessingType)
	groups, ok := fakeContactSegmentGroupsFromBranch(body.FilterBranch, processingType)
	if strings.TrimSpace(body.Name) == "" || body.ObjectTypeID != "0-1" || !ok ||
		(processingType != "manual" && processingType != "dynamic" && processingType != "snapshot") {
		writeFakeError(response, http.StatusBadRequest, "VALIDATION_ERROR", "", "The Contact segment did not match the supported definition.")
		return
	}
	for _, entry := range f.contactSegments {
		if !entry.segment.Deleted && entry.segment.Name == body.Name {
			writeFakeError(response, http.StatusConflict, "CONFLICT", "", "An active Contact segment already uses this name.")
			return
		}
	}
	f.nextContactSegmentID++
	id := strconv.Itoa(700000 + f.nextContactSegmentID)
	segment := hubspot.ContactSegment{
		ID: id, Version: 1, Name: body.Name, ProcessingStatus: "COMPLETE",
		ProcessingType: processingType, FilterGroups: groups,
	}
	if f.nextContactFault == ContactSegmentFaultProcessingFailed {
		segment.ProcessingStatus = "FAILED"
		f.nextContactFault = ""
	}
	f.contactSegments[id] = &fakeContactSegment{segment: segment, createWrites: 1}
	switch f.nextContactFault {
	case ContactSegmentFaultCreateKnown:
		f.nextContactFault = ""
		writeFakeJSON(response, http.StatusCreated, map[string]any{"list": map[string]any{
			"listId": id, "listVersion": 1, "name": body.Name, "objectTypeId": "unsupported",
			"processingStatus": "COMPLETE", "processingType": body.ProcessingType,
		}})
	case ContactSegmentFaultCreateUnknown:
		f.nextContactFault = ""
		writeFakeJSON(response, http.StatusCreated, map[string]any{"list": map[string]any{"futureServiceMetadata": true}})
	default:
		writeFakeJSON(response, http.StatusCreated, map[string]any{"list": fakeContactSegmentDocument(segment)})
	}
}

func (f *FakeHubSpot) handleContactSegmentItem(response http.ResponseWriter, request *http.Request, id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	entry := f.contactSegments[id]
	switch request.Method {
	case http.MethodGet:
		if entry == nil {
			writeFakeError(response, http.StatusNotFound, "OBJECT_NOT_FOUND", "", "No Contact segment matched this identity.")
			return
		}
		entry.readWrites++
		observed := entry.segment
		if entry.staleReads > 0 {
			observed = entry.staleSegment
			entry.staleReads--
		} else if entry.pendingReads > 0 {
			entry.pendingReads--
			if entry.pendingReads == 0 {
				entry.segment.ProcessingStatus = "COMPLETE"
			}
		}
		writeFakeJSON(response, http.StatusOK, map[string]any{"list": fakeContactSegmentDocument(observed)})
	case http.MethodDelete:
		if entry == nil || entry.segment.Deleted {
			writeFakeError(response, http.StatusNotFound, "OBJECT_NOT_FOUND", "", "No active Contact segment matched this identity.")
			return
		}
		if f.nextContactFault == ContactSegmentFaultDeleteRejected {
			f.nextContactFault = ""
			writeFakeError(response, http.StatusForbidden, "MISSING_SCOPES", "", "The Contact segment delete was rejected.")
			return
		}
		entry.segment.Deleted = true
		entry.segment.Version++
		entry.deleteWrites++
		if f.nextContactFault == ContactSegmentFaultDeleteApplied {
			f.nextContactFault = ""
			writeFakeError(response, http.StatusServiceUnavailable, "TEMPORARY_UNAVAILABLE", "", "The Contact segment delete outcome is ambiguous.")
			return
		}
		response.WriteHeader(http.StatusNoContent)
	default:
		writeFakeError(response, http.StatusMethodNotAllowed, "VALIDATION_ERROR", "", "Unsupported method.")
	}
}

func (f *FakeHubSpot) handleContactSegmentAction(response http.ResponseWriter, request *http.Request, id, action string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	entry := f.contactSegments[id]
	if request.Method != http.MethodPut || entry == nil {
		writeFakeError(response, http.StatusNotFound, "OBJECT_NOT_FOUND", "", "No Contact segment matched this identity.")
		return
	}
	switch action {
	case "update-list-name":
		if entry.segment.Deleted || strings.TrimSpace(request.URL.Query().Get("listName")) == "" {
			writeFakeError(response, http.StatusBadRequest, "VALIDATION_ERROR", "", "The Contact segment name update was invalid.")
			return
		}
		name := request.URL.Query().Get("listName")
		for otherID, other := range f.contactSegments {
			if otherID != id && !other.segment.Deleted && other.segment.Name == name {
				writeFakeError(response, http.StatusConflict, "CONFLICT", "", "An active Contact segment already uses this name.")
				return
			}
		}
		entry.segment.Name = name
		entry.nameWrites++
		writeFakeJSON(response, http.StatusOK, map[string]any{"updatedList": fakeContactSegmentDocument(entry.segment)})
	case "update-list-filters":
		if entry.segment.Deleted || entry.segment.ProcessingType != "dynamic" {
			writeFakeError(response, http.StatusBadRequest, "VALIDATION_ERROR", "", "Only active dynamic Contact segment filters can be updated.")
			return
		}
		var body struct {
			FilterBranch *fakeContactSegmentBranch `json:"filterBranch"`
		}
		if !decodeFakeBody(response, request, &body) {
			return
		}
		groups, ok := fakeContactSegmentGroupsFromBranch(body.FilterBranch, "dynamic")
		if !ok {
			writeFakeError(response, http.StatusBadRequest, "VALIDATION_ERROR", "", "The Contact segment filter update was invalid.")
			return
		}
		if f.nextContactFault == ContactSegmentFaultUpdateRejected {
			f.nextContactFault = ""
			writeFakeError(response, http.StatusBadRequest, "VALIDATION_ERROR", "", "The Contact segment filter update was rejected.")
			return
		}
		before := entry.segment
		entry.segment.FilterGroups = groups
		entry.segment.Version++
		entry.filterWrites++
		switch f.nextContactFault {
		case ContactSegmentFaultProcessingFailed:
			entry.segment.ProcessingStatus = "FAILED"
			f.nextContactFault = ""
		case ContactSegmentFaultUpdatePending:
			entry.segment.ProcessingStatus = "PROCESSING"
			entry.pendingReads = 2
			f.nextContactFault = ""
		case ContactSegmentFaultUpdateStale:
			entry.segment.ProcessingStatus = "COMPLETE"
			entry.staleSegment = before
			entry.staleReads = 2
			f.nextContactFault = ""
		case ContactSegmentFaultUpdateTimeout:
			entry.segment.ProcessingStatus = "PROCESSING"
			f.nextContactFault = ""
		default:
			entry.segment.ProcessingStatus = "COMPLETE"
		}
		if f.nextContactFault == ContactSegmentFaultUpdateApplied {
			f.nextContactFault = ""
			writeFakeError(response, http.StatusServiceUnavailable, "TEMPORARY_UNAVAILABLE", "", "The Contact segment update outcome is ambiguous.")
			return
		}
		writeFakeJSON(response, http.StatusOK, map[string]any{"updatedList": fakeContactSegmentDocument(entry.segment)})
	case "restore":
		if !entry.segment.Deleted {
			writeFakeError(response, http.StatusConflict, "CONFLICT", "", "The Contact segment is already active.")
			return
		}
		for otherID, other := range f.contactSegments {
			if otherID != id && !other.segment.Deleted && other.segment.Name == entry.segment.Name {
				writeFakeError(response, http.StatusConflict, "CONFLICT", "", "An active Contact segment already uses this name.")
				return
			}
		}
		entry.segment.Deleted = false
		entry.segment.Version++
		entry.segment.ProcessingStatus = "COMPLETE"
		entry.restoreWrites++
		if f.nextContactFault == ContactSegmentFaultRestoreApplied {
			f.nextContactFault = ""
			writeFakeError(response, http.StatusServiceUnavailable, "TEMPORARY_UNAVAILABLE", "", "The Contact segment restore outcome is ambiguous.")
			return
		}
		response.WriteHeader(http.StatusNoContent)
	default:
		writeFakeError(response, http.StatusNotFound, "OBJECT_NOT_FOUND", "", "No route matched this request.")
	}
}

func fakeContactSegmentGroupsFromBranch(branch *fakeContactSegmentBranch, processingType string) ([]hubspot.ContactSegmentFilterGroup, bool) {
	if processingType == "manual" {
		return nil, branch == nil
	}
	if branch == nil || branch.FilterBranchType != "OR" || branch.FilterBranchOperator != "OR" || len(branch.Filters) != 0 || len(branch.FilterBranches) == 0 {
		return nil, false
	}
	groups := make([]hubspot.ContactSegmentFilterGroup, 0, len(branch.FilterBranches))
	for _, and := range branch.FilterBranches {
		if and.FilterBranchType != "AND" || and.FilterBranchOperator != "AND" || len(and.FilterBranches) != 0 || len(and.Filters) == 0 {
			return nil, false
		}
		group := hubspot.ContactSegmentFilterGroup{Filters: make([]hubspot.ContactSegmentFilter, 0, len(and.Filters))}
		for _, filter := range and.Filters {
			semantic, ok := fakeContactSegmentFilterFromWire(filter)
			if !ok {
				return nil, false
			}
			group.Filters = append(group.Filters, semantic)
		}
		groups = append(groups, group)
	}
	return groups, true
}

func fakeContactSegmentFilterFromWire(filter fakeContactSegmentFilter) (hubspot.ContactSegmentFilter, bool) {
	if filter.FilterType != "PROPERTY" || strings.TrimSpace(filter.Property) == "" ||
		filter.Operation.IncludeObjectsWithNoValueSet == nil || *filter.Operation.IncludeObjectsWithNoValueSet {
		return hubspot.ContactSegmentFilter{}, false
	}
	semantic := hubspot.ContactSegmentFilter{Property: filter.Property}
	switch filter.Operation.OperationType {
	case "STRING":
		if filter.Operation.Value == nil || strings.TrimSpace(*filter.Operation.Value) == "" || len(filter.Operation.Values) != 0 {
			return semantic, false
		}
		semantic.PropertyKind, semantic.Value = "text", *filter.Operation.Value
		semantic.Operator = map[string]string{"IS_EQUAL_TO": "is_equal_to", "IS_NOT_EQUAL_TO": "is_not_equal_to"}[filter.Operation.Operator]
	case "ENUMERATION":
		if filter.Operation.Value != nil || len(filter.Operation.Values) != 1 || strings.TrimSpace(filter.Operation.Values[0]) == "" {
			return semantic, false
		}
		semantic.PropertyKind, semantic.Value = "select", filter.Operation.Values[0]
		semantic.Operator = map[string]string{"IS_ANY_OF": "is_equal_to", "IS_NONE_OF": "is_not_equal_to"}[filter.Operation.Operator]
	case "ALL_PROPERTY":
		if filter.Operation.Value != nil || len(filter.Operation.Values) != 0 {
			return semantic, false
		}
		semantic.Operator = map[string]string{"IS_KNOWN": "is_known", "IS_UNKNOWN": "is_not_known"}[filter.Operation.Operator]
	default:
		return semantic, false
	}
	return semantic, semantic.Operator != ""
}

func fakeContactSegmentDocument(segment hubspot.ContactSegment) map[string]any {
	document := map[string]any{
		"listId": segment.ID, "listVersion": segment.Version, "name": segment.Name,
		"objectTypeId": "0-1", "processingStatus": segment.ProcessingStatus,
		"processingType":        strings.ToUpper(segment.ProcessingType),
		"futureServiceMetadata": map[string]any{"revision": segment.Version},
	}
	if segment.Deleted {
		document["deletedAt"] = time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC).Format(time.RFC3339)
	}
	if segment.ProcessingType != "manual" {
		document["filterBranch"] = fakeContactSegmentBranchFromGroups(segment.FilterGroups)
	}
	return document
}

func fakeContactSegmentBranchFromGroups(groups []hubspot.ContactSegmentFilterGroup) fakeContactSegmentBranch {
	root := fakeContactSegmentBranch{FilterBranchType: "OR", FilterBranchOperator: "OR", FilterBranches: []fakeContactSegmentBranch{}, Filters: []fakeContactSegmentFilter{}}
	for _, group := range groups {
		and := fakeContactSegmentBranch{FilterBranchType: "AND", FilterBranchOperator: "AND", FilterBranches: []fakeContactSegmentBranch{}, Filters: []fakeContactSegmentFilter{}}
		for _, filter := range group.Filters {
			includeUnset := false
			operation := fakeContactSegmentOperation{IncludeObjectsWithNoValueSet: &includeUnset}
			switch filter.Operator {
			case "is_equal_to", "is_not_equal_to":
				if filter.PropertyKind == "text" {
					operation.OperationType = "STRING"
					operation.Operator = map[string]string{"is_equal_to": "IS_EQUAL_TO", "is_not_equal_to": "IS_NOT_EQUAL_TO"}[filter.Operator]
					value := filter.Value
					operation.Value = &value
				} else {
					operation.OperationType = "ENUMERATION"
					operation.Operator = map[string]string{"is_equal_to": "IS_ANY_OF", "is_not_equal_to": "IS_NONE_OF"}[filter.Operator]
					operation.Values = []string{filter.Value}
				}
			case "is_known", "is_not_known":
				operation.OperationType = "ALL_PROPERTY"
				operation.Operator = map[string]string{"is_known": "IS_KNOWN", "is_not_known": "IS_UNKNOWN"}[filter.Operator]
			}
			and.Filters = append(and.Filters, fakeContactSegmentFilter{FilterType: "PROPERTY", Property: filter.Property, Operation: operation})
		}
		root.FilterBranches = append(root.FilterBranches, and)
	}
	return root
}

func (f *FakeHubSpot) FailNextContactSegmentOperation(fault ContactSegmentFault) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextContactFault = fault
}

func (f *FakeHubSpot) ActiveContactSegmentIDs() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	ids := make([]string, 0)
	for id, entry := range f.contactSegments {
		if !entry.segment.Deleted {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}

func (f *FakeHubSpot) ContactSegmentSnapshot(id string) (hubspot.ContactSegment, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	entry := f.contactSegments[id]
	if entry == nil {
		return hubspot.ContactSegment{}, false
	}
	copy := entry.segment
	copy.FilterGroups = append([]hubspot.ContactSegmentFilterGroup(nil), entry.segment.FilterGroups...)
	return copy, true
}

func (f *FakeHubSpot) ContactSegmentWriteCounts(id string) (name, filters, deletes, restores int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if entry := f.contactSegments[id]; entry != nil {
		return entry.nameWrites, entry.filterWrites, entry.deleteWrites, entry.restoreWrites
	}
	return 0, 0, 0, 0
}

// ContactSegmentRequestCounts returns exact per-identity request counts.
func (f *FakeHubSpot) ContactSegmentRequestCounts(id string) (creates, reads, names, filters, deletes, restores int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if entry := f.contactSegments[id]; entry != nil {
		return entry.createWrites, entry.readWrites, entry.nameWrites, entry.filterWrites, entry.deleteWrites, entry.restoreWrites
	}
	return 0, 0, 0, 0, 0, 0
}

func (f *FakeHubSpot) DriftContactSegmentName(id, name string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	entry := f.contactSegments[id]
	if entry == nil || entry.segment.Deleted {
		return false
	}
	entry.segment.Name = name
	return true
}

func (f *FakeHubSpot) DriftContactSegmentFilters(id string, groups []hubspot.ContactSegmentFilterGroup) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	entry := f.contactSegments[id]
	if entry == nil || entry.segment.Deleted || entry.segment.ProcessingType != "dynamic" {
		return false
	}
	entry.segment.FilterGroups = groups
	entry.segment.Version++
	return true
}

func (f *FakeHubSpot) CompleteContactSegment(id string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	entry := f.contactSegments[id]
	if entry == nil || entry.segment.Deleted {
		return false
	}
	entry.segment.ProcessingStatus = "COMPLETE"
	entry.pendingReads = 0
	return true
}

func (f *FakeHubSpot) DeleteContactSegment(id string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	entry := f.contactSegments[id]
	if entry == nil || entry.segment.Deleted {
		return false
	}
	entry.segment.Deleted = true
	entry.segment.Version++
	entry.deleteWrites++
	return true
}

func (f *FakeHubSpot) PurgeContactSegment(id string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.contactSegments[id]; !ok {
		return false
	}
	delete(f.contactSegments, id)
	return true
}

func (f *FakeHubSpot) ContactSegmentIDsByNamePrefix(prefix string) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	ids := make([]string, 0)
	for id, entry := range f.contactSegments {
		if strings.HasPrefix(entry.segment.Name, prefix) {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}
