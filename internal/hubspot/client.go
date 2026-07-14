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
	"strconv"
	"strings"
)

// ClientSet is the provider's configured, alias-local typed client boundary.
// Credentials remain encapsulated by Transport and are never exposed as data.
type ClientSet struct {
	PropertyGroups *PropertyGroupClient
	Properties     *PropertyDefinitionClient
}

func NewClientSet(config TransportConfig) (*ClientSet, error) {
	transport, err := NewTransport(config)
	if err != nil {
		return nil, err
	}
	return &ClientSet{PropertyGroups: &PropertyGroupClient{transport: transport}, Properties: &PropertyDefinitionClient{transport: transport}}, nil
}

type PropertyDefinitionClient struct{ transport *Transport }

type PropertyOption struct {
	Value        string  `json:"value"`
	Label        string  `json:"label"`
	Description  *string `json:"description"`
	DisplayOrder *int64  `json:"displayOrder"`
	Hidden       *bool   `json:"hidden"`
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
