// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

package hubspot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

const (
	accountMembershipPageLimit    = "100"
	accountMembershipWaitAttempts = 20
	accountMembershipWaitDelay    = time.Second
)

// AccountMembershipClient manages account membership through the Settings user
// API. Settings user IDs are distinct from CRM user profile IDs.
type AccountMembershipClient struct{ transport *Transport }

// AccountMembership is the narrow response contract used by the provider.
// HubSpot can add response fields without changing this contract.
type AccountMembership struct {
	ID               string   `json:"id"`
	Email            string   `json:"email"`
	FirstName        string   `json:"firstName"`
	LastName         string   `json:"lastName"`
	SuperAdmin       bool     `json:"superAdmin"`
	RoleID           string   `json:"roleId"`
	RoleIDs          []string `json:"roleIds"`
	PrimaryTeamID    *string  `json:"primaryTeamId"`
	SecondaryTeamIDs []string `json:"secondaryTeamIds"`
}

// AccountMembershipCreate contains only documented membership creation
// fields. Welcome email delivery is an explicit creation-time choice.
type AccountMembershipCreate struct {
	Email            string `json:"email"`
	FirstName        string `json:"firstName,omitempty"`
	LastName         string `json:"lastName,omitempty"`
	SendWelcomeEmail bool   `json:"sendWelcomeEmail"`
}

// AccountMembershipNameUpdate is the complete safe name-only PUT body used by
// the provider after it verifies that role and team assignments are empty.
type AccountMembershipNameUpdate struct {
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
}

type accountMembershipPage struct {
	Results []AccountMembership `json:"results"`
	Paging  struct {
		Next struct {
			After string `json:"after"`
		} `json:"next"`
	} `json:"paging"`
}

// HasRoleOrTeamAssignments reports whether a name-only PUT could omit an
// existing role or team assignment.
func (m AccountMembership) HasRoleOrTeamAssignments() bool {
	return m.RoleID != "" || len(m.RoleIDs) != 0 ||
		(m.PrimaryTeamID != nil && *m.PrimaryTeamID != "") ||
		len(m.SecondaryTeamIDs) != 0
}

func accountMembershipsPath() string { return "/settings/users/2026-03" }

func (c *AccountMembershipClient) GetByID(ctx context.Context, id string) (AccountMembership, error) {
	if id == "" {
		return AccountMembership{}, errors.New("account membership id must not be empty")
	}
	return c.get(ctx, accountMembershipsPath()+"/"+url.PathEscape(id), "account-membership-read")
}

func (c *AccountMembershipClient) GetByEmail(ctx context.Context, email string) (AccountMembership, error) {
	if email == "" {
		return AccountMembership{}, errors.New("account membership email must not be empty")
	}
	query := url.Values{"idProperty": []string{"EMAIL"}}
	return c.get(ctx, accountMembershipsPath()+"/"+url.PathEscape(email)+"?"+query.Encode(), "account-membership-read-by-email")
}

func (c *AccountMembershipClient) get(ctx context.Context, path, operation string) (AccountMembership, error) {
	var out AccountMembership
	if err := c.transport.Do(ctx, Operation{Name: operation, Method: http.MethodGet, Path: path, Replay: ReplaySafe}, nil, &out); err != nil {
		return AccountMembership{}, err
	}
	if out.ID == "" {
		return AccountMembership{}, errors.New("HubSpot account membership response omitted id")
	}
	return out, nil
}

func (c *AccountMembershipClient) Create(ctx context.Context, input AccountMembershipCreate) (AccountMembership, error) {
	if input.Email == "" {
		return AccountMembership{}, errors.New("account membership email must not be empty")
	}
	body, err := json.Marshal(input)
	if err != nil {
		return AccountMembership{}, err
	}
	var out AccountMembership
	err = c.transport.Do(ctx, Operation{
		Name: "account-membership-create", Method: http.MethodPost,
		Path: accountMembershipsPath(), Replay: ReplayNever,
	}, bytes.NewReader(body), &out)
	if err != nil {
		// Preserve a generated Settings ID decoded before a malformed successful
		// response so the resource can verify that exact identity.
		return out, err
	}
	if out.ID == "" {
		return AccountMembership{}, errors.New("HubSpot account membership response omitted id")
	}
	return out, nil
}

func (c *AccountMembershipClient) UpdateNames(ctx context.Context, id string, input AccountMembershipNameUpdate) (AccountMembership, error) {
	if id == "" {
		return AccountMembership{}, errors.New("account membership id must not be empty")
	}
	body, err := json.Marshal(input)
	if err != nil {
		return AccountMembership{}, err
	}
	var out AccountMembership
	if err := c.transport.Do(ctx, Operation{
		Name: "account-membership-update-names", Method: http.MethodPut,
		Path: accountMembershipsPath() + "/" + url.PathEscape(id), Replay: ReplayNever,
	}, bytes.NewReader(body), &out); err != nil {
		return AccountMembership{}, err
	}
	if out.ID == "" {
		return AccountMembership{}, errors.New("HubSpot account membership response omitted id")
	}
	if out.ID != id {
		return AccountMembership{}, errors.New("HubSpot account membership response returned a different id")
	}
	return out, nil
}

func (c *AccountMembershipClient) Delete(ctx context.Context, id string) error {
	if id == "" {
		return errors.New("account membership id must not be empty")
	}
	return c.transport.Do(ctx, Operation{
		Name: "account-membership-delete", Method: http.MethodDelete,
		Path: accountMembershipsPath() + "/" + url.PathEscape(id), Replay: ReplayExplicit,
	}, nil, nil)
}

// WaitForAbsence verifies direct ID and email absence plus eventual collection
// absence. Only HTTP 404 proves direct absence.
func (c *AccountMembershipClient) WaitForAbsence(ctx context.Context, id, email string) error {
	for attempt := 1; attempt <= accountMembershipWaitAttempts; attempt++ {
		idAbsent, err := c.readIsAbsent(ctx, func() error {
			_, err := c.GetByID(ctx, id)
			return err
		})
		if err != nil {
			return err
		}
		emailAbsent, err := c.readIsAbsent(ctx, func() error {
			_, err := c.GetByEmail(ctx, email)
			return err
		})
		if err != nil {
			return err
		}
		memberships, err := c.List(ctx)
		if err != nil {
			return err
		}
		collectionAbsent := true
		for _, membership := range memberships {
			if membership.ID == id || membership.Email == email {
				collectionAbsent = false
				break
			}
		}
		if idAbsent && emailAbsent && collectionAbsent {
			return nil
		}
		if attempt < accountMembershipWaitAttempts {
			if err := c.transport.sleep(ctx, accountMembershipWaitDelay); err != nil {
				return err
			}
		}
	}
	return fmt.Errorf("HubSpot account membership %s remained present after deletion", id)
}

func (c *AccountMembershipClient) readIsAbsent(ctx context.Context, read func() error) (bool, error) {
	err := read()
	if err == nil {
		return false, nil
	}
	var apiError *Error
	if errors.As(err, &apiError) && apiError.Status == http.StatusNotFound {
		return true, nil
	}
	return false, err
}

// List returns all account memberships by following HubSpot's cursor.
func (c *AccountMembershipClient) List(ctx context.Context) ([]AccountMembership, error) {
	results := make([]AccountMembership, 0)
	after := ""
	seen := make(map[string]struct{})
	for {
		query := url.Values{"limit": []string{accountMembershipPageLimit}}
		if after != "" {
			query.Set("after", after)
		}
		var page accountMembershipPage
		if err := c.transport.Do(ctx, Operation{
			Name: "account-membership-list", Method: http.MethodGet,
			Path: accountMembershipsPath() + "?" + query.Encode(), Replay: ReplaySafe,
		}, nil, &page); err != nil {
			return nil, err
		}
		for _, membership := range page.Results {
			if membership.ID == "" {
				return nil, errors.New("HubSpot account membership list response omitted id")
			}
			results = append(results, membership)
		}
		next := page.Paging.Next.After
		if next == "" {
			return results, nil
		}
		if _, exists := seen[next]; exists {
			return nil, errors.New("HubSpot account membership list cursor repeated")
		}
		seen[next] = struct{}{}
		after = next
	}
}
