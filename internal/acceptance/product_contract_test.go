// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

package acceptance_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/jackemcpherson/terraform-provider-hubspot/internal/acceptance"
	"github.com/jackemcpherson/terraform-provider-hubspot/internal/hubspot"
)

func TestProductArchiveReplayAccepted(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		err  error
		want bool
	}{
		"success":           {err: nil, want: true},
		"not found":         {err: &hubspot.Error{Status: 404}, want: true},
		"wrapped not found": {err: fmt.Errorf("archive replay: %w", &hubspot.Error{Status: 404}), want: true},
		"forbidden":         {err: &hubspot.Error{Status: 403}, want: false},
		"transport failure": {err: errors.New("connection failed"), want: false},
	}
	for name, test := range tests {
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := acceptance.ProductArchiveReplayAccepted(test.err); got != test.want {
				t.Fatalf("ProductArchiveReplayAccepted() = %t, want %t", got, test.want)
			}
		})
	}
}

func TestValidateProductPropertySchemaAcceptsTextBackedRecurrence(t *testing.T) {
	t.Parallel()

	for _, recurrenceType := range []string{"string", "enumeration"} {
		recurrenceType := recurrenceType
		t.Run(recurrenceType, func(t *testing.T) {
			t.Parallel()
			properties := productContractProperties(recurrenceType)
			if err := acceptance.ValidateProductPropertySchema(properties); err != nil {
				t.Fatalf("ValidateProductPropertySchema() error = %v", err)
			}
		})
	}
}

func TestValidateProductPropertySchemaRejectsIncompatibleRecurrence(t *testing.T) {
	t.Parallel()

	err := acceptance.ValidateProductPropertySchema(productContractProperties("number"))
	if err == nil || !strings.Contains(err.Error(), "hs_recurring_billing_period") {
		t.Fatalf("ValidateProductPropertySchema() error = %v", err)
	}
}

func TestValidateProductPropertySchemaRejectsIncompatibleManagedScalar(t *testing.T) {
	t.Parallel()

	properties := productContractProperties("string")
	properties[productContractPropertyIndex(t, properties, "price")].Type = "string"
	err := acceptance.ValidateProductPropertySchema(properties)
	if err == nil || !strings.Contains(err.Error(), "price") {
		t.Fatalf("ValidateProductPropertySchema() error = %v", err)
	}
}

func TestValidateProductPropertySchemaRequiresManagedProperties(t *testing.T) {
	t.Parallel()

	properties := productContractProperties("string")
	properties = removeProductContractProperty(t, properties, "description")
	err := acceptance.ValidateProductPropertySchema(properties)
	if err == nil || !strings.Contains(err.Error(), "description") {
		t.Fatalf("ValidateProductPropertySchema() error = %v", err)
	}
}

func TestValidateProductPropertySchemaRequiresRootFolderProperty(t *testing.T) {
	t.Parallel()

	properties := productContractProperties("string")
	properties = removeProductContractProperty(t, properties, "hs_folder")
	err := acceptance.ValidateProductPropertySchema(properties)
	if err == nil || !strings.Contains(err.Error(), "hs_folder") {
		t.Fatalf("ValidateProductPropertySchema() error = %v", err)
	}
}

func productContractProperties(recurrenceType string) []hubspot.ProductProperty {
	return []hubspot.ProductProperty{
		{Name: "name", Type: "string"},
		{Name: "hs_sku", Type: "string"},
		{Name: "description", Type: "string"},
		{Name: "price", Type: "number"},
		{Name: "hs_cost_of_goods_sold", Type: "number"},
		{Name: "hs_recurring_billing_period", Type: recurrenceType},
		{Name: "hs_folder", Type: "string"},
	}
}

func productContractPropertyIndex(t *testing.T, properties []hubspot.ProductProperty, name string) int {
	t.Helper()
	for index, property := range properties {
		if property.Name == name {
			return index
		}
	}
	t.Fatalf("test property %q is missing", name)
	return -1
}

func removeProductContractProperty(
	t *testing.T,
	properties []hubspot.ProductProperty,
	name string,
) []hubspot.ProductProperty {
	t.Helper()
	index := productContractPropertyIndex(t, properties, name)
	return append(properties[:index], properties[index+1:]...)
}
