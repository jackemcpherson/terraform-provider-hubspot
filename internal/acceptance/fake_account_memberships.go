// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

//go:build acceptance

package acceptance

import (
	"net/http"
	"sort"
	"strconv"

	"github.com/jackemcpherson/terraform-provider-hubspot/internal/hubspot"
)

type fakeAccountMembership struct {
	membership     hubspot.AccountMembership
	active         bool
	activated      bool
	lastWelcome    bool
	createCount    int
	updateCount    int
	updateAttempts int
	deleteCount    int
	collectionLag  int
}

type fakeAccountMembershipReadOverride struct {
	identity   string
	idProperty string
	membership hubspot.AccountMembership
}

type AccountMembershipFault string

const (
	AccountMembershipFaultCreateUnknown            AccountMembershipFault = "create_unknown"
	AccountMembershipFaultCreateKnown              AccountMembershipFault = "create_known"
	AccountMembershipFaultCreateKnownEmailMismatch AccountMembershipFault = "create_known_email_mismatch"
	AccountMembershipFaultCreateKnownNameMismatch  AccountMembershipFault = "create_known_name_mismatch"
	AccountMembershipFaultCreateKnownLastMismatch  AccountMembershipFault = "create_known_last_mismatch"
)

func (f *FakeHubSpot) handleAccountMemberships(response http.ResponseWriter, request *http.Request, rest []string) {
	switch len(rest) {
	case 0:
		f.handleAccountMembershipCollection(response, request)
	case 1:
		f.handleAccountMembershipItem(response, request, rest[0])
	default:
		writeFakeError(response, http.StatusNotFound, "OBJECT_NOT_FOUND", "", "No account membership route matched this request.")
	}
}

func (f *FakeHubSpot) handleAccountMembershipCollection(response http.ResponseWriter, request *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch request.Method {
	case http.MethodGet:
		ids := make([]string, 0, len(f.accountMemberships))
		for id, membership := range f.accountMemberships {
			if membership.active || membership.collectionLag > 0 {
				ids = append(ids, id)
			}
		}
		sort.Strings(ids)
		results := make([]hubspot.AccountMembership, 0, len(ids))
		for _, id := range ids {
			results = append(results, f.accountMemberships[id].membership)
			if f.accountMemberships[id].collectionLag > 0 {
				f.accountMemberships[id].collectionLag--
			}
		}
		writeFakeJSON(response, http.StatusOK, map[string]any{"results": results})
	case http.MethodPost:
		var input hubspot.AccountMembershipCreate
		if !decodeFakeBody(response, request, &input) {
			return
		}
		if input.Email == "" {
			writeFakeError(response, http.StatusBadRequest, "VALIDATION_ERROR", "", "An email is required.")
			return
		}
		if existingID := f.accountMembershipIDsByEmail[input.Email]; existingID != "" {
			existing := f.accountMemberships[existingID]
			if existing.active {
				writeFakeError(response, http.StatusConflict, "VALIDATION_ERROR", "USER_ALREADY_EXISTS", "An account membership already uses this email.")
				return
			}
			existing.membership.FirstName = input.FirstName
			existing.membership.LastName = input.LastName
			existing.membership.SuperAdmin = false
			existing.membership.RoleID = ""
			existing.membership.RoleIDs = []string{}
			existing.membership.PrimaryTeamID = nil
			existing.membership.SecondaryTeamIDs = []string{}
			existing.active = true
			existing.activated = true
			existing.lastWelcome = input.SendWelcomeEmail
			existing.createCount++
			writeFakeAccountMembership(response, http.StatusCreated, existing)
			return
		}
		f.nextAccountMembershipID++
		id := strconv.Itoa(20_000 + f.nextAccountMembershipID)
		membership := &fakeAccountMembership{
			membership: hubspot.AccountMembership{
				ID: id, Email: input.Email, FirstName: input.FirstName, LastName: input.LastName,
				RoleIDs: []string{}, SecondaryTeamIDs: []string{},
			},
			active: true, activated: true, lastWelcome: input.SendWelcomeEmail, createCount: 1,
		}
		f.accountMemberships[id] = membership
		f.accountMembershipIDsByEmail[input.Email] = id
		switch f.nextAccountMembershipFault {
		case AccountMembershipFaultCreateUnknown:
			f.nextAccountMembershipFault = ""
			dropFakeConnection(response)
			return
		case AccountMembershipFaultCreateKnown,
			AccountMembershipFaultCreateKnownEmailMismatch,
			AccountMembershipFaultCreateKnownNameMismatch,
			AccountMembershipFaultCreateKnownLastMismatch:
			if f.nextAccountMembershipFault == AccountMembershipFaultCreateKnownEmailMismatch {
				membership.membership.Email = "mismatched@example.invalid"
			}
			if f.nextAccountMembershipFault == AccountMembershipFaultCreateKnownNameMismatch {
				membership.membership.FirstName = "Mismatched"
			}
			if f.nextAccountMembershipFault == AccountMembershipFaultCreateKnownLastMismatch {
				membership.membership.LastName = "Mismatched"
			}
			f.nextAccountMembershipFault = ""
			writeFakeJSON(response, http.StatusCreated, map[string]any{"id": id, "superAdmin": "invalid"})
			return
		}
		writeFakeAccountMembership(response, http.StatusCreated, membership)
	default:
		writeFakeError(response, http.StatusMethodNotAllowed, "VALIDATION_ERROR", "", "Unsupported method.")
	}
}

func (f *FakeHubSpot) handleAccountMembershipItem(response http.ResponseWriter, request *http.Request, identity string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	idProperty := request.URL.Query().Get("idProperty")
	if request.Method == http.MethodGet && f.nextAccountMembershipRead != nil &&
		f.nextAccountMembershipRead.identity == identity && f.nextAccountMembershipRead.idProperty == idProperty {
		override := &fakeAccountMembership{membership: f.nextAccountMembershipRead.membership, active: true}
		f.nextAccountMembershipRead = nil
		writeFakeAccountMembership(response, http.StatusOK, override)
		return
	}
	membership := f.accountMembershipByIdentity(identity, idProperty)
	if membership == nil || !membership.active {
		writeFakeError(response, http.StatusNotFound, "OBJECT_NOT_FOUND", "", "No account membership matched this identity.")
		return
	}
	switch request.Method {
	case http.MethodGet:
		writeFakeAccountMembership(response, http.StatusOK, membership)
	case http.MethodPut:
		membership.updateAttempts++
		if !membership.activated {
			writeFakeError(response, http.StatusBadRequest, "VALIDATION_ERROR", "USER_NOT_ON_ANY_HUBS", "USER_NOT_ON_ANY_HUBS")
			return
		}
		var input hubspot.AccountMembershipNameUpdate
		if !decodeFakeBody(response, request, &input) {
			return
		}
		membership.membership.FirstName = input.FirstName
		membership.membership.LastName = input.LastName
		membership.updateCount++
		writeFakeAccountMembership(response, http.StatusOK, membership)
	case http.MethodDelete:
		membership.active = false
		membership.collectionLag = f.nextMembershipCollectionLag
		f.nextMembershipCollectionLag = 0
		membership.deleteCount++
		response.WriteHeader(http.StatusNoContent)
	default:
		writeFakeError(response, http.StatusMethodNotAllowed, "VALIDATION_ERROR", "", "Unsupported method.")
	}
}

func (f *FakeHubSpot) accountMembershipByIdentity(identity, idProperty string) *fakeAccountMembership {
	if idProperty == "EMAIL" {
		return f.accountMemberships[f.accountMembershipIDsByEmail[identity]]
	}
	return f.accountMemberships[identity]
}

func writeFakeAccountMembership(response http.ResponseWriter, status int, membership *fakeAccountMembership) {
	writeFakeJSON(response, status, map[string]any{
		"id": membership.membership.ID, "email": membership.membership.Email,
		"firstName": membership.membership.FirstName, "lastName": membership.membership.LastName,
		"superAdmin": membership.membership.SuperAdmin, "roleId": membership.membership.RoleID,
		"roleIds": membership.membership.RoleIDs, "primaryTeamId": membership.membership.PrimaryTeamID,
		"secondaryTeamIds": membership.membership.SecondaryTeamIDs,
		"sendWelcomeEmail": membership.lastWelcome, "seatNames": []string{"core"},
	})
}

func (f *FakeHubSpot) AccountMembershipWriteCounts(id string) (int, int, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	membership := f.accountMemberships[id]
	if membership == nil {
		return 0, 0, 0
	}
	return membership.createCount, membership.updateCount, membership.deleteCount
}

func (f *FakeHubSpot) AccountMembershipUpdateAttempts(id string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	if membership := f.accountMemberships[id]; membership != nil {
		return membership.updateAttempts
	}
	return 0
}

func (f *FakeHubSpot) AccountMembershipSnapshot(id string) (hubspot.AccountMembership, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	membership := f.accountMemberships[id]
	if membership == nil || !membership.active {
		return hubspot.AccountMembership{}, false
	}
	return membership.membership, true
}

func (f *FakeHubSpot) OverrideNextAccountMembershipIDRead(id string, membership hubspot.AccountMembership) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextAccountMembershipRead = &fakeAccountMembershipReadOverride{identity: id, membership: membership}
}

func (f *FakeHubSpot) OverrideNextAccountMembershipEmailRead(email string, membership hubspot.AccountMembership) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextAccountMembershipRead = &fakeAccountMembershipReadOverride{
		identity: email, idProperty: "EMAIL", membership: membership,
	}
}

func (f *FakeHubSpot) DriftAccountMembershipNames(id, firstName, lastName string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	membership := f.accountMemberships[id]
	if membership == nil || !membership.active {
		return false
	}
	membership.membership.FirstName = firstName
	membership.membership.LastName = lastName
	return true
}

func (f *FakeHubSpot) SetAccountMembershipAssignments(id string, assigned bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if membership := f.accountMemberships[id]; membership != nil {
		if assigned {
			membership.membership.RoleIDs = []string{"role-1"}
			return
		}
		membership.membership.RoleIDs = []string{}
		membership.membership.RoleID = ""
		membership.membership.PrimaryTeamID = nil
		membership.membership.SecondaryTeamIDs = []string{}
	}
}

func (f *FakeHubSpot) SetAccountMembershipTeamAssignment(id string, assigned bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if membership := f.accountMemberships[id]; membership != nil {
		if assigned {
			teamID := "team-1"
			membership.membership.PrimaryTeamID = &teamID
			membership.membership.SecondaryTeamIDs = []string{"team-2"}
			return
		}
		membership.membership.PrimaryTeamID = nil
		membership.membership.SecondaryTeamIDs = []string{}
	}
}

func (f *FakeHubSpot) SetAccountMembershipActivated(id string, activated bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if membership := f.accountMemberships[id]; membership != nil {
		membership.activated = activated
	}
}

func (f *FakeHubSpot) SetAccountMembershipSuperAdmin(id string, superAdmin bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if membership := f.accountMemberships[id]; membership != nil {
		membership.membership.SuperAdmin = superAdmin
	}
}

func (f *FakeHubSpot) ActiveAccountMembershipCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	count := 0
	for _, membership := range f.accountMemberships {
		if membership.active {
			count++
		}
	}
	return count
}

func (f *FakeHubSpot) DisappearAccountMembership(id string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	membership := f.accountMemberships[id]
	if membership == nil || !membership.active {
		return false
	}
	membership.active = false
	return true
}

func (f *FakeHubSpot) LastAccountMembershipWelcomeChoice(email string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if membership := f.accountMemberships[f.accountMembershipIDsByEmail[email]]; membership != nil {
		return membership.lastWelcome
	}
	return false
}

func (f *FakeHubSpot) FailNextAccountMembershipOperation(fault AccountMembershipFault) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextAccountMembershipFault = fault
}

func (f *FakeHubSpot) LagNextAccountMembershipCollectionAfterDelete(reads int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextMembershipCollectionLag = reads
}

func (f *FakeHubSpot) ActiveAccountMembershipID(email string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := f.accountMembershipIDsByEmail[email]
	if membership := f.accountMemberships[id]; membership != nil && membership.active {
		return id
	}
	return ""
}
