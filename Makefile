SHELL := /bin/sh
export GOTOOLCHAIN := local
TOOLS_BIN := $(shell go env GOPATH 2>/dev/null)/bin
export PATH := $(TOOLS_BIN):$(PATH)

GO_VERSION := 1.26.6
TOFU_CURRENT_VERSION := 1.12.3
TERRAFORM_CURRENT_VERSION := 1.15.8
TFPLUGINDOCS_VERSION := v0.25.0
GORELEASER_VERSION := v2.17.0
STATICCHECK_VERSION := v0.6.1
ZIZMOR_VERSION := 1.27.0
STATICCHECK_BIN := $(TOOLS_BIN)/staticcheck

.PHONY: tools maintenance-tools check check-go check-docs check-workflows engine-smoke maintenance check-security check-release product-contract-preflight contact-segment-contract-preflight northstar-maintenance docs docs-portal docs-portal-update docs-portal-serve test test-race test-hermetic fuzz-seeds fmt

tools:
	@command -v go >/dev/null || { echo "go $(GO_VERSION) required; install tools before running checks"; exit 1; }
	@go version | grep -F "go$(GO_VERSION)" >/dev/null || { echo "exact Go $(GO_VERSION) required"; exit 1; }
	@./scripts/install-engines.sh "$(TOOLS_BIN)" "$(TOFU_CURRENT_VERSION)" "$(TERRAFORM_CURRENT_VERSION)"
	@go install honnef.co/go/tools/cmd/staticcheck@$(STATICCHECK_VERSION)
	@go install github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs@$(TFPLUGINDOCS_VERSION)
	@tofu version | grep -F "OpenTofu v$(TOFU_CURRENT_VERSION)" >/dev/null
	@terraform version | grep -F "Terraform v$(TERRAFORM_CURRENT_VERSION)" >/dev/null
	@"$(STATICCHECK_BIN)" -version | grep -F '0.6.1' >/dev/null
	@test -x "$(TOOLS_BIN)/tfplugindocs"

maintenance-tools: tools
	@go install github.com/goreleaser/goreleaser/v2@$(GORELEASER_VERSION)
	@./scripts/install-zizmor.sh "$(TOOLS_BIN)" "$(ZIZMOR_VERSION)"
	@"$(TOOLS_BIN)/goreleaser" --version | grep -F 'v2.17.0' >/dev/null
	@"$(TOOLS_BIN)/zizmor" --version | grep -F 'zizmor $(ZIZMOR_VERSION)' >/dev/null

check: check-go check-docs check-workflows engine-smoke

check-go:
	@command -v go >/dev/null || { echo "go $(GO_VERSION) required"; exit 1; }
	@go version | grep -F "go$(GO_VERSION)" >/dev/null || { echo "exact Go $(GO_VERSION) required"; exit 1; }
	@test -z "$$(gofmt -l .)" || { echo "Go files require formatting"; gofmt -l .; exit 1; }
	@go vet ./...
	@test -x "$(STATICCHECK_BIN)" || { echo "staticcheck $(STATICCHECK_VERSION) required; run make tools"; exit 1; }
	@"$(STATICCHECK_BIN)" ./...
	@go test ./...
	@go test -tags=acceptance ./cmd/northstar-lifecycle
	@go mod tidy -diff
	@go mod verify

check-docs:
	@./scripts/check-license.sh
	@./scripts/check-generated-docs.sh

check-workflows:
	@go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12
	@./scripts/check-workflows.sh
	@./scripts/release-preflight_test.sh
	@./scripts/northstar-maintenance_test.sh
	@./scripts/acceptance-cleanup_test.sh

engine-smoke:
	@./scripts/engine-smoke.sh

maintenance: check-security check-release product-contract-preflight contact-segment-contract-preflight northstar-maintenance

check-security:
	@test -x "$(TOOLS_BIN)/zizmor" || { echo "zizmor $(ZIZMOR_VERSION) is required; run make maintenance-tools"; exit 1; }
	@go run golang.org/x/vuln/cmd/govulncheck@v1.1.4 ./...
	@"$(TOOLS_BIN)/zizmor" .

check-release:
	@test -x "$(TOOLS_BIN)/goreleaser" || { echo "goreleaser $(GORELEASER_VERSION) is required; run make maintenance-tools"; exit 1; }
	@"$(TOOLS_BIN)/goreleaser" check
	@"$(TOOLS_BIN)/goreleaser" healthcheck
	@./scripts/verify-registry-manifest.sh terraform-registry-manifest.json
	@./scripts/verify-release-assets_test.sh
	@./scripts/verify-registry-checksums_test.sh
	@./scripts/verify-registry-manifest_test.sh

product-contract-preflight:
	@HUBSPOT_ACCEPTANCE=1 HUBSPOT_ACCEPTANCE_PREFIX=tf_acc_products_contract_ go test -tags=acceptance ./internal/acceptance -run '^TestAcc_product_definitions_ContractPreflight$$' -count=1 -timeout=5m

contact-segment-contract-preflight:
	@HUBSPOT_ACCEPTANCE=1 HUBSPOT_ACCEPTANCE_PREFIX=tf_acc_contact_segments_contract_ go test -tags=acceptance ./internal/acceptance -run '^TestAcc_contact_segments_ContractPreflight$$' -count=1 -timeout=15m

northstar-maintenance:
	@./scripts/northstar-maintenance.sh

docs:
	@test -x "$(TOOLS_BIN)/tfplugindocs" || { echo "tfplugindocs $(TFPLUGINDOCS_VERSION) required; run make tools"; exit 1; }
	@GOFLAGS="-ldflags=-X=main.providerAddress=registry.terraform.io/hashicorp/hubspot" "$(TOOLS_BIN)/tfplugindocs" generate --provider-name hubspot

docs-portal:
	@go run ./cmd/docs-portal
	@go run ./cmd/docs-portal-serve --dir dist/docs-portal --smoke

docs-portal-update:
	@DOCS_PORTAL_UPDATE=1 go run ./cmd/docs-portal
	@go run ./cmd/docs-portal-serve --dir dist/docs-portal --smoke

docs-portal-serve: docs-portal
	@go run ./cmd/docs-portal-serve --dir dist/docs-portal

test:
	@go test ./...

test-race:
	@go test -race ./...

# test-hermetic runs the provider lifecycle shards against the in-process
# FakeHubSpot fake instead of a real HubSpot portal, plus the fake's own
# direct fidelity tests. It requires no HubSpot credentials and needs
# whichever of tofu/terraform are installed to be exercised.
test-hermetic:
	@go test -tags=acceptance ./internal/acceptance -run '^(TestHermetic|TestFakeHubSpot)' -v

fuzz-seeds:
	@go test -run=^$ -fuzz=Fuzz -fuzztime=1x ./internal/provider

fmt:
	@gofmt -w $$(find . -name '*.go' -not -path './.git/*')
