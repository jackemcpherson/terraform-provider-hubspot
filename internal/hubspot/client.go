// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

package hubspot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

// ClientSet is the provider's configured, alias-local typed client boundary.
// Credentials remain encapsulated by Transport and are never exposed as data.
type ClientSet struct {
	PropertyGroups *PropertyGroupClient
	Properties     *PropertyDefinitionClient
	Pipelines      *PipelineClient
}

func NewClientSet(config TransportConfig) (*ClientSet, error) {
	transport, err := NewTransport(config)
	if err != nil {
		return nil, err
	}
	return &ClientSet{PropertyGroups: &PropertyGroupClient{transport: transport}, Properties: &PropertyDefinitionClient{transport: transport}, Pipelines: &PipelineClient{transport: transport}}, nil
}

type PipelineClient struct{ transport *Transport }
type Pipeline struct {
	ID           string          `json:"id"`
	Label        string          `json:"label"`
	DisplayOrder int64           `json:"displayOrder"`
	Stages       []PipelineStage `json:"stages"`
	Archived     bool            `json:"archived"`
}
type PipelineStage struct {
	ID               string            `json:"id"`
	Label            string            `json:"label"`
	DisplayOrder     int64             `json:"displayOrder"`
	Metadata         map[string]string `json:"metadata"`
	WritePermissions string            `json:"writePermissions"`
}
type PipelineStageWrite struct {
	Label        string            `json:"label"`
	DisplayOrder int64             `json:"displayOrder"`
	Metadata     map[string]string `json:"metadata"`
}
type PipelineWrite struct {
	Label        string               `json:"label"`
	DisplayOrder int64                `json:"displayOrder"`
	Stages       []PipelineStageWrite `json:"stages"`
}

func pipelinePath(objectType string) string {
	return "/crm/pipelines/2026-03/" + url.PathEscape(objectType)
}
func (c *PipelineClient) Get(ctx context.Context, objectType, id string) (Pipeline, error) {
	if err := validateObjectType(objectType); err != nil {
		return Pipeline{}, err
	}
	var out Pipeline
	if err := c.transport.Do(ctx, Operation{Name: "pipeline-read", Method: http.MethodGet, Path: pipelinePath(objectType) + "/" + url.PathEscape(id), Replay: ReplaySafe}, nil, &out); err != nil {
		return Pipeline{}, err
	}
	if out.ID == "" {
		return Pipeline{}, errors.New("HubSpot pipeline response omitted id")
	}
	return out, nil
}
func (c *PipelineClient) Create(ctx context.Context, objectType string, input PipelineWrite) (Pipeline, error) {
	if err := validateObjectType(objectType); err != nil {
		return Pipeline{}, err
	}
	if len(input.Stages) == 0 {
		return Pipeline{}, errors.New("pipeline requires at least one stage")
	}
	body, err := json.Marshal(input)
	if err != nil {
		return Pipeline{}, err
	}
	var out Pipeline
	if err := c.transport.Do(ctx, Operation{Name: "pipeline-create", Method: http.MethodPost, Path: pipelinePath(objectType), Replay: ReplayNever}, bytes.NewReader(body), &out); err != nil {
		return Pipeline{}, err
	}
	if out.ID == "" {
		return Pipeline{}, errors.New("HubSpot pipeline response omitted id")
	}
	return out, nil
}
func (c *PipelineClient) Update(ctx context.Context, objectType, id string, input PipelineWrite) (Pipeline, error) {
	body, err := json.Marshal(input)
	if err != nil {
		return Pipeline{}, err
	}
	var out Pipeline
	if err := c.transport.Do(ctx, Operation{Name: "pipeline-update", Method: http.MethodPut, Path: pipelinePath(objectType) + "/" + url.PathEscape(id), Replay: ReplayExplicit}, bytes.NewReader(body), &out); err != nil {
		return Pipeline{}, err
	}
	return out, nil
}
func (c *PipelineClient) Restore(ctx context.Context, objectType, id string) (Pipeline, error) {
	body := []byte(`{"archived":false}`)
	var out Pipeline
	if err := c.transport.Do(ctx, Operation{Name: "pipeline-restore", Method: http.MethodPatch, Path: pipelinePath(objectType) + "/" + url.PathEscape(id), Replay: ReplayExplicit}, bytes.NewReader(body), &out); err != nil {
		return Pipeline{}, err
	}
	return out, nil
}
func (c *PipelineClient) Archive(ctx context.Context, objectType, id string) error {
	return c.transport.Do(ctx, Operation{Name: "pipeline-archive", Method: http.MethodDelete, Path: pipelinePath(objectType) + "/" + url.PathEscape(id) + "?validateReferencesBeforeDelete=true", Replay: ReplayExplicit}, nil, nil)
}

type PropertyDefinitionClient struct{ transport *Transport }

type PropertyWrite struct {
	Name                 string
	Label                string
	GroupName            string
	Type                 string
	FieldType            string
	Description          *string
	DisplayOrder         *int64
	FormField            *bool
	Hidden               *bool
	HasUniqueValue       *bool
	DataSensitivity      *string
	ExternalOptions      *bool
	ShowCurrencySymbol   *bool
	CalculationFormula   *string
	CurrencyPropertyName *string
	NumberDisplayHint    *string
	TextDisplayHint      *string
	ReferencedObjectType *string
	Options              []PropertyOption
}

type PropertyOption struct {
	Value        string  `json:"value"`
	Label        string  `json:"label"`
	Description  *string `json:"description"`
	DisplayOrder *int64  `json:"displayOrder"`
	Hidden       *bool   `json:"hidden"`
}

type propertyOptionPayload struct {
	Value        string  `json:"value"`
	Label        string  `json:"label"`
	Description  *string `json:"description,omitempty"`
	DisplayOrder *int64  `json:"displayOrder,omitempty"`
	Hidden       *bool   `json:"hidden,omitempty"`
}

type ModificationMetadata struct {
	Archivable         *bool `json:"archivable"`
	ReadOnlyDefinition *bool `json:"readOnlyDefinition"`
	ReadOnlyValue      *bool `json:"readOnlyValue"`
	ReadOnlyOptions    *bool `json:"readOnlyOptions"`
}

type PropertyDefinition struct {
	Name                 string                `json:"name"`
	Label                string                `json:"label"`
	GroupName            string                `json:"groupName"`
	Type                 string                `json:"type"`
	FieldType            string                `json:"fieldType"`
	Description          *string               `json:"description"`
	DisplayOrder         *int64                `json:"displayOrder"`
	FormField            *bool                 `json:"formField"`
	Hidden               *bool                 `json:"hidden"`
	HasUniqueValue       *bool                 `json:"hasUniqueValue"`
	ExternalOptions      *bool                 `json:"externalOptions"`
	ReferencedObjectType *string               `json:"referencedObjectType"`
	ShowCurrencySymbol   *bool                 `json:"showCurrencySymbol"`
	CalculationFormula   *string               `json:"calculationFormula"`
	CurrencyPropertyName *string               `json:"currencyPropertyName"`
	NumberDisplayHint    *string               `json:"numberDisplayHint"`
	TextDisplayHint      *string               `json:"textDisplayHint"`
	DateDisplayHint      *string               `json:"dateDisplayHint"`
	DataSensitivity      *string               `json:"dataSensitivity"`
	SensitivityCategory  *string               `json:"sensitivityCategory"`
	Calculated           *bool                 `json:"calculated"`
	HubSpotDefined       *bool                 `json:"hubspotDefined"`
	Archived             *bool                 `json:"archived"`
	ArchivedAt           *string               `json:"archivedAt"`
	CreatedAt            *string               `json:"createdAt"`
	UpdatedAt            *string               `json:"updatedAt"`
	CreatedUserID        *string               `json:"createdUserId"`
	UpdatedUserID        *string               `json:"updatedUserId"`
	Options              []PropertyOption      `json:"options"`
	ModificationMetadata *ModificationMetadata `json:"modificationMetadata"`
}

type propertyDefinitionCollection struct {
	Results []PropertyDefinition `json:"results"`
	Paging  *struct {
		Next *struct {
			After string `json:"after"`
		} `json:"next"`
	} `json:"paging"`
}

func (c *PropertyDefinitionClient) List(ctx context.Context, objectType string, archived bool, sensitivity, locale string) ([]PropertyDefinition, error) {
	if err := validateObjectType(objectType); err != nil {
		return nil, err
	}
	if err := validateSensitivity(sensitivity); err != nil {
		return nil, err
	}
	results := make([]PropertyDefinition, 0)
	after := ""
	for page := 0; page < 100; page++ {
		query := "?archived=" + strconv.FormatBool(archived) + "&dataSensitivity=" + url.QueryEscape(sensitivity)
		if locale != "" {
			query += "&locale=" + url.QueryEscape(locale)
		}
		if after != "" {
			query += "&after=" + url.QueryEscape(after)
		}
		var response propertyDefinitionCollection
		if err := c.transport.Do(ctx, Operation{Name: "property-definition-list", Method: http.MethodGet, Path: propertyDefinitionPath(objectType) + query, Replay: ReplaySafe}, nil, &response); err != nil {
			return nil, err
		}
		results = append(results, response.Results...)
		if len(results) > 10000 {
			return nil, errors.New("property definition response exceeds result limit")
		}
		if response.Paging == nil || response.Paging.Next == nil || response.Paging.Next.After == "" {
			return results, nil
		}
		after = response.Paging.Next.After
	}
	return nil, errors.New("property definition pagination exceeded limit")
}

func (c *PropertyDefinitionClient) Get(ctx context.Context, objectType, name string, archived bool, sensitivity, locale string) (PropertyDefinition, error) {
	if err := validateObjectType(objectType); err != nil {
		return PropertyDefinition{}, err
	}
	if err := validateGroupName(name); err != nil {
		return PropertyDefinition{}, errors.New("invalid property name")
	}
	if err := validateSensitivity(sensitivity); err != nil {
		return PropertyDefinition{}, err
	}
	query := "?archived=" + strconv.FormatBool(archived) + "&dataSensitivity=" + url.QueryEscape(sensitivity)
	if locale != "" {
		query += "&locale=" + url.QueryEscape(locale)
	}
	var response PropertyDefinition
	if err := c.transport.Do(ctx, Operation{Name: "property-definition-read", Method: http.MethodGet, Path: propertyDefinitionPath(objectType) + "/" + url.PathEscape(name) + query, Replay: ReplaySafe}, nil, &response); err != nil {
		return PropertyDefinition{}, err
	}
	if response.Name == "" {
		return PropertyDefinition{}, errors.New("HubSpot property response omitted name")
	}
	return response, nil
}

func validateSensitivity(value string) error {
	switch value {
	case "non_sensitive", "sensitive", "highly_sensitive":
		return nil
	default:
		return errors.New("invalid property data sensitivity")
	}
}

func propertyDefinitionPath(objectType string) string {
	return "/crm/properties/2026-03/" + url.PathEscape(objectType)
}

func (c *PropertyDefinitionClient) Create(ctx context.Context, objectType string, input PropertyWrite) (PropertyDefinition, error) {
	if err := validateObjectType(objectType); err != nil {
		return PropertyDefinition{}, err
	}
	if err := validateGroupName(input.Name); err != nil {
		return PropertyDefinition{}, errors.New("invalid property name")
	}
	if err := validatePropertyShape(input.Type, input.FieldType, input.ExternalOptions, input.Options); err != nil {
		return PropertyDefinition{}, err
	}
	body, err := json.Marshal(propertyWritePayload(input, true))
	if err != nil {
		return PropertyDefinition{}, err
	}
	var response PropertyDefinition
	if err := c.transport.Do(ctx, Operation{Name: "property-create", Method: http.MethodPost, Path: propertyDefinitionPath(objectType), Replay: ReplayNever}, bytes.NewReader(body), &response); err != nil {
		return PropertyDefinition{}, err
	}
	if response.Name == "" {
		return PropertyDefinition{}, errors.New("HubSpot property response omitted name")
	}
	return response, nil
}

func (c *PropertyDefinitionClient) Update(ctx context.Context, objectType, name string, input PropertyWrite) (PropertyDefinition, error) {
	if err := validateObjectType(objectType); err != nil {
		return PropertyDefinition{}, err
	}
	if err := validateGroupName(name); err != nil {
		return PropertyDefinition{}, errors.New("invalid property name")
	}
	if err := validatePropertyShape(input.Type, input.FieldType, input.ExternalOptions, input.Options); err != nil {
		return PropertyDefinition{}, err
	}
	body, err := json.Marshal(propertyWritePayload(input, false))
	if err != nil {
		return PropertyDefinition{}, err
	}
	var response PropertyDefinition
	if err := c.transport.Do(ctx, Operation{Name: "property-update", Method: http.MethodPatch, Path: propertyDefinitionPath(objectType) + "/" + url.PathEscape(name), Replay: ReplayExplicit}, bytes.NewReader(body), &response); err != nil {
		return PropertyDefinition{}, err
	}
	if response.Name == "" {
		return PropertyDefinition{}, errors.New("HubSpot property response omitted name")
	}
	return response, nil
}

func (c *PropertyDefinitionClient) Archive(ctx context.Context, objectType, name string) error {
	if err := validateObjectType(objectType); err != nil {
		return err
	}
	if err := validateGroupName(name); err != nil {
		return errors.New("invalid property name")
	}
	return c.transport.Do(ctx, Operation{Name: "property-archive", Method: http.MethodDelete, Path: propertyDefinitionPath(objectType) + "/" + url.PathEscape(name), Replay: ReplayExplicit}, nil, nil)
}

func propertyWritePayload(input PropertyWrite, create bool) map[string]any {
	payload := map[string]any{"label": input.Label, "groupName": input.GroupName, "type": input.Type, "fieldType": input.FieldType}
	for key, value := range map[string]any{"description": input.Description, "displayOrder": input.DisplayOrder, "formField": input.FormField, "hidden": input.Hidden, "showCurrencySymbol": input.ShowCurrencySymbol, "calculationFormula": input.CalculationFormula, "currencyPropertyName": input.CurrencyPropertyName, "numberDisplayHint": input.NumberDisplayHint, "textDisplayHint": input.TextDisplayHint, "referencedObjectType": input.ReferencedObjectType} {
		if value != nil {
			payload[key] = value
		}
	}
	if create {
		payload["name"] = input.Name
		if input.HasUniqueValue != nil {
			payload["hasUniqueValue"] = input.HasUniqueValue
		}
		if input.DataSensitivity != nil {
			payload["dataSensitivity"] = input.DataSensitivity
		}
		if input.ExternalOptions != nil {
			payload["externalOptions"] = input.ExternalOptions
		}
	}
	if input.Options != nil {
		options := append([]PropertyOption(nil), input.Options...)
		sort.Slice(options, func(i, j int) bool { return options[i].Value < options[j].Value })
		encoded := make([]propertyOptionPayload, 0, len(options))
		for _, option := range options {
			encoded = append(encoded, propertyOptionPayload(option))
		}
		payload["options"] = encoded
	}
	return payload
}

func validatePropertyShape(kind, field string, external *bool, options []PropertyOption) error {
	valid := map[string]map[string]bool{"bool": {"booleancheckbox": true, "calculation_equation": true}, "enumeration": {"booleancheckbox": true, "checkbox": true, "radio": true, "select": true, "calculation_equation": true}, "date": {"date": true}, "datetime": {"date": true}, "string": {"file": true, "text": true, "textarea": true, "calculation_equation": true, "html": true, "phonenumber": true}, "number": {"number": true, "calculation_equation": true}}
	if !valid[kind][field] {
		return errors.New("invalid property type and field type combination")
	}
	if kind == "enumeration" && (external == nil || !*external) && len(options) == 0 {
		return errors.New("enumeration properties require options")
	}
	if kind != "enumeration" || (external != nil && *external) {
		if len(options) != 0 {
			return errors.New("options are only valid for non-external enumeration properties")
		}
	}
	for _, option := range options {
		if option.Value == "" || option.Label == "" {
			return errors.New("property options require value and label")
		}
	}
	return nil
}

type PropertyGroupClient struct {
	transport *Transport
}

type PropertyGroup struct {
	Name         string `json:"name"`
	Label        string `json:"label"`
	DisplayOrder int64  `json:"displayOrder"`
	Archived     bool   `json:"archived"`
}

type propertyGroupRead = PropertyGroup

type propertyGroupCollection struct {
	Results []propertyGroupRead `json:"results"`
}

type PropertyGroupCreate struct {
	Name         string
	Label        string
	DisplayOrder int64
}

type PropertyGroupUpdate struct {
	Label        string
	DisplayOrder int64
}

func (c *PropertyGroupClient) List(ctx context.Context, objectType string) ([]PropertyGroup, error) {
	if err := validateObjectType(objectType); err != nil {
		return nil, err
	}
	var response propertyGroupCollection
	if err := c.transport.Do(ctx, Operation{
		Name:   "property-group-list",
		Method: http.MethodGet,
		Path:   propertyGroupPath(objectType),
		Replay: ReplaySafe,
	}, nil, &response); err != nil {
		return nil, err
	}
	return append([]PropertyGroup(nil), response.Results...), nil
}

func (c *PropertyGroupClient) Get(ctx context.Context, objectType, name string) (PropertyGroup, error) {
	if err := validateObjectType(objectType); err != nil {
		return PropertyGroup{}, err
	}
	if err := validateGroupName(name); err != nil {
		return PropertyGroup{}, err
	}
	var response propertyGroupRead
	if err := c.transport.Do(ctx, Operation{
		Name:   "property-group-read",
		Method: http.MethodGet,
		Path:   propertyGroupPath(objectType) + "/" + url.PathEscape(name),
		Replay: ReplaySafe,
	}, nil, &response); err != nil {
		return PropertyGroup{}, err
	}
	return response, nil
}

func (c *PropertyGroupClient) Create(ctx context.Context, objectType string, input PropertyGroupCreate) (PropertyGroup, error) {
	if err := validateObjectType(objectType); err != nil {
		return PropertyGroup{}, err
	}
	if err := validateGroupName(input.Name); err != nil {
		return PropertyGroup{}, err
	}
	if input.Label == "" {
		return PropertyGroup{}, errors.New("property group label must not be empty")
	}
	body, err := json.Marshal(map[string]any{"name": input.Name, "label": input.Label, "displayOrder": input.DisplayOrder})
	if err != nil {
		return PropertyGroup{}, err
	}
	var response propertyGroupRead
	if err := c.transport.Do(ctx, Operation{
		Name:   "property-group-create",
		Method: http.MethodPost,
		Path:   propertyGroupPath(objectType),
		Replay: ReplayNever,
	}, bytes.NewReader(body), &response); err != nil {
		return PropertyGroup{}, err
	}
	return response, nil
}

func (c *PropertyGroupClient) Update(ctx context.Context, objectType, name string, input PropertyGroupUpdate) (PropertyGroup, error) {
	if err := validateObjectType(objectType); err != nil {
		return PropertyGroup{}, err
	}
	if err := validateGroupName(name); err != nil {
		return PropertyGroup{}, err
	}
	if input.Label == "" {
		return PropertyGroup{}, errors.New("property group label must not be empty")
	}
	body, err := json.Marshal(map[string]any{"label": input.Label, "displayOrder": input.DisplayOrder})
	if err != nil {
		return PropertyGroup{}, err
	}
	var response propertyGroupRead
	if err := c.transport.Do(ctx, Operation{
		Name:   "property-group-update",
		Method: http.MethodPatch,
		Path:   propertyGroupPath(objectType) + "/" + url.PathEscape(name),
		Replay: ReplayExplicit,
	}, bytes.NewReader(body), &response); err != nil {
		return PropertyGroup{}, err
	}
	return response, nil
}

func (c *PropertyGroupClient) Archive(ctx context.Context, objectType, name string) error {
	if err := validateObjectType(objectType); err != nil {
		return err
	}
	if err := validateGroupName(name); err != nil {
		return err
	}
	return c.transport.Do(ctx, Operation{
		Name:   "property-group-archive",
		Method: http.MethodDelete,
		Path:   propertyGroupPath(objectType) + "/" + url.PathEscape(name),
		Replay: ReplayExplicit,
	}, nil, nil)
}

func propertyGroupPath(objectType string) string {
	return "/crm/properties/2026-03/" + url.PathEscape(objectType) + "/groups"
}

func validateObjectType(value string) error {
	if value == "" || value != strings.TrimSpace(value) || strings.ContainsAny(value, "/?#") {
		return fmt.Errorf("invalid CRM object type")
	}
	return nil
}

func validateGroupName(value string) error {
	if value == "" || value != strings.TrimSpace(value) || strings.ContainsAny(value, "/?#") {
		return fmt.Errorf("invalid property group name")
	}
	return nil
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
