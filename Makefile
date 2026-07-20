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
SYFT_VERSION := v1.33.0
STATICCHECK_BIN := $(TOOLS_BIN)/staticcheck

.PHONY: tools check check-go check-docs check-release-tools check-workflows engine-smoke docs test test-race fuzz-seeds fmt release-preflight release-snapshot one-portal-free-lifecycle

tools:
	@command -v go >/dev/null || { echo "go $(GO_VERSION) required; install tools before running checks"; exit 1; }
	@go version | grep -F "go$(GO_VERSION)" >/dev/null || { echo "exact Go $(GO_VERSION) required"; exit 1; }
	@./scripts/install-engines.sh "$(TOOLS_BIN)" "$(TOFU_CURRENT_VERSION)" "$(TERRAFORM_CURRENT_VERSION)"
	@tofu version | grep -F "OpenTofu v$(TOFU_CURRENT_VERSION)" >/dev/null || { echo "exact OpenTofu $(TOFU_CURRENT_VERSION) required"; exit 1; }
	@terraform version | grep -F "Terraform v$(TERRAFORM_CURRENT_VERSION)" >/dev/null || { echo "exact Terraform $(TERRAFORM_CURRENT_VERSION) required"; exit 1; }
	@go install honnef.co/go/tools/cmd/staticcheck@$(STATICCHECK_VERSION)
	@go install github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs@$(TFPLUGINDOCS_VERSION)
	@go install github.com/goreleaser/goreleaser/v2@$(GORELEASER_VERSION)
	@go install github.com/anchore/syft/cmd/syft@$(SYFT_VERSION)
	@"$(STATICCHECK_BIN)" -version | grep -F '0.6.1' >/dev/null
	@test -x "$(TOOLS_BIN)/tfplugindocs"
	@"$(TOOLS_BIN)/goreleaser" --version | grep -F 'v2.17.0' >/dev/null
	@go version -m "$(TOOLS_BIN)/syft" | grep -E 'github.com/anchore/syft[[:space:]]+v1.33.0' >/dev/null

check: check-go check-docs check-release-tools check-workflows engine-smoke

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
	@go test -tags=acceptance ./internal/acceptance -run '^Test.*AcceptanceConfigurationSyntax$$'
	@go test -race ./...
	@go mod tidy -diff
	@go mod verify
	@go test -run=^$ -fuzz=Fuzz -fuzztime=1x ./internal/provider

check-docs:
	@test -f docs/index.md || { echo "generated provider docs missing"; exit 1; }
	@test -f terraform-registry-manifest.json || { echo "protocol manifest missing"; exit 1; }
	@test -f registry-addresses.txt || { echo "registry address inventory missing"; exit 1; }
	@test "$$(wc -l < registry-addresses.txt | tr -d ' ')" = 2
	@./scripts/verify-registry-manifest.sh terraform-registry-manifest.json
	@./scripts/check-generated-docs.sh

check-release-tools:
	@"$(TOOLS_BIN)/goreleaser" check
	@"$(TOOLS_BIN)/goreleaser" healthcheck

check-workflows:
	@./scripts/one-portal-free-lifecycle_test.sh
	@./scripts/compare-release-builds_test.sh
	@./scripts/verify-registry-checksums_test.sh
	@./scripts/verify-registry-manifest_test.sh
	@./scripts/check-workflows.sh

one-portal-free-lifecycle:
	@./scripts/one-portal-free-lifecycle.sh

release-preflight:
	@./scripts/registry-release-preflight.sh "$(or $(VERSION),v0.0.0-preflight)"

release-snapshot:
	@"$(TOOLS_BIN)/goreleaser" release --snapshot --clean --skip=sign

docs:
	@test -x "$(TOOLS_BIN)/tfplugindocs" || { echo "tfplugindocs $(TFPLUGINDOCS_VERSION) required; run make tools"; exit 1; }
	@"$(TOOLS_BIN)/tfplugindocs" generate --provider-name hubspot

test:
	@go test ./...

test-race:
	@go test -race ./...

fuzz-seeds:
	@go test -run=^$ -fuzz=Fuzz -fuzztime=1x ./internal/provider

fmt:
	@gofmt -w $$(find . -name '*.go' -not -path './.git/*')
