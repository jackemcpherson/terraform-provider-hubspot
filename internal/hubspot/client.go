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
	"strings"
)

// ClientSet is the provider's configured, alias-local typed client boundary.
// Credentials remain encapsulated by Transport and are never exposed as data.
type ClientSet struct {
	PropertyGroups *PropertyGroupClient
}

func NewClientSet(config TransportConfig) (*ClientSet, error) {
	transport, err := NewTransport(config)
	if err != nil {
		return nil, err
	}
	return &ClientSet{PropertyGroups: &PropertyGroupClient{transport: transport}}, nil
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
