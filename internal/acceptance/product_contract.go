// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

package acceptance

import (
	"fmt"
	"strings"

	"github.com/jackemcpherson/terraform-provider-hubspot/internal/hubspot"
)

// ValidateProductPropertySchema checks the runtime properties needed by the
// frozen Product resource contract. Recurrence remains text-backed across the
// string and enumeration metadata forms used by HubSpot portals.
func ValidateProductPropertySchema(properties []hubspot.ProductProperty) error {
	seen := make(map[string]hubspot.ProductProperty, len(properties))
	for _, property := range properties {
		seen[property.Name] = property
	}

	requirements := []struct {
		name         string
		allowedTypes []string
	}{
		{name: "name", allowedTypes: []string{"string"}},
		{name: "hs_sku", allowedTypes: []string{"string"}},
		{name: "description", allowedTypes: []string{"string"}},
		{name: "price", allowedTypes: []string{"number"}},
		{name: "hs_cost_of_goods_sold", allowedTypes: []string{"number"}},
		{name: "hs_recurring_billing_period", allowedTypes: []string{"string", "enumeration"}},
	}
	for _, requirement := range requirements {
		property, ok := seen[requirement.name]
		if !ok {
			return fmt.Errorf("product runtime property schema omitted %s", requirement.name)
		}
		if !productPropertyTypeAllowed(property.Type, requirement.allowedTypes) {
			return fmt.Errorf(
				"product runtime property %s has unsupported type %q; expected %s",
				requirement.name,
				property.Type,
				strings.Join(requirement.allowedTypes, " or "),
			)
		}
	}
	if _, ok := seen["hs_folder"]; !ok {
		return fmt.Errorf("product runtime property schema omitted hs_folder")
	}

	return nil
}

func productPropertyTypeAllowed(actual string, allowed []string) bool {
	for _, propertyType := range allowed {
		if actual == propertyType {
			return true
		}
	}
	return false
}
