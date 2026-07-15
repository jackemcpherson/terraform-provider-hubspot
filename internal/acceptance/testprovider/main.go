// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

// Command testprovider serves the development-only provider surface for offline
// acceptance tests. It is never used to build release artifacts.
package main

import (
	"context"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/jackemcpherson/terraform-provider-hubspot/internal/provider"
)

func main() {
	if err := providerserver.Serve(context.Background(), provider.NewDevelopment("test"), providerserver.ServeOpts{
		Address: "registry.terraform.io/jackemcpherson/hubspot",
	}); err != nil {
		log.Fatal(err)
	}
}
