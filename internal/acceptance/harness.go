// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

// Package acceptance runs provider lifecycle tests through pinned Terraform and
// OpenTofu command-line interfaces.
package acceptance

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

type Engine string

const (
	OpenTofu  Engine = "tofu"
	Terraform Engine = "terraform"
)

type Shard string

const (
	FreeProperties      Shard = "free_properties"
	DealPipelines       Shard = "deal_pipelines"
	TicketPipelines     Shard = "ticket_pipelines"
	CustomSchemas       Shard = "custom_schemas"
	SensitiveProperties Shard = "sensitive_properties"
	CustomPipelines     Shard = "custom_pipelines"
)

type Warning string

const (
	PropertyTypeTransition      Warning = "Property type transition"
	PropertyOptionValuesChanged Warning = "Property option values changed"
)

type Failure string

const (
	PropertyGroupHasActiveProperties Failure = "HubSpot HTTP 400 (VALIDATION_ERROR) [PropertyGroupError.GROUP_WITH_ACTIVE_PROPERTIES]"
	PipelineStageInUse               Failure = "HubSpot HTTP 400 (VALIDATION_ERROR) [PipelineError.STAGE_ID_IN_USE]"
)

type Options struct {
	Engine       Engine
	Shard        Shard
	Prefix       string
	LedgerPath   string
	ProbeBaseURL string
}

type Session struct {
	t          testing.TB
	engine     Engine
	workDir    string
	env        []string
	ledgerPath string
	ledgerID   string
	shard      Shard
	prefix     string
	probeURL   string
	registered bool
	config     string
}

var acceptancePrefix = regexp.MustCompile(`^tf_acc_[A-Za-z0-9_]+_$`)
var (
	engineErrorTitle = regexp.MustCompile(`(?m)^Error: ([A-Za-z][A-Za-z ]+)$`)
	hubSpotStatus    = regexp.MustCompile(`HubSpot returned HTTP ([0-9]{3})(?: \(([A-Za-z0-9_.-]+)\))?(?:[\t\r\n ]+\[([A-Za-z0-9_.-]+)\])?`)
	inconsistentPath = regexp.MustCompile(`unexpected new value for \.([A-Za-z0-9_.]+):`)
	stateValuePath   = regexp.MustCompile(`\.([a-z][a-z0-9_]*): was cty\.`)
	planChangePath   = regexp.MustCompile(`(?m)^\s*[~+-]\s+([a-z][a-z0-9_]*)\s+=`)
)

func Run(t testing.TB, options Options, scenario func(*Session)) {
	t.Helper()

	if options.Engine != OpenTofu && options.Engine != Terraform {
		t.Fatalf("unsupported acceptance engine %q", options.Engine)
	}
	if !validShard(options.Shard) {
		t.Fatalf("unsupported acceptance shard %q", options.Shard)
	}
	if !acceptancePrefix.MatchString(options.Prefix) {
		t.Fatal("acceptance prefix must start with tf_acc_ and end with an underscore")
	}
	if options.LedgerPath == "" {
		t.Fatal("acceptance cleanup ledger path is required")
	}
	if _, err := exec.LookPath(string(options.Engine)); err != nil {
		t.Fatalf("pinned %s executable is required", options.Engine)
	}

	root, err := repositoryRoot()
	if err != nil {
		t.Fatal(err)
	}
	temp := t.TempDir()
	binDir := filepath.Join(temp, "bin")
	if err := os.Mkdir(binDir, 0o700); err != nil {
		t.Fatalf("create provider directory: %v", err)
	}
	providerBinary := filepath.Join(binDir, "terraform-provider-hubspot")
	if exactBinary := os.Getenv("HUBSPOT_ACCEPTANCE_PROVIDER_BINARY"); exactBinary != "" {
		contents, err := os.ReadFile(exactBinary)
		if err != nil {
			t.Fatalf("read exact acceptance provider binary: %v", err)
		}
		if err := os.WriteFile(providerBinary, contents, 0o700); err != nil {
			t.Fatalf("install exact acceptance provider binary: %v", err)
		}
	} else {
		build := exec.Command("go", "build", "-trimpath", "-o", providerBinary, root)
		build.Dir = root
		build.Env = append(os.Environ(), "CGO_ENABLED=0", "GOTOOLCHAIN=local")
		if output, err := build.CombinedOutput(); err != nil {
			t.Fatalf("build acceptance provider: %v: %s", err, boundedText(output))
		}
	}

	cliConfig := filepath.Join(temp, "cli.tfrc")
	config := fmt.Sprintf(`provider_installation {
  dev_overrides {
    "registry.terraform.io/jackemcpherson/hubspot" = %q
    "registry.opentofu.org/jackemcpherson/hubspot" = %q
  }
  direct {}
}
`, binDir, binDir)
	if err := os.WriteFile(cliConfig, []byte(config), 0o600); err != nil {
		t.Fatalf("write CLI configuration: %v", err)
	}

	workDir := filepath.Join(temp, "configuration")
	if err := os.Mkdir(workDir, 0o700); err != nil {
		t.Fatalf("create acceptance configuration directory: %v", err)
	}
	ledgerID, err := randomID()
	if err != nil {
		t.Fatalf("create cleanup ledger identity: %v", err)
	}
	session := &Session{
		t:          t,
		engine:     options.Engine,
		workDir:    workDir,
		env:        append(os.Environ(), "TF_CLI_CONFIG_FILE="+cliConfig, "TF_IN_AUTOMATION=1", "CHECKPOINT_DISABLE=1"),
		ledgerPath: options.LedgerPath,
		ledgerID:   ledgerID,
		shard:      options.Shard,
		prefix:     options.Prefix,
		probeURL:   options.ProbeBaseURL,
	}

	defer session.cleanup()
	scenario(session)
}

func validShard(shard Shard) bool {
	switch shard {
	case FreeProperties, DealPipelines, TicketPipelines, CustomSchemas, SensitiveProperties, CustomPipelines:
		return true
	default:
		return false
	}
}

func (s *Session) Apply(config string) {
	s.t.Helper()
	s.writeConfig(config)
	s.registerCleanup()
	if err := s.command("apply", "-auto-approve", "-input=false", "-no-color"); err != nil {
		s.t.Fatalf("%s apply failed: %v", s.engine, err)
	}
}

func (s *Session) RequireApplyFailure(config string) {
	s.t.Helper()
	s.writeConfig(config)
	s.registerCleanup()
	if err := s.command("apply", "-auto-approve", "-input=false", "-no-color"); err == nil {
		s.t.Fatal("acceptance apply unexpectedly succeeded")
	}
}

func (s *Session) RequireApplyFailureWithStatus(config string, failure Failure) {
	s.t.Helper()
	s.writeConfig(config)
	s.registerCleanup()
	err := s.command("apply", "-auto-approve", "-input=false", "-no-color")
	var commandError engineCommandError
	if err == nil || !errors.As(err, &commandError) {
		s.t.Fatal("acceptance apply did not produce the required safe failure")
	}
	if commandError.status != string(failure) {
		s.t.Fatalf("acceptance apply produced a different safe failure category: %v", err)
	}
}

func (s *Session) Refresh(config string) {
	s.t.Helper()
	s.writeConfig(config)
	if err := s.command("apply", "-refresh-only", "-auto-approve", "-input=false", "-no-color"); err != nil {
		s.t.Fatalf("%s refresh failed: %v", s.engine, err)
	}
}

func (s *Session) Destroy(config string) {
	s.t.Helper()
	s.writeConfig(config)
	if err := s.command("destroy", "-auto-approve", "-input=false", "-no-color"); err != nil {
		s.t.Fatalf("%s destroy failed: %v", s.engine, err)
	}
}

func (s *Session) RequireValidationFailure(config, title string) {
	s.t.Helper()
	s.writeConfig(config)
	err := s.command("validate", "-no-color")
	var commandError engineCommandError
	if err == nil || !errors.As(err, &commandError) {
		s.t.Fatal("acceptance configuration unexpectedly validated")
	}
	if commandError.title != title {
		s.t.Fatalf("acceptance validation produced a different safe failure: %v", err)
	}
}

func (s *Session) RequirePlanFailure(config, title string) {
	s.t.Helper()
	s.writeConfig(config)
	err := s.command("plan", "-input=false", "-lock=false", "-no-color")
	var commandError engineCommandError
	if err == nil || !errors.As(err, &commandError) {
		s.t.Fatal("acceptance plan unexpectedly succeeded")
	}
	if commandError.title != title {
		s.t.Fatalf("acceptance plan produced a different safe failure: %v", err)
	}
}

func (s *Session) RequireEmptyPlan(config string) {
	s.t.Helper()
	s.writeConfig(config)
	if err := s.command("plan", "-detailed-exitcode", "-input=false", "-lock=false", "-no-color"); err != nil {
		s.t.Fatalf("%s plan was not empty: %v", s.engine, err)
	}
}

func (s *Session) RequirePlanDiff(config string) {
	s.t.Helper()
	s.writeConfig(config)
	err := s.command("plan", "-detailed-exitcode", "-input=false", "-lock=false", "-no-color")
	if err == nil {
		s.t.Fatal("acceptance plan did not detect the expected drift")
	}
	var commandError engineCommandError
	if !errors.As(err, &commandError) || commandError.exitCode != 2 {
		s.t.Fatalf("%s drift plan failed: %v", s.engine, err)
	}
}

func (s *Session) RequirePlanDiffAttributes(config, address string, expected ...string) {
	s.t.Helper()
	s.writeConfig(config)
	planPath := filepath.Join(s.workDir, "acceptance.tfplan")
	err := s.command("plan", "-detailed-exitcode", "-input=false", "-lock=false", "-no-color", "-out="+planPath)
	var commandError engineCommandError
	if err == nil || !errors.As(err, &commandError) || commandError.exitCode != 2 {
		s.t.Fatalf("%s exact-drift plan did not produce the required change", s.engine)
	}
	output, err := s.commandOutput("show", "-json", "-no-color", planPath)
	if err != nil {
		s.t.Fatalf("%s exact-drift plan inspection failed: %v", s.engine, err)
	}
	var document struct {
		ResourceChanges []struct {
			Address string `json:"address"`
			Change  struct {
				Before map[string]json.RawMessage `json:"before"`
				After  map[string]json.RawMessage `json:"after"`
			} `json:"change"`
		} `json:"resource_changes"`
	}
	if err := json.Unmarshal([]byte(output), &document); err != nil {
		s.t.Fatalf("decode %s exact-drift plan", s.engine)
	}
	for _, resource := range document.ResourceChanges {
		if resource.Address != address {
			continue
		}
		actual := changedAttributeNames(resource.Change.Before, resource.Change.After)
		want := append([]string(nil), expected...)
		sort.Strings(want)
		if strings.Join(actual, ",") != strings.Join(want, ",") {
			s.t.Fatalf("exact-drift plan changed attributes %s; wanted %s", strings.Join(actual, ","), strings.Join(want, ","))
		}
		return
	}
	s.t.Fatal("exact-drift plan did not contain the required resource")
}

func changedAttributeNames(before, after map[string]json.RawMessage) []string {
	keys := make(map[string]struct{}, len(before)+len(after))
	for key := range before {
		keys[key] = struct{}{}
	}
	for key := range after {
		keys[key] = struct{}{}
	}
	changed := make([]string, 0, len(keys))
	for key := range keys {
		if !bytes.Equal(before[key], after[key]) {
			changed = append(changed, key)
		}
	}
	sort.Strings(changed)
	return changed
}

func (s *Session) RequirePlanWarning(config string, warning Warning) {
	s.t.Helper()
	s.writeConfig(config)
	output, err := s.commandOutput("plan", "-detailed-exitcode", "-input=false", "-lock=false", "-no-color")
	var commandError engineCommandError
	if err == nil || !errors.As(err, &commandError) || commandError.exitCode != 2 {
		s.t.Fatalf("%s warning plan did not produce the required change", s.engine)
	}
	if !strings.Contains(output, "Warning: "+string(warning)) {
		s.t.Fatalf("plan did not emit required warning %q", warning)
	}
}

func (s *Session) RequirePlanWithoutWarning(config string, warning Warning) {
	s.t.Helper()
	s.writeConfig(config)
	output, err := s.commandOutput("plan", "-detailed-exitcode", "-input=false", "-lock=false", "-no-color")
	var commandError engineCommandError
	if err == nil || !errors.As(err, &commandError) || commandError.exitCode != 2 {
		s.t.Fatalf("%s safe-change plan did not produce the required change", s.engine)
	}
	if strings.Contains(output, "Warning: "+string(warning)) {
		s.t.Fatalf("safe-change plan emitted warning %q", warning)
	}
}

func (s *Session) RemoveState(address string) {
	s.t.Helper()
	if err := s.command("state", "rm", address); err != nil {
		s.t.Fatalf("%s state removal failed: %v", s.engine, err)
	}
}

func (s *Session) Import(address, id string) {
	s.t.Helper()
	if err := s.command("import", "-input=false", "-no-color", address, id); err != nil {
		s.t.Fatalf("%s import failed: %v", s.engine, err)
	}
}

func (s *Session) RequireImportFailure(config, address, id, title string) {
	s.t.Helper()
	s.writeConfig(config)
	err := s.command("import", "-input=false", "-no-color", address, id)
	var commandError engineCommandError
	if err == nil || !errors.As(err, &commandError) {
		s.t.Fatal("acceptance import unexpectedly succeeded")
	}
	if commandError.title != title {
		s.t.Fatalf("acceptance import produced a different safe failure: %v", err)
	}
}

func (s *Session) RequireStateString(address, attribute, expected string) {
	s.t.Helper()
	output, err := s.commandOutput("show", "-json", "-no-color")
	if err != nil {
		s.t.Fatalf("%s state inspection failed: %v", s.engine, err)
	}
	var document struct {
		Values struct {
			RootModule struct {
				Resources []struct {
					Address string                     `json:"address"`
					Values  map[string]json.RawMessage `json:"values"`
				} `json:"resources"`
			} `json:"root_module"`
		} `json:"values"`
	}
	if err := json.Unmarshal([]byte(output), &document); err != nil {
		s.t.Fatalf("decode %s state output", s.engine)
	}
	for _, resource := range document.Values.RootModule.Resources {
		if resource.Address != address {
			continue
		}
		var actual string
		if err := json.Unmarshal(resource.Values[attribute], &actual); err != nil || actual != expected {
			s.t.Fatalf("state attribute %s did not match the acceptance contract", attribute)
		}
		return
	}
	s.t.Fatal("acceptance resource was absent from state")
}

func (s *Session) RequireStateStringPrefix(address, attribute, prefix string) {
	s.t.Helper()
	if !strings.HasPrefix(s.OpaqueStateString(address, attribute), prefix) {
		s.t.Fatalf("state attribute %s did not use the required canonical prefix", attribute)
	}
}

func (s *Session) OpaqueStateString(address, attribute string) string {
	s.t.Helper()
	values := s.stateValues(address)
	var value string
	if err := json.Unmarshal(values[attribute], &value); err != nil || value == "" {
		s.t.Fatalf("decode nonempty state attribute %s", attribute)
	}
	return value
}

func (s *Session) OpaqueStateMapNestedStrings(address, mapAttribute, nestedAttribute string) map[string]string {
	s.t.Helper()
	values := s.stateValues(address)
	var entries map[string]map[string]json.RawMessage
	if err := json.Unmarshal(values[mapAttribute], &entries); err != nil {
		s.t.Fatalf("decode state map attribute %s", mapAttribute)
	}
	result := make(map[string]string, len(entries))
	for key, entry := range entries {
		var value string
		if err := json.Unmarshal(entry[nestedAttribute], &value); err != nil || value == "" {
			s.t.Fatalf("decode nonempty nested state attribute %s", nestedAttribute)
		}
		result[key] = value
	}
	return result
}

func (s *Session) RequireStateAbsent(address string) {
	s.t.Helper()
	output, err := s.commandOutput("show", "-json", "-no-color")
	if err != nil {
		s.t.Fatalf("%s state inspection failed: %v", s.engine, err)
	}
	var document struct {
		Values struct {
			RootModule struct {
				Resources []struct {
					Address string `json:"address"`
				} `json:"resources"`
			} `json:"root_module"`
		} `json:"values"`
	}
	if err := json.Unmarshal([]byte(output), &document); err != nil {
		s.t.Fatalf("decode %s state output", s.engine)
	}
	for _, resource := range document.Values.RootModule.Resources {
		if resource.Address == address {
			s.t.Fatal("acceptance resource remained in state after confirmed remote absence")
		}
	}
}

func (s *Session) RequireStateAttributePresent(address, attribute string) {
	s.t.Helper()
	values := s.stateValues(address)
	if _, ok := values[attribute]; !ok {
		s.t.Fatalf("state attribute %s was absent", attribute)
	}
}

func (s *Session) RequireStateMapKey(address, attribute, key string, present bool) {
	s.t.Helper()
	values := s.stateValues(address)
	var entries map[string]json.RawMessage
	if err := json.Unmarshal(values[attribute], &entries); err != nil {
		s.t.Fatalf("decode state map attribute %s", attribute)
	}
	_, found := entries[key]
	if found != present {
		s.t.Fatalf("state map attribute %s key presence did not match the acceptance contract", attribute)
	}
}

func (s *Session) RequireStateMapNestedStringOneOf(address, mapAttribute, nestedAttribute string, allowed ...string) {
	s.t.Helper()
	values := s.stateValues(address)
	var entries map[string]map[string]json.RawMessage
	if err := json.Unmarshal(values[mapAttribute], &entries); err != nil {
		s.t.Fatalf("decode state map attribute %s", mapAttribute)
	}
	accepted := make(map[string]struct{}, len(allowed))
	for _, value := range allowed {
		accepted[value] = struct{}{}
	}
	for _, entry := range entries {
		var value string
		if err := json.Unmarshal(entry[nestedAttribute], &value); err == nil {
			if _, ok := accepted[value]; ok {
				return
			}
		}
	}
	s.t.Fatalf("state map attribute %s did not expose an accepted %s value", mapAttribute, nestedAttribute)
}

func (s *Session) stateValues(address string) map[string]json.RawMessage {
	s.t.Helper()
	output, err := s.commandOutput("show", "-json", "-no-color")
	if err != nil {
		s.t.Fatalf("%s state inspection failed: %v", s.engine, err)
	}
	var document struct {
		Values struct {
			RootModule struct {
				Resources []struct {
					Address string                     `json:"address"`
					Values  map[string]json.RawMessage `json:"values"`
				} `json:"resources"`
			} `json:"root_module"`
		} `json:"values"`
	}
	if err := json.Unmarshal([]byte(output), &document); err != nil {
		s.t.Fatalf("decode %s state output", s.engine)
	}
	for _, resource := range document.Values.RootModule.Resources {
		if resource.Address == address {
			return resource.Values
		}
	}
	s.t.Fatal("acceptance resource was absent from state")
	return nil
}

func (s *Session) writeConfig(config string) {
	s.t.Helper()
	s.config = config
	if err := os.WriteFile(filepath.Join(s.workDir, "main.tf"), []byte(config), 0o600); err != nil {
		s.t.Fatalf("write acceptance configuration: %v", err)
	}
}

func (s *Session) registerCleanup() {
	s.t.Helper()
	if s.registered {
		return
	}
	if err := appendLedger(s.ledgerPath, ledgerEntry{ID: s.ledgerID, Shard: s.shard, Prefix: s.prefix}); err != nil {
		s.t.Fatalf("register acceptance cleanup: %v", err)
	}
	s.registered = true
}

func (s *Session) command(arguments ...string) error {
	_, err := s.commandOutput(arguments...)
	return err
}

func (s *Session) commandOutput(arguments ...string) (string, error) {
	command := exec.Command(string(s.engine), arguments...)
	command.Dir = s.workDir
	command.Env = s.env
	var output bytes.Buffer
	command.Stdout = &output
	command.Stderr = &output
	err := command.Run()
	if err == nil {
		return output.String(), nil
	}
	return output.String(), sanitizedEngineError(err, output.String())
}

func (s *Session) cleanup() {
	if !s.registered {
		return
	}
	if s.config == "" {
		s.t.Errorf("acceptance cleanup had no configuration")
		return
	}
	if err := s.command("destroy", "-auto-approve", "-input=false", "-no-color"); err != nil {
		s.t.Errorf("%s acceptance cleanup failed: %v", s.engine, err)
		return
	}
	if err := removeLedger(s.ledgerPath, s.ledgerID); err != nil {
		s.t.Errorf("complete acceptance cleanup ledger: %v", err)
	}
}

type ledgerEntry struct {
	ID     string `json:"id"`
	Shard  Shard  `json:"shard"`
	Prefix string `json:"prefix"`
}

func appendLedger(path string, entry ledgerEntry) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	return json.NewEncoder(file).Encode(entry)
}

func removeLedger(path, id string) error {
	contents, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(strings.TrimSpace(string(contents)), "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			continue
		}
		var entry ledgerEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			return fmt.Errorf("decode cleanup ledger: %w", err)
		}
		if entry.ID != id {
			kept = append(kept, line)
		}
	}
	output := ""
	if len(kept) > 0 {
		output = strings.Join(kept, "\n") + "\n"
	}
	return os.WriteFile(path, []byte(output), 0o600)
}

func repositoryRoot() (string, error) {
	directory, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(directory, "go.mod")); err == nil {
			return directory, nil
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			return "", errors.New("repository root not found")
		}
		directory = parent
	}
}

func randomID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}

func boundedText(value []byte) string {
	const limit = 2048
	if len(value) > limit {
		value = value[:limit]
	}
	return strings.TrimSpace(string(value))
}

type engineCommandError struct {
	exitCode int
	title    string
	status   string
}

func (e engineCommandError) Error() string {
	detail := "exit code " + strconv.Itoa(e.exitCode)
	if e.title != "" {
		detail += "; " + e.title
	}
	if e.status != "" {
		detail += "; " + e.status
	}
	return detail
}

func sanitizedEngineError(err error, output string) error {
	exitCode := 1
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		exitCode = exitError.ExitCode()
	}
	title := ""
	if match := engineErrorTitle.FindStringSubmatch(output); len(match) == 2 {
		title = match[1]
	}
	status := ""
	if match := hubSpotStatus.FindStringSubmatch(output); len(match) > 1 {
		status = "HubSpot HTTP " + match[1]
		if len(match) > 2 && match[2] != "" {
			status += " (" + match[2] + ")"
		}
		if len(match) > 3 && match[3] != "" {
			status += " [" + match[3] + "]"
		}
	}
	if match := inconsistentPath.FindStringSubmatch(output); status == "" && len(match) == 2 {
		status = "inconsistent state attribute " + match[1]
	}
	if match := stateValuePath.FindStringSubmatch(output); status == "" && len(match) == 2 {
		status = "inconsistent state attribute " + match[1]
	}
	if matches := planChangePath.FindAllStringSubmatch(output, -1); status == "" && len(matches) > 0 {
		attributes := make([]string, 0, len(matches))
		for _, match := range matches {
			if len(match) == 2 {
				attributes = append(attributes, match[1])
			}
		}
		status = "planned changes to attributes " + strings.Join(attributes, ",")
	}
	return engineCommandError{exitCode: exitCode, title: title, status: status}
}
