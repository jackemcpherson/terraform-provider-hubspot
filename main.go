// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/jackemcpherson/terraform-provider-hubspot/internal/provider"
)

var version = "dev"

func main() {
	if err := providerserver.Serve(context.Background(), provider.New(version), providerserver.ServeOpts{
		Address: "registry.terraform.io/jackemcpherson/hubspot",
	}); err != nil {
		log.Fatal(err)
	}
}
