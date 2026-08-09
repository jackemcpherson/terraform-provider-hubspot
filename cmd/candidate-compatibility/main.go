// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"fmt"
	"os"

	"github.com/jackemcpherson/terraform-provider-hubspot/internal/candidatecompat"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: candidate-compatibility v-prefixed-version cumulative-demo-checkout")
		os.Exit(1)
	}
	if err := candidatecompat.Validate(os.Args[1], os.Args[2]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("Candidate compatibility passed for %s\n", os.Args[1])
}
