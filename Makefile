SHELL := /bin/sh
export GOTOOLCHAIN := local
TOOLS_BIN := $(shell go env GOPATH 2>/dev/null)/bin
export PATH := $(TOOLS_BIN):$(PATH)

GO_VERSION := 1.26.5
TOFU_MIN_VERSION := 1.8.8
TOFU_CURRENT_VERSION := 1.12.3
TERRAFORM_MIN_VERSION := 1.8.5
TERRAFORM_CURRENT_VERSION := 1.15.8
TFPLUGINDOCS_VERSION := v0.25.0
GORELEASER_VERSION := v2.17.0
STATICCHECK_VERSION := v0.6.1
ZIZMOR_VERSION := 1.27.0
STATICCHECK_BIN := $(TOOLS_BIN)/staticcheck

.PHONY: tools check check-go check-docs check-release-tools check-security check-workflows engine-smoke docs docs-portal docs-portal-update docs-portal-serve test test-race test-hermetic fuzz-seeds fmt release-preflight release-snapshot one-portal-free-lifecycle

tools:
	@command -v go >/dev/null || { echo "go $(GO_VERSION) required; install tools before running checks"; exit 1; }
	@go version | grep -F "go$(GO_VERSION)" >/dev/null || { echo "exact Go $(GO_VERSION) required"; exit 1; }
	@./scripts/install-engines.sh "$(TOOLS_BIN)" "$(TOFU_CURRENT_VERSION)" "$(TERRAFORM_CURRENT_VERSION)"
	@tofu version | grep -F "OpenTofu v$(TOFU_CURRENT_VERSION)" >/dev/null || { echo "exact OpenTofu $(TOFU_CURRENT_VERSION) required"; exit 1; }
	@terraform version | grep -F "Terraform v$(TERRAFORM_CURRENT_VERSION)" >/dev/null || { echo "exact Terraform $(TERRAFORM_CURRENT_VERSION) required"; exit 1; }
	@go install honnef.co/go/tools/cmd/staticcheck@$(STATICCHECK_VERSION)
	@go install github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs@$(TFPLUGINDOCS_VERSION)
	@go install github.com/goreleaser/goreleaser/v2@$(GORELEASER_VERSION)
	@./scripts/install-zizmor.sh "$(TOOLS_BIN)" "$(ZIZMOR_VERSION)"
	@"$(STATICCHECK_BIN)" -version | grep -F '0.6.1' >/dev/null
	@test -x "$(TOOLS_BIN)/tfplugindocs"
	@"$(TOOLS_BIN)/goreleaser" --version | grep -F 'v2.17.0' >/dev/null
	@"$(TOOLS_BIN)/zizmor" --version | grep -F 'zizmor $(ZIZMOR_VERSION)' >/dev/null

check: check-go check-docs check-release-tools check-workflows check-security engine-smoke

engine-smoke:
	@./scripts/engine-smoke.sh

check-go:
	@command -v go >/dev/null || { echo "go $(GO_VERSION) required"; exit 1; }
	@go version | grep -F "go$(GO_VERSION)" >/dev/null || { echo "exact Go $(GO_VERSION) required"; exit 1; }
	@test -z "$$(gofmt -l .)" || { echo "Go files require formatting"; gofmt -l .; exit 1; }
	@go vet ./...
	@test -x "$(STATICCHECK_BIN)" || { echo "staticcheck $(STATICCHECK_VERSION) required; run make tools"; exit 1; }
	@"$(STATICCHECK_BIN)" ./...
	@go test ./...
	@go test -tags=acceptance ./cmd/northstar-lifecycle
	@go test -tags=acceptance ./internal/acceptance -run '^Test.*AcceptanceConfigurationSyntax$$'
	@go test -race ./...
	@go mod tidy -diff
	@go mod verify
	@go test -run=^$ -fuzz=Fuzz -fuzztime=1x ./internal/provider

check-docs:
	@./scripts/check-license.sh
	@test -f docs/index.md || { echo "generated provider docs missing"; exit 1; }
	@test -f terraform-registry-manifest.json || { echo "protocol manifest missing"; exit 1; }
	@test -f registry-addresses.txt || { echo "registry address inventory missing"; exit 1; }
	@test "$$(wc -l < registry-addresses.txt | tr -d ' ')" = 2
	@./scripts/verify-registry-manifest.sh terraform-registry-manifest.json
	@./scripts/check-generated-docs.sh
	@$(MAKE) docs-portal

check-release-tools:
	@"$(TOOLS_BIN)/goreleaser" check
	@"$(TOOLS_BIN)/goreleaser" healthcheck

check-workflows:
	@./scripts/one-portal-free-lifecycle_test.sh
	@./scripts/acceptance-cleanup_test.sh
	@./scripts/released-provider-journey_test.sh
	@./scripts/released-northstar-journey_test.sh
	@./scripts/verify-released-provider_test.sh
	@./scripts/observe-release_test.sh
	@./scripts/compare-release-builds_test.sh
	@./scripts/verify-registry-checksums_test.sh
	@./scripts/verify-registry-manifest_test.sh
	@./scripts/check-workflows.sh

check-security:
	@test -x "$(TOOLS_BIN)/zizmor" || { echo "zizmor $(ZIZMOR_VERSION) is required; run make tools"; exit 1; }
	@"$(TOOLS_BIN)/zizmor" --version | grep -F 'zizmor $(ZIZMOR_VERSION)' >/dev/null || { echo "exact zizmor $(ZIZMOR_VERSION) required; run make tools"; exit 1; }
	@go run golang.org/x/vuln/cmd/govulncheck@v1.1.4 ./...
	@go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12
	@"$(TOOLS_BIN)/zizmor" .

one-portal-free-lifecycle:
	@./scripts/one-portal-free-lifecycle.sh

release-preflight:
	@./scripts/registry-release-preflight.sh "$(or $(VERSION),v0.0.0-preflight)"

release-snapshot:
	@"$(TOOLS_BIN)/goreleaser" release --snapshot --clean --skip=sign

docs:
	@test -x "$(TOOLS_BIN)/tfplugindocs" || { echo "tfplugindocs $(TFPLUGINDOCS_VERSION) required; run make tools"; exit 1; }
	@"$(TOOLS_BIN)/tfplugindocs" generate --provider-name hubspot

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
