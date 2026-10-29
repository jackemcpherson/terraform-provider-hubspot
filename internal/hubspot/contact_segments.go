// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

package hubspot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/url"
	"sort"
	"strings"
)

const contactSegmentsPath = "/crm/lists/2026-03"

// ContactSegmentClient manages Contact segment definitions by generated ILS
// list ID. Memberships and derived size are outside this client boundary.
type ContactSegmentClient struct{ transport *Transport }

// ContactSegment is the supported semantic projection of one definition.
type ContactSegment struct {
	ID               string
	Version          int64
	Name             string
	ProcessingStatus string
	ProcessingType   string
	Deleted          bool
	FilterGroups     []ContactSegmentFilterGroup
}

// ContactSegmentWrite is the complete supported desired definition.
type ContactSegmentWrite struct {
	Name           string
	ProcessingType string
	FilterGroups   []ContactSegmentFilterGroup
}

// ContactSegmentFilterGroup is one OR branch containing AND predicates.
type ContactSegmentFilterGroup struct {
	Filters []ContactSegmentFilter
}

// ContactSegmentFilter is one supported Contact property predicate.
type ContactSegmentFilter struct {
	Property     string
	Operator     string
	PropertyKind string
	Value        string
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
	OperationType                string   `json:"operationType"`
	Operator                     string   `json:"operator"`
	Value                        *string  `json:"value,omitempty"`
	Values                       []string `json:"values,omitempty"`
	IncludeObjectsWithNoValueSet *bool    `json:"includeObjectsWithNoValueSet"`
}

// Create creates one definition without replaying an ambiguous write.
func (c *ContactSegmentClient) Create(ctx context.Context, input ContactSegmentWrite) (ContactSegment, error) {
	payload, err := contactSegmentCreateFromWrite(input)
	if err != nil {
		return ContactSegment{}, err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return ContactSegment{}, err
	}
	var envelope contactSegmentEnvelope
	err = c.transport.Do(ctx, Operation{
		Name: "contact-segment-create", Method: http.MethodPost,
		Path: contactSegmentsPath, Replay: ReplayNever,
	}, bytes.NewReader(body), &envelope)
	segment, modelErr := contactSegmentFromWire(envelope.List, false)
	if err != nil {
		markContactSegmentAmbiguousWrite(err)
		return segment, err
	}
	if modelErr != nil {
		return segment, contactSegmentAmbiguousModelError("contact-segment-create", http.StatusCreated, modelErr)
	}
	if segment.ID == "" {
		return segment, &Error{
			Operation: "contact-segment-create", Status: http.StatusCreated,
			Cause:     errors.New("HubSpot contact segment create response omitted listId"),
			Ambiguous: true,
		}
	}
	return segment, nil
}

// Get reads one exact definition and includes its complete filter tree.
func (c *ContactSegmentClient) Get(ctx context.Context, id string) (ContactSegment, error) {
	if !validContactSegmentID(id) {
		return ContactSegment{}, errors.New("contact segment ID must be a canonical positive decimal string")
	}
	query := url.Values{"includeFilters": []string{"true"}}
	var envelope contactSegmentEnvelope
	if err := c.transport.Do(ctx, Operation{
		Name: "contact-segment-read", Method: http.MethodGet,
		Path: contactSegmentsPath + "/" + url.PathEscape(id) + "?" + query.Encode(), Replay: ReplaySafe,
	}, nil, &envelope); err != nil {
		return ContactSegment{}, err
	}
	segment, err := contactSegmentFromWire(envelope.List, true)
	if err != nil {
		return segment, err
	}
	if segment.ID != id {
		return segment, errors.New("HubSpot contact segment response returned a different listId")
	}
	return segment, nil
}

// UpdateName changes only the presentation name of one exact definition.
func (c *ContactSegmentClient) UpdateName(ctx context.Context, id, name string) (ContactSegment, error) {
	if !validContactSegmentID(id) {
		return ContactSegment{}, errors.New("contact segment ID must be a canonical positive decimal string")
	}
	if strings.TrimSpace(name) == "" {
		return ContactSegment{}, errors.New("contact segment name must not be blank")
	}
	query := url.Values{"includeFilters": []string{"true"}, "listName": []string{name}}
	var envelope updatedContactSegmentEnvelope
	err := c.transport.Do(ctx, Operation{
		Name: "contact-segment-name-update", Method: http.MethodPut,
		Path:   contactSegmentsPath + "/" + url.PathEscape(id) + "/update-list-name?" + query.Encode(),
		Replay: ReplayNever,
	}, nil, &envelope)
	segment, modelErr := contactSegmentFromWire(envelope.UpdatedList, false)
	if err != nil {
		markContactSegmentAmbiguousWrite(err)
		return segment, err
	}
	if modelErr != nil {
		return segment, contactSegmentAmbiguousModelError("contact-segment-name-update", http.StatusOK, modelErr)
	}
	if segment.ID != id {
		return segment, errors.New("HubSpot contact segment name update returned a different listId")
	}
	return segment, nil
}

// UpdateFilters replaces the complete filter tree of one dynamic definition.
func (c *ContactSegmentClient) UpdateFilters(ctx context.Context, id string, groups []ContactSegmentFilterGroup) (ContactSegment, error) {
	if !validContactSegmentID(id) {
		return ContactSegment{}, errors.New("contact segment ID must be a canonical positive decimal string")
	}
	branch, err := contactSegmentBranchFromGroups(groups)
	if err != nil {
		return ContactSegment{}, err
	}
	body, err := json.Marshal(contactSegmentFilterUpdate{FilterBranch: branch})
	if err != nil {
		return ContactSegment{}, err
	}
	var envelope updatedContactSegmentEnvelope
	err = c.transport.Do(ctx, Operation{
		Name: "contact-segment-filter-update", Method: http.MethodPut,
		Path:   contactSegmentsPath + "/" + url.PathEscape(id) + "/update-list-filters",
		Replay: ReplayNever,
	}, bytes.NewReader(body), &envelope)
	segment, modelErr := contactSegmentFromWire(envelope.UpdatedList, false)
	if err != nil {
		markContactSegmentAmbiguousWrite(err)
		return segment, err
	}
	if modelErr != nil {
		return segment, contactSegmentAmbiguousModelError("contact-segment-filter-update", http.StatusOK, modelErr)
	}
	if segment.ID != id {
		return segment, errors.New("HubSpot contact segment filter update returned a different listId")
	}
	return segment, nil
}

// Delete moves one exact definition into HubSpot's retained tombstone window.
func (c *ContactSegmentClient) Delete(ctx context.Context, id string) error {
	if !validContactSegmentID(id) {
		return errors.New("contact segment ID must be a canonical positive decimal string")
	}
	err := c.transport.Do(ctx, Operation{
		Name: "contact-segment-delete", Method: http.MethodDelete,
		Path: contactSegmentsPath + "/" + url.PathEscape(id), Replay: ReplayNever,
	}, nil, nil)
	markContactSegmentAmbiguousWrite(err)
	return err
}

// Restore returns one retained exact definition to active use.
func (c *ContactSegmentClient) Restore(ctx context.Context, id string) error {
	if !validContactSegmentID(id) {
		return errors.New("contact segment ID must be a canonical positive decimal string")
	}
	err := c.transport.Do(ctx, Operation{
		Name: "contact-segment-restore", Method: http.MethodPut,
		Path: contactSegmentsPath + "/" + url.PathEscape(id) + "/restore", Replay: ReplayNever,
	}, nil, nil)
	markContactSegmentAmbiguousWrite(err)
	return err
}

func contactSegmentCreateFromWrite(input ContactSegmentWrite) (contactSegmentCreate, error) {
	if strings.TrimSpace(input.Name) == "" {
		return contactSegmentCreate{}, errors.New("contact segment name must not be blank")
	}
	if input.ProcessingType != "manual" && input.ProcessingType != "dynamic" && input.ProcessingType != "snapshot" {
		return contactSegmentCreate{}, errors.New("unsupported Contact segment processing type")
	}
	if input.ProcessingType == "manual" && len(input.FilterGroups) != 0 {
		return contactSegmentCreate{}, errors.New("manual Contact segment must not contain filters")
	}
	if input.ProcessingType != "manual" && len(input.FilterGroups) == 0 {
		return contactSegmentCreate{}, errors.New("dynamic and snapshot Contact segments require filters")
	}
	payload := contactSegmentCreate{
		Name: input.Name, ObjectTypeID: "0-1",
		ProcessingType: strings.ToUpper(input.ProcessingType),
	}
	if len(input.FilterGroups) == 0 {
		return payload, nil
	}
	branch, err := contactSegmentBranchFromGroups(input.FilterGroups)
	if err != nil {
		return contactSegmentCreate{}, err
	}
	payload.FilterBranch = &branch
	return payload, nil
}

func contactSegmentBranchFromGroups(groups []ContactSegmentFilterGroup) (contactSegmentFilterBranch, error) {
	if len(groups) == 0 {
		return contactSegmentFilterBranch{}, errors.New("contact segment filter tree requires at least one group")
	}
	root := contactSegmentFilterBranch{
		FilterBranchType: "OR", FilterBranchOperator: "OR",
		FilterBranches: make([]contactSegmentFilterBranch, 0, len(groups)),
		Filters:        []contactSegmentFilter{},
	}
	for _, group := range groups {
		if len(group.Filters) == 0 {
			return contactSegmentFilterBranch{}, errors.New("contact segment filter group requires at least one filter")
		}
		and := contactSegmentFilterBranch{
			FilterBranchType: "AND", FilterBranchOperator: "AND",
			FilterBranches: []contactSegmentFilterBranch{},
			Filters:        make([]contactSegmentFilter, 0, len(group.Filters)),
		}
		for _, filter := range group.Filters {
			wire, err := contactSegmentFilterFromSemantic(filter)
			if err != nil {
				return contactSegmentFilterBranch{}, err
			}
			and.Filters = append(and.Filters, wire)
		}
		root.FilterBranches = append(root.FilterBranches, and)
	}
	return root, nil
}

func contactSegmentFilterFromSemantic(filter ContactSegmentFilter) (contactSegmentFilter, error) {
	if strings.TrimSpace(filter.Property) == "" {
		return contactSegmentFilter{}, errors.New("contact segment filter property must not be blank")
	}
	includeUnset := false
	operation := contactSegmentFilterOperation{IncludeObjectsWithNoValueSet: &includeUnset}
	switch filter.Operator {
	case "is_equal_to", "is_not_equal_to":
		if strings.TrimSpace(filter.Value) == "" {
			return contactSegmentFilter{}, errors.New("contact segment equality filter value must not be blank")
		}
		switch filter.PropertyKind {
		case "text":
			operation.OperationType = "STRING"
			operation.Operator = map[string]string{"is_equal_to": "IS_EQUAL_TO", "is_not_equal_to": "IS_NOT_EQUAL_TO"}[filter.Operator]
			operation.Value = &filter.Value
		case "select":
			operation.OperationType = "ENUMERATION"
			operation.Operator = map[string]string{"is_equal_to": "IS_ANY_OF", "is_not_equal_to": "IS_NONE_OF"}[filter.Operator]
			operation.Values = []string{filter.Value}
		default:
			return contactSegmentFilter{}, errors.New("contact segment equality filter requires text or select property kind")
		}
	case "is_known", "is_not_known":
		if filter.PropertyKind != "" || filter.Value != "" {
			return contactSegmentFilter{}, errors.New("contact segment presence filter forbids property kind and value")
		}
		operation.OperationType = "ALL_PROPERTY"
		operation.Operator = map[string]string{"is_known": "IS_KNOWN", "is_not_known": "IS_UNKNOWN"}[filter.Operator]
	default:
		return contactSegmentFilter{}, errors.New("unsupported Contact segment operator")
	}
	return contactSegmentFilter{
		FilterType: "PROPERTY", Property: filter.Property, Operation: operation,
	}, nil
}

func contactSegmentFromWire(wire contactSegmentWire, requireFilters bool) (ContactSegment, error) {
	segment := ContactSegment{
		ID: wire.ListID, Version: wire.ListVersion, Name: wire.Name,
		ProcessingStatus: wire.ProcessingStatus,
		ProcessingType:   strings.ToLower(wire.ProcessingType),
		Deleted:          wire.DeletedAt != nil && strings.TrimSpace(*wire.DeletedAt) != "",
	}
	if !validContactSegmentID(segment.ID) || segment.Version <= 0 || strings.TrimSpace(segment.Name) == "" ||
		wire.ObjectTypeID != "0-1" || strings.TrimSpace(segment.ProcessingStatus) == "" {
		return segment, errors.New("HubSpot contact segment response omitted or changed a required definition field")
	}
	if segment.ProcessingType != "manual" && segment.ProcessingType != "dynamic" && segment.ProcessingType != "snapshot" {
		return segment, errors.New("HubSpot contact segment response returned an unsupported processing type")
	}
	if wire.DeletedAt != nil && strings.TrimSpace(*wire.DeletedAt) == "" {
		return segment, errors.New("HubSpot contact segment response returned an empty deletedAt timestamp")
	}
	if wire.FilterBranch == nil {
		if requireFilters && segment.ProcessingType != "manual" {
			return segment, errors.New("HubSpot contact segment response omitted the filter definition")
		}
		return segment, nil
	}
	groups, err := contactSegmentGroupsFromBranch(*wire.FilterBranch)
	if err != nil {
		return segment, err
	}
	if segment.ProcessingType == "manual" {
		return segment, errors.New("HubSpot manual contact segment returned filters")
	}
	segment.FilterGroups = groups
	return segment, nil
}

func contactSegmentGroupsFromBranch(root contactSegmentFilterBranch) ([]ContactSegmentFilterGroup, error) {
	if root.FilterBranchType != "OR" || root.FilterBranchOperator != "OR" || len(root.Filters) != 0 || len(root.FilterBranches) == 0 {
		return nil, errors.New("HubSpot contact segment returned an unsupported root filter branch")
	}
	groups := make([]ContactSegmentFilterGroup, 0, len(root.FilterBranches))
	seenGroups := make(map[string]struct{}, len(root.FilterBranches))
	for _, branch := range root.FilterBranches {
		if branch.FilterBranchType != "AND" || branch.FilterBranchOperator != "AND" || len(branch.FilterBranches) != 0 || len(branch.Filters) == 0 {
			return nil, errors.New("HubSpot contact segment returned an unsupported nested filter branch")
		}
		group := ContactSegmentFilterGroup{Filters: make([]ContactSegmentFilter, 0, len(branch.Filters))}
		seenFilters := make(map[string]struct{}, len(branch.Filters))
		filterKeys := make([]string, 0, len(branch.Filters))
		for _, wire := range branch.Filters {
			filter, err := contactSegmentFilterFromWire(wire)
			if err != nil {
				return nil, err
			}
			encoded, _ := json.Marshal(filter)
			if _, exists := seenFilters[string(encoded)]; exists {
				return nil, errors.New("HubSpot contact segment returned duplicate property filters")
			}
			seenFilters[string(encoded)] = struct{}{}
			filterKeys = append(filterKeys, string(encoded))
			group.Filters = append(group.Filters, filter)
		}
		sort.Strings(filterKeys)
		groupKey := strings.Join(filterKeys, "\x00")
		if _, exists := seenGroups[groupKey]; exists {
			return nil, errors.New("HubSpot contact segment returned duplicate filter groups")
		}
		seenGroups[groupKey] = struct{}{}
		groups = append(groups, group)
	}
	return groups, nil
}

func contactSegmentFilterFromWire(wire contactSegmentFilter) (ContactSegmentFilter, error) {
	if wire.FilterType != "PROPERTY" || strings.TrimSpace(wire.Property) == "" ||
		wire.Operation.IncludeObjectsWithNoValueSet == nil || *wire.Operation.IncludeObjectsWithNoValueSet {
		return ContactSegmentFilter{}, errors.New("HubSpot contact segment returned an unsupported property filter")
	}
	filter := ContactSegmentFilter{Property: wire.Property}
	switch wire.Operation.OperationType {
	case "STRING":
		if wire.Operation.Value == nil || strings.TrimSpace(*wire.Operation.Value) == "" || len(wire.Operation.Values) != 0 {
			return ContactSegmentFilter{}, errors.New("HubSpot contact segment returned an unsupported STRING operation")
		}
		filter.PropertyKind, filter.Value = "text", *wire.Operation.Value
		filter.Operator = map[string]string{"IS_EQUAL_TO": "is_equal_to", "IS_NOT_EQUAL_TO": "is_not_equal_to"}[wire.Operation.Operator]
	case "MULTISTRING":
		if wire.Operation.Value != nil || len(wire.Operation.Values) != 1 || strings.TrimSpace(wire.Operation.Values[0]) == "" {
			return ContactSegmentFilter{}, errors.New("HubSpot contact segment returned an unsupported MULTISTRING operation")
		}
		filter.PropertyKind, filter.Value = "text", wire.Operation.Values[0]
		filter.Operator = map[string]string{"IS_EQUAL_TO": "is_equal_to", "IS_NOT_EQUAL_TO": "is_not_equal_to"}[wire.Operation.Operator]
	case "ENUMERATION":
		if wire.Operation.Value != nil || len(wire.Operation.Values) != 1 || strings.TrimSpace(wire.Operation.Values[0]) == "" {
			return ContactSegmentFilter{}, errors.New("HubSpot contact segment returned an unsupported ENUMERATION operation")
		}
		filter.PropertyKind, filter.Value = "select", wire.Operation.Values[0]
		filter.Operator = map[string]string{"IS_ANY_OF": "is_equal_to", "IS_NONE_OF": "is_not_equal_to"}[wire.Operation.Operator]
	case "ALL_PROPERTY":
		if wire.Operation.Value != nil || len(wire.Operation.Values) != 0 {
			return ContactSegmentFilter{}, errors.New("HubSpot contact segment returned an unsupported ALL_PROPERTY operation")
		}
		filter.Operator = map[string]string{"IS_KNOWN": "is_known", "IS_UNKNOWN": "is_not_known"}[wire.Operation.Operator]
	default:
		return ContactSegmentFilter{}, errors.New("HubSpot contact segment returned an unsupported property operation type")
	}
	if filter.Operator == "" {
		return ContactSegmentFilter{}, errors.New("HubSpot contact segment returned an unsupported property operator")
	}
	return filter, nil
}

func validContactSegmentID(id string) bool {
	value, ok := new(big.Int).SetString(id, 10)
	return ok && value.Sign() > 0 && value.String() == id
}

func markContactSegmentAmbiguousWrite(err error) {
	var apiError *Error
	if errors.As(err, &apiError) && ((apiError.Status >= 200 && apiError.Status < 300) || apiError.Status >= 500) {
		apiError.Ambiguous = true
	}
}

func contactSegmentAmbiguousModelError(operation string, status int, cause error) error {
	return &Error{Operation: operation, Status: status, Cause: cause, Ambiguous: true}
}
