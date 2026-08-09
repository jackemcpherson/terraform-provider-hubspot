// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"fmt"
	"os"

	"github.com/jackemcpherson/terraform-provider-hubspot/internal/provenance"
)

func main() {
	if len(os.Args) != 3 {
		fatal(fmt.Errorf("usage: validate-checkout CHECKOUT EXPECTED_COMMIT"))
	}
	if _, err := provenance.ValidateCheckout(os.Args[1], os.Args[2]); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
