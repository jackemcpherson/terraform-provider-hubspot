// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

//go:build acceptance

package acceptance

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/jackemcpherson/terraform-provider-hubspot/internal/hubspot"
)

// FakeHubSpot is a stateful in-process fake of the HubSpot API surfaces this
// provider uses: property groups, property definitions, pipelines, forms,
// stages, and account-info. Construct one fresh instance per test with
// NewFakeHubSpot and serve it with httptest.NewServer; all state access is
// mutex-guarded so it tolerates concurrent requests from the Terraform CLI.
//
// It deliberately does not implement pagination, throttling emulation, or
// CRM record endpoints: the provider under test never exercises those
// surfaces, and adding them would only make the fake harder to trust.
type FakeHubSpot struct {
	mu sync.Mutex

	token    string
	portalID int64

	// groups is keyed by objectType then group name. Archived groups are
	// removed outright: HubSpot's groups endpoint has no archived-visibility
	// query, and every acceptance probe that archives a group expects a
	// subsequent read to 404, so there is nothing to gain from retaining a
	// tombstone.
	groups map[string]map[string]*hubspot.PropertyGroup

	// properties is keyed by objectType then property name. Each name can
	// have one active definition and one retained archived tombstone because
	// the current /2026-03 API permits immediate same-name reuse.
	properties map[string]map[string]*fakePropertyVersions

	// pipelines is keyed by objectType then server-assigned pipeline id and
	// retains archived entries for the same reason as properties.
	pipelines      map[string]map[string]*hubspot.Pipeline
	nextPipelineID int
	nextStageID    int

	// forms retains both active definitions and archived tombstones under the
	// generated Forms v3 ID. Archived visibility is selected explicitly.
	forms      map[string]*hubspot.FormDefinition
	nextFormID int
}

// NewFakeHubSpot returns a fake with fresh, empty state authenticating
// exactly one bearer token and reporting portalID from its account-info
// endpoint.
func NewFakeHubSpot(token string, portalID int64) *FakeHubSpot {
	return &FakeHubSpot{
		token:      token,
		portalID:   portalID,
		groups:     make(map[string]map[string]*hubspot.PropertyGroup),
		properties: make(map[string]map[string]*fakePropertyVersions),
		pipelines:  make(map[string]map[string]*hubspot.Pipeline),
		forms:      make(map[string]*hubspot.FormDefinition),
	}
}

func (f *FakeHubSpot) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	if !f.authenticated(request) {
		writeFakeError(response, http.StatusUnauthorized, "VALIDATION_ERROR", "", "This access token is not authorized.")
		return
	}
	response.Header().Set("Content-Type", "application/json")

	segments := pathSegments(request.URL.Path)
	switch {
	case len(segments) == 3 && segments[0] == "account-info" && segments[1] == "v3" && segments[2] == "details":
		f.handleAccountInfo(response, request)
	case len(segments) >= 4 && segments[0] == "crm" && segments[1] == "properties" && segments[2] == "2026-03":
		f.handleProperties(response, request, segments[3:])
	case len(segments) >= 4 && segments[0] == "crm" && segments[1] == "pipelines" && segments[2] == "2026-03":
		f.handlePipelines(response, request, segments[3:])
	case len(segments) >= 3 && segments[0] == "marketing" && segments[1] == "v3" && segments[2] == "forms":
		f.handleForms(response, request, segments[3:])
	default:
		writeFakeError(response, http.StatusNotFound, "OBJECT_NOT_FOUND", "", "No route matched this request.")
	}
}

// --- form definitions ---

func (f *FakeHubSpot) handleForms(response http.ResponseWriter, request *http.Request, rest []string) {
	switch len(rest) {
	case 0:
		f.handleFormCollection(response, request)
	case 1:
		f.handleFormItem(response, request, rest[0])
	default:
		writeFakeError(response, http.StatusNotFound, "OBJECT_NOT_FOUND", "", "No route matched this request.")
	}
}

func (f *FakeHubSpot) handleFormCollection(response http.ResponseWriter, request *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if request.Method != http.MethodPost {
		writeFakeError(response, http.StatusMethodNotAllowed, "VALIDATION_ERROR", "", "Unsupported method.")
		return
	}
	var body hubspot.FormDefinitionWrite
	if !decodeFakeBody(response, request, &body) {
		return
	}
	if !supportedFakeForm(body) {
		writeFakeError(response, http.StatusBadRequest, "VALIDATION_ERROR", "", "The form did not match the supported typed definition.")
		return
	}
	f.nextFormID++
	id := "form-" + strconv.Itoa(f.nextFormID)
	form := &hubspot.FormDefinition{ID: id, Archived: false, FormDefinitionWrite: body}
	f.forms[id] = form
	writeFakeJSON(response, http.StatusCreated, *form)
}

func (f *FakeHubSpot) handleFormItem(response http.ResponseWriter, request *http.Request, id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	form, exists := f.forms[id]
	switch request.Method {
	case http.MethodGet:
		archived := request.URL.Query().Get("archived") == "true"
		if !exists || form.Archived != archived {
			writeFakeError(response, http.StatusNotFound, "OBJECT_NOT_FOUND", "", "No form definition matched this identity.")
			return
		}
		writeFakeJSON(response, http.StatusOK, *form)
	case http.MethodDelete:
		if !exists || form.Archived {
			writeFakeError(response, http.StatusNotFound, "OBJECT_NOT_FOUND", "", "No active form definition matched this identity.")
			return
		}
		form.Archived = true
		response.WriteHeader(http.StatusNoContent)
	default:
		writeFakeError(response, http.StatusMethodNotAllowed, "VALIDATION_ERROR", "", "Unsupported method.")
	}
}

func supportedFakeForm(form hubspot.FormDefinitionWrite) bool {
	if form.FormType != "hubspot" || form.Name == "" || len(form.FieldGroups) != 1 ||
		form.FieldGroups[0].GroupType != "default_group" || form.FieldGroups[0].RichTextType != "text" ||
		len(form.FieldGroups[0].Fields) != 1 {
		return false
	}
	field := form.FieldGroups[0].Fields[0]
	return field.ObjectTypeID == "0-1" && !field.Hidden && field.Name == "email" && field.FieldType == "email" &&
		len(field.DependentFields) == 0 && !form.Configuration.CreateNewContactForNewEmail && form.Configuration.Editable &&
		form.Configuration.PostSubmitAction.Type == "thank_you" && form.Configuration.Cloneable && !form.Configuration.NotifyContactOwner &&
		form.Configuration.Archivable && len(form.Configuration.NotifyRecipients) == 0 && !form.DisplayOptions.RenderRawHTML &&
		form.DisplayOptions.Theme == "default_style" && form.LegalConsentOptions.Type == "none"
}

func (f *FakeHubSpot) authenticated(request *http.Request) bool {
	const prefix = "Bearer "
	header := request.Header.Get("Authorization")
	return strings.HasPrefix(header, prefix) && strings.TrimPrefix(header, prefix) == f.token
}

func (f *FakeHubSpot) handleAccountInfo(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writeFakeError(response, http.StatusMethodNotAllowed, "VALIDATION_ERROR", "", "Unsupported method.")
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	writeFakeJSON(response, http.StatusOK, hubspot.AccountInfo{PortalID: f.portalID})
}

// --- property groups and property definitions ---

func (f *FakeHubSpot) handleProperties(response http.ResponseWriter, request *http.Request, rest []string) {
	objectType := rest[0]
	switch {
	case len(rest) == 2 && rest[1] == "groups":
		f.handlePropertyGroupCollection(response, request, objectType)
	case len(rest) == 3 && rest[1] == "groups":
		f.handlePropertyGroupItem(response, request, objectType, rest[2])
	case len(rest) == 1:
		f.handlePropertyCollection(response, request, objectType)
	case len(rest) == 2:
		f.handlePropertyItem(response, request, objectType, rest[1])
	default:
		writeFakeError(response, http.StatusNotFound, "OBJECT_NOT_FOUND", "", "No route matched this request.")
	}
}

func (f *FakeHubSpot) handlePropertyGroupCollection(response http.ResponseWriter, request *http.Request, objectType string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch request.Method {
	case http.MethodGet:
		names := make([]string, 0, len(f.groups[objectType]))
		for name := range f.groups[objectType] {
			names = append(names, name)
		}
		sort.Strings(names)
		results := make([]hubspot.PropertyGroup, 0, len(names))
		for _, name := range names {
			results = append(results, *f.groups[objectType][name])
		}
		writeFakeJSON(response, http.StatusOK, map[string]any{"results": results})
	case http.MethodPost:
		var body struct {
			Name         string `json:"name"`
			Label        string `json:"label"`
			DisplayOrder int64  `json:"displayOrder"`
		}
		if !decodeFakeBody(response, request, &body) {
			return
		}
		if f.groups[objectType] == nil {
			f.groups[objectType] = make(map[string]*hubspot.PropertyGroup)
		}
		if _, exists := f.groups[objectType][body.Name]; exists {
			writeFakeError(response, http.StatusConflict, "VALIDATION_ERROR", "PropertyGroupError.GROUP_ALREADY_EXISTS", "A property group with this name already exists.")
			return
		}
		group := &hubspot.PropertyGroup{Name: body.Name, Label: body.Label, DisplayOrder: f.groupDisplayOrder(objectType, "", body.DisplayOrder)}
		f.groups[objectType][body.Name] = group
		writeFakeJSON(response, http.StatusCreated, *group)
	default:
		writeFakeError(response, http.StatusMethodNotAllowed, "VALIDATION_ERROR", "", "Unsupported method.")
	}
}

func (f *FakeHubSpot) handlePropertyGroupItem(response http.ResponseWriter, request *http.Request, objectType, name string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	group, exists := f.groups[objectType][name]
	switch request.Method {
	case http.MethodGet:
		if !exists {
			writeFakeError(response, http.StatusNotFound, "OBJECT_NOT_FOUND", "", "No property group matched this identity.")
			return
		}
		writeFakeJSON(response, http.StatusOK, *group)
	case http.MethodPatch:
		if !exists {
			writeFakeError(response, http.StatusNotFound, "OBJECT_NOT_FOUND", "", "No property group matched this identity.")
			return
		}
		var body struct {
			Label        string `json:"label"`
			DisplayOrder int64  `json:"displayOrder"`
		}
		if !decodeFakeBody(response, request, &body) {
			return
		}
		group.Label = body.Label
		group.DisplayOrder = f.groupDisplayOrder(objectType, name, body.DisplayOrder)
		writeFakeJSON(response, http.StatusOK, *group)
	case http.MethodDelete:
		if !exists {
			writeFakeError(response, http.StatusNotFound, "OBJECT_NOT_FOUND", "", "No property group matched this identity.")
			return
		}
		if f.groupHasActiveProperties(objectType, name) {
			writeFakeNestedError(response, http.StatusBadRequest, "VALIDATION_ERROR", "PropertyGroupError.GROUP_WITH_ACTIVE_PROPERTIES", "Can't delete or purge a group with active properties")
			return
		}
		delete(f.groups[objectType], name)
		response.WriteHeader(http.StatusNoContent)
	default:
		writeFakeError(response, http.StatusMethodNotAllowed, "VALIDATION_ERROR", "", "Unsupported method.")
	}
}

func (f *FakeHubSpot) groupHasActiveProperties(objectType, groupName string) bool {
	for _, versions := range f.properties[objectType] {
		if versions.Active != nil && versions.Active.GroupName == groupName {
			return true
		}
	}
	return false
}

type fakePropertyWrite struct {
	Name                 *string                  `json:"name"`
	Label                *string                  `json:"label"`
	GroupName            *string                  `json:"groupName"`
	Type                 *string                  `json:"type"`
	FieldType            *string                  `json:"fieldType"`
	Description          *string                  `json:"description"`
	DisplayOrder         *int64                   `json:"displayOrder"`
	FormField            *bool                    `json:"formField"`
	Hidden               *bool                    `json:"hidden"`
	HasUniqueValue       *bool                    `json:"hasUniqueValue"`
	DataSensitivity      *string                  `json:"dataSensitivity"`
	ExternalOptions      *bool                    `json:"externalOptions"`
	ShowCurrencySymbol   *bool                    `json:"showCurrencySymbol"`
	CalculationFormula   *string                  `json:"calculationFormula"`
	CurrencyPropertyName *string                  `json:"currencyPropertyName"`
	NumberDisplayHint    *string                  `json:"numberDisplayHint"`
	TextDisplayHint      *string                  `json:"textDisplayHint"`
	ReferencedObjectType *string                  `json:"referencedObjectType"`
	Options              []hubspot.PropertyOption `json:"options"`
}

type fakePropertyVersions struct {
	Active   *hubspot.PropertyDefinition
	Archived *hubspot.PropertyDefinition
}

func (f *FakeHubSpot) handlePropertyCollection(response http.ResponseWriter, request *http.Request, objectType string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch request.Method {
	case http.MethodGet:
		archived := request.URL.Query().Get("archived") == "true"
		names := make([]string, 0, len(f.properties[objectType]))
		for name := range f.properties[objectType] {
			names = append(names, name)
		}
		sort.Strings(names)
		results := make([]hubspot.PropertyDefinition, 0, len(names))
		for _, name := range names {
			versions := f.properties[objectType][name]
			property := versions.Active
			if archived {
				property = versions.Archived
			}
			if property != nil {
				results = append(results, *property)
			}
		}
		writeFakeJSON(response, http.StatusOK, map[string]any{"results": results})
	case http.MethodPost:
		var body fakePropertyWrite
		if !decodeFakeBody(response, request, &body) {
			return
		}
		if body.Name == nil || *body.Name == "" {
			writeFakeError(response, http.StatusBadRequest, "VALIDATION_ERROR", "", "A property name is required.")
			return
		}
		name := *body.Name
		if f.properties[objectType] == nil {
			f.properties[objectType] = make(map[string]*fakePropertyVersions)
		}
		versions, exists := f.properties[objectType][name]
		if exists && versions.Active != nil {
			writeFakeError(response, http.StatusConflict, "VALIDATION_ERROR", "PropertyValidationError.PROPERTY_EXISTS", "A property with this name already exists.")
			return
		}
		if !exists {
			versions = &fakePropertyVersions{}
			f.properties[objectType][name] = versions
		}
		archivedFalse := false
		definition := &hubspot.PropertyDefinition{
			Name: name, Label: valueOr(body.Label, ""), GroupName: valueOr(body.GroupName, ""),
			Type: valueOr(body.Type, ""), FieldType: valueOr(body.FieldType, ""),
			Description: body.Description, DisplayOrder: f.propertyDisplayOrder(objectType, valueOr(body.GroupName, ""), "", body.DisplayOrder), FormField: body.FormField,
			Hidden: body.Hidden, HasUniqueValue: body.HasUniqueValue, DataSensitivity: body.DataSensitivity,
			ExternalOptions: body.ExternalOptions, ShowCurrencySymbol: body.ShowCurrencySymbol,
			CalculationFormula: body.CalculationFormula, CurrencyPropertyName: body.CurrencyPropertyName,
			NumberDisplayHint: body.NumberDisplayHint, TextDisplayHint: body.TextDisplayHint,
			ReferencedObjectType: body.ReferencedObjectType, Options: normalizeOptionOrders(body.Options),
			Archived: &archivedFalse,
		}
		versions.Active = definition
		writeFakeJSON(response, http.StatusCreated, *definition)
	default:
		writeFakeError(response, http.StatusMethodNotAllowed, "VALIDATION_ERROR", "", "Unsupported method.")
	}
}

func (f *FakeHubSpot) handlePropertyItem(response http.ResponseWriter, request *http.Request, objectType, name string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	versions, exists := f.properties[objectType][name]
	switch request.Method {
	case http.MethodGet:
		archived := request.URL.Query().Get("archived") == "true"
		var property *hubspot.PropertyDefinition
		if exists {
			property = versions.Active
			if archived {
				property = versions.Archived
			}
		}
		if property == nil {
			writeFakeError(response, http.StatusNotFound, "OBJECT_NOT_FOUND", "", "No property definition matched this identity.")
			return
		}
		writeFakeJSON(response, http.StatusOK, *property)
	case http.MethodPatch:
		if !exists || versions.Active == nil {
			writeFakeError(response, http.StatusNotFound, "OBJECT_NOT_FOUND", "", "No active property definition matched this identity.")
			return
		}
		property := versions.Active
		var body fakePropertyWrite
		if !decodeFakeBody(response, request, &body) {
			return
		}
		if body.Label != nil {
			property.Label = *body.Label
		}
		if body.GroupName != nil {
			property.GroupName = *body.GroupName
		}
		if body.Type != nil {
			property.Type = *body.Type
		}
		if body.FieldType != nil {
			property.FieldType = *body.FieldType
		}
		property.Description = body.Description
		property.DisplayOrder = f.propertyDisplayOrder(objectType, property.GroupName, name, body.DisplayOrder)
		property.FormField = body.FormField
		property.Hidden = body.Hidden
		property.ShowCurrencySymbol = body.ShowCurrencySymbol
		property.CalculationFormula = body.CalculationFormula
		property.CurrencyPropertyName = body.CurrencyPropertyName
		property.NumberDisplayHint = body.NumberDisplayHint
		property.TextDisplayHint = body.TextDisplayHint
		property.ReferencedObjectType = body.ReferencedObjectType
		property.Options = normalizeOptionOrders(body.Options)
		writeFakeJSON(response, http.StatusOK, *property)
	case http.MethodDelete:
		if !exists || versions.Active == nil {
			writeFakeError(response, http.StatusNotFound, "OBJECT_NOT_FOUND", "", "No active property definition matched this identity.")
			return
		}
		archivedTrue := true
		versions.Active.Archived = &archivedTrue
		versions.Archived = versions.Active
		versions.Active = nil
		response.WriteHeader(http.StatusNoContent)
	default:
		writeFakeError(response, http.StatusMethodNotAllowed, "VALIDATION_ERROR", "", "Unsupported method.")
	}
}

// --- pipelines and stages ---

func (f *FakeHubSpot) handlePipelines(response http.ResponseWriter, request *http.Request, rest []string) {
	objectType := rest[0]
	switch {
	case len(rest) == 1:
		f.handlePipelineCollection(response, request, objectType)
	case len(rest) == 2:
		f.handlePipelineItem(response, request, objectType, rest[1])
	default:
		writeFakeError(response, http.StatusNotFound, "OBJECT_NOT_FOUND", "", "No route matched this request.")
	}
}

func (f *FakeHubSpot) handlePipelineCollection(response http.ResponseWriter, request *http.Request, objectType string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch request.Method {
	case http.MethodGet:
		ids := make([]string, 0, len(f.pipelines[objectType]))
		for id := range f.pipelines[objectType] {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		results := make([]hubspot.Pipeline, 0, len(ids))
		for _, id := range ids {
			pipeline := f.pipelines[objectType][id]
			if !pipeline.Archived {
				results = append(results, *pipeline)
			}
		}
		writeFakeJSON(response, http.StatusOK, map[string]any{"results": results})
	case http.MethodPost:
		var body hubspot.PipelineWrite
		if !decodeFakeBody(response, request, &body) {
			return
		}
		if len(body.Stages) == 0 {
			writeFakeError(response, http.StatusBadRequest, "VALIDATION_ERROR", "", "A pipeline requires at least one stage.")
			return
		}
		f.nextPipelineID++
		id := "pipeline-" + strconv.Itoa(f.nextPipelineID)
		pipeline := &hubspot.Pipeline{ID: id, Label: body.Label, DisplayOrder: body.DisplayOrder, Stages: f.normalizeStages(objectType, nil, body.Stages)}
		if f.pipelines[objectType] == nil {
			f.pipelines[objectType] = make(map[string]*hubspot.Pipeline)
		}
		f.pipelines[objectType][id] = pipeline
		writeFakeJSON(response, http.StatusCreated, *pipeline)
	default:
		writeFakeError(response, http.StatusMethodNotAllowed, "VALIDATION_ERROR", "", "Unsupported method.")
	}
}

func (f *FakeHubSpot) handlePipelineItem(response http.ResponseWriter, request *http.Request, objectType, id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	pipeline, exists := f.pipelines[objectType][id]
	switch request.Method {
	case http.MethodGet:
		archived := request.URL.Query().Get("archived") == "true"
		if !exists || pipeline.Archived != archived {
			writeFakeError(response, http.StatusNotFound, "OBJECT_NOT_FOUND", "", "No pipeline matched this identity.")
			return
		}
		writeFakeJSON(response, http.StatusOK, *pipeline)
	case http.MethodPut:
		if !exists {
			writeFakeError(response, http.StatusNotFound, "OBJECT_NOT_FOUND", "", "No pipeline matched this identity.")
			return
		}
		var body hubspot.PipelineWrite
		if !decodeFakeBody(response, request, &body) {
			return
		}
		pipeline.Label = body.Label
		pipeline.DisplayOrder = body.DisplayOrder
		pipeline.Stages = f.normalizeStages(objectType, pipeline.Stages, body.Stages)
		writeFakeJSON(response, http.StatusOK, *pipeline)
	case http.MethodPatch:
		if !exists {
			writeFakeError(response, http.StatusNotFound, "OBJECT_NOT_FOUND", "", "No pipeline matched this identity.")
			return
		}
		var body struct {
			Archived *bool `json:"archived"`
		}
		if !decodeFakeBody(response, request, &body) {
			return
		}
		if body.Archived != nil {
			pipeline.Archived = *body.Archived
		}
		writeFakeJSON(response, http.StatusOK, *pipeline)
	case http.MethodDelete:
		if !exists {
			writeFakeError(response, http.StatusNotFound, "OBJECT_NOT_FOUND", "", "No pipeline matched this identity.")
			return
		}
		pipeline.Archived = true
		response.WriteHeader(http.StatusNoContent)
	default:
		writeFakeError(response, http.StatusMethodNotAllowed, "VALIDATION_ERROR", "", "Unsupported method.")
	}
}

// normalizeStages assigns server-side stage identities to stages the client
// did not already identify and injects object-specific default metadata
// keys (HubSpot requires "probability" on deal stages and "ticketState" on
// ticket stages). It always builds fresh stage and metadata values rather
// than writing into the caller-supplied PipelineStageWrite slice or its
// metadata maps, so the client's payload is never mutated as a side effect.
func (f *FakeHubSpot) normalizeStages(objectType string, existing []hubspot.PipelineStage, writes []hubspot.PipelineStageWrite) []hubspot.PipelineStage {
	existingByID := make(map[string]hubspot.PipelineStage, len(existing))
	for _, stage := range existing {
		existingByID[stage.ID] = stage
	}
	stages := make([]hubspot.PipelineStage, 0, len(writes))
	for _, write := range writes {
		id := write.StageID
		if id == "" {
			f.nextStageID++
			id = "stage-" + strconv.Itoa(f.nextStageID)
		}
		metadata := make(map[string]string, len(write.Metadata)+1)
		for key, value := range write.Metadata {
			metadata[key] = value
		}
		switch objectType {
		case "deals":
			if _, ok := metadata["probability"]; !ok {
				metadata["probability"] = "0.0"
			}
		case "tickets":
			if _, ok := metadata["ticketState"]; !ok {
				metadata["ticketState"] = "OPEN"
			}
		}
		writePermissions := "EDITABLE"
		if previous, ok := existingByID[id]; ok && previous.WritePermissions != "" {
			writePermissions = previous.WritePermissions
		}
		stages = append(stages, hubspot.PipelineStage{ID: id, Label: write.Label, DisplayOrder: write.DisplayOrder, Metadata: metadata, WritePermissions: writePermissions})
	}
	return stages
}

// --- shared helpers ---

func pathSegments(path string) []string {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "/")
}

func decodeFakeBody(response http.ResponseWriter, request *http.Request, destination any) bool {
	defer request.Body.Close()
	if err := json.NewDecoder(request.Body).Decode(destination); err != nil {
		writeFakeError(response, http.StatusBadRequest, "VALIDATION_ERROR", "", "The request body could not be decoded.")
		return false
	}
	return true
}

func writeFakeJSON(response http.ResponseWriter, status int, body any) {
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(body)
}

// writeFakeError writes HubSpot's uniform error envelope: a top-level
// status/message plus category, subCategory, and correlationId.
func writeFakeError(response http.ResponseWriter, status int, category, subCategory, message string) {
	writeFakeJSON(response, status, map[string]any{
		"status":        "error",
		"message":       message,
		"category":      category,
		"subCategory":   subCategory,
		"correlationId": "fake-correlation-id",
	})
}

// writeFakeNestedError reproduces the nested-envelope shape HubSpot uses for
// some validation failures (and that this repository's transport already
// knows how to unwrap): the outer envelope's message is itself a JSON string
// carrying the real category and subCategory. The harness's failure-identity
// matching (acceptance.PropertyGroupHasActiveProperties) depends on exactly
// this shape.
func writeFakeNestedError(response http.ResponseWriter, status int, category, subCategory, message string) {
	nested, err := json.Marshal(map[string]any{
		"status":        "error",
		"message":       message,
		"category":      category,
		"subCategory":   subCategory,
		"correlationId": "fake-correlation-id",
	})
	if err != nil {
		panic(fmt.Sprintf("encode nested fake error: %v", err))
	}
	writeFakeJSON(response, status, map[string]any{
		"status":        "error",
		"message":       string(nested),
		"category":      category,
		"subCategory":   subCategory,
		"correlationId": "fake-correlation-id",
	})
}

func boolPtrValue(value *bool) bool {
	return value != nil && *value
}

func valueOr(value *string, fallback string) string {
	if value == nil {
		return fallback
	}
	return *value
}

func (f *FakeHubSpot) groupDisplayOrder(objectType, excludedName string, requested int64) int64 {
	if requested != -1 {
		return requested
	}
	last := int64(-1)
	for name, group := range f.groups[objectType] {
		if name != excludedName && group.DisplayOrder > last {
			last = group.DisplayOrder
		}
	}
	return last + 1
}

func (f *FakeHubSpot) propertyDisplayOrder(objectType, groupName, excludedName string, requested *int64) *int64 {
	if requested == nil || *requested != -1 {
		return copyInt64(requested)
	}
	last := int64(-1)
	for name, versions := range f.properties[objectType] {
		if name == excludedName || versions.Active == nil || versions.Active.GroupName != groupName || versions.Active.DisplayOrder == nil {
			continue
		}
		if *versions.Active.DisplayOrder > last {
			last = *versions.Active.DisplayOrder
		}
	}
	generated := last + 1
	return &generated
}

func normalizeOptionOrders(options []hubspot.PropertyOption) []hubspot.PropertyOption {
	if options == nil {
		return nil
	}
	last := int64(-1)
	for _, option := range options {
		if option.DisplayOrder != nil && *option.DisplayOrder > last {
			last = *option.DisplayOrder
		}
	}
	result := make([]hubspot.PropertyOption, 0, len(options))
	for _, option := range options {
		copy := hubspot.PropertyOption{
			Value: option.Value, Label: option.Label, Description: copyString(option.Description),
			DisplayOrder: copyInt64(option.DisplayOrder), Hidden: copyBool(option.Hidden),
		}
		if copy.DisplayOrder != nil && *copy.DisplayOrder == -1 {
			last++
			*copy.DisplayOrder = last
		}
		result = append(result, copy)
	}
	return result
}

func copyString(value *string) *string {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func copyInt64(value *int64) *int64 {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func copyBool(value *bool) *bool {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}
