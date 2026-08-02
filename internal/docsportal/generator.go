// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

package docsportal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	frameworkprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

type Config struct {
	Provider               frameworkprovider.Provider
	ProviderRepo           string
	DemoRepo               string
	OutputDir              string
	Version                string
	Released               bool
	RequireClean           bool
	ExpectedProviderCommit string
	ExpectedDemoCommit     string
	ProviderProvenance     Provenance
	DemoProvenance         Provenance
}

type Provenance struct {
	Commit    string `json:"commit"`
	Timestamp string `json:"timestamp"`
	Dirty     bool   `json:"dirty"`
}

type manifest struct {
	Version     string     `json:"version"`
	State       string     `json:"state"`
	GeneratedAt string     `json:"generated_at"`
	Provider    Provenance `json:"provider"`
	Demo        Provenance `json:"demo"`
	Resources   []string   `json:"resources"`
	DataSources []string   `json:"data_sources"`
	Modules     []string   `json:"modules"`
}

type providerRegistrar interface {
	Resources(context.Context) []func() resource.Resource
	DataSources(context.Context) []func() datasource.DataSource
}

type attributeDoc struct {
	Name        string
	Description string
	Mode        string
}

type providerType struct {
	Name        string
	Description string
	Attributes  []attributeDoc
	Example     string
}

type moduleDoc struct {
	Name      string
	Variables []string
	Resources []string
	Outputs   []string
	Sources   map[string]string
	Usage     map[string]string
}

var (
	blockNamePattern = regexp.MustCompile(`(?m)^\s*(variable|output)\s+"([^"]+)"\s*\{`)
	resourcePattern  = regexp.MustCompile(`(?m)^\s*resource\s+"([^"]+)"\s+"[^"]+"\s*\{`)
	hrefPattern      = regexp.MustCompile(`href="([^"]+)"`)
)

func Generate(ctx context.Context, config Config) error {
	if config.Provider == nil || config.ProviderRepo == "" || config.DemoRepo == "" || config.OutputDir == "" || config.Version == "" {
		return errors.New("provider, repositories, output directory, and version are required")
	}
	absOutput, err := filepath.Abs(config.OutputDir)
	if err != nil {
		return err
	}
	absProvider, _ := filepath.Abs(config.ProviderRepo)
	absDemo, _ := filepath.Abs(config.DemoRepo)
	if filepath.Dir(absOutput) == absOutput || absOutput == absProvider || absOutput == absDemo {
		return errors.New("portal output must be a dedicated non-root directory")
	}
	config.OutputDir = absOutput
	registrar, ok := config.Provider.(providerRegistrar)
	if !ok {
		return errors.New("provider does not expose registered resources and data sources")
	}
	if config.ProviderProvenance.Commit == "" {
		provenance, err := gitProvenance(config.ProviderRepo)
		if err != nil {
			return fmt.Errorf("provider provenance: %w", err)
		}
		config.ProviderProvenance = provenance
	}
	if config.DemoProvenance.Commit == "" {
		provenance, err := gitProvenance(config.DemoRepo)
		if err != nil {
			return fmt.Errorf("demo provenance: %w", err)
		}
		config.DemoProvenance = provenance
	}
	if config.RequireClean && (config.ProviderProvenance.Dirty || config.DemoProvenance.Dirty) {
		return errors.New("candidate portal generation requires clean provider and demo checkouts")
	}
	if config.ExpectedProviderCommit != "" && config.ProviderProvenance.Commit != config.ExpectedProviderCommit {
		return errors.New("provider checkout does not match the candidate evidence anchor")
	}
	if config.ExpectedDemoCommit != "" && config.DemoProvenance.Commit != config.ExpectedDemoCommit {
		return errors.New("demo checkout does not match the candidate evidence anchor")
	}

	resources, err := discoverResources(ctx, registrar, config.ProviderRepo)
	if err != nil {
		return err
	}
	dataSources, err := discoverDataSources(ctx, registrar, config.ProviderRepo)
	if err != nil {
		return err
	}
	modules, err := discoverModules(config.DemoRepo)
	if err != nil {
		return err
	}
	if !containsModule(modules, "crm-schema") {
		return errors.New("required consumer module crm-schema was not discovered from HCL")
	}
	for _, required := range []string{"hubspot_property_group", "hubspot_property"} {
		if !containsProviderType(resources, required) {
			return fmt.Errorf("required registered resource %s was not discovered", required)
		}
	}
	for _, required := range []string{"hubspot_property_definition", "hubspot_property_definitions"} {
		if !containsProviderType(dataSources, required) {
			return fmt.Errorf("required registered data source %s was not discovered", required)
		}
	}

	if err := os.RemoveAll(config.OutputDir); err != nil {
		return fmt.Errorf("reset portal output: %w", err)
	}
	for _, directory := range []string{"assets", "resources", "data-sources", "modules"} {
		if err := os.MkdirAll(filepath.Join(config.OutputDir, directory), 0o755); err != nil {
			return err
		}
	}
	if err := writeFile(filepath.Join(config.OutputDir, "assets", "style.css"), portalCSS); err != nil {
		return err
	}

	state := "unreleased"
	if config.Released {
		state = "released"
	}
	metadata := manifest{
		Version: config.Version, State: state, GeneratedAt: config.ProviderProvenance.Timestamp,
		Provider: config.ProviderProvenance, Demo: config.DemoProvenance,
		Resources: namesOfProviderTypes(resources), DataSources: namesOfProviderTypes(dataSources), Modules: namesOfModules(modules),
	}
	encoded, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}
	if err := writeFile(filepath.Join(config.OutputDir, "provenance.json"), string(encoded)+"\n"); err != nil {
		return err
	}

	header := provenanceHeader(metadata)
	if err := writeFile(filepath.Join(config.OutputDir, "index.html"), page("CRM property schema portal", "", header, `<h1>HubSpot configuration surfaces</h1><p><a href="crm-property-schema.html">CRM property schema</a></p><nav><a href="resources/index.html">Resources</a> · <a href="data-sources/index.html">Data sources</a> · <a href="modules/index.html">Consumer modules</a> · <a href="provenance.json">Provenance</a></nav>`)); err != nil {
		return err
	}
	overview, err := os.ReadFile(filepath.Join(config.ProviderRepo, "docs", "surfaces", "crm-property-schema.md"))
	if err != nil {
		return fmt.Errorf("CRM property schema overview: %w", err)
	}
	overviewBody := `<h1>CRM property schema</h1><pre class="prose">` + template.HTMLEscapeString(string(overview)) + `</pre><p><a href="resources/index.html">Provider resources</a> · <a href="data-sources/index.html">Property discovery</a> · <a href="modules/crm-schema.html">crm-schema module</a></p>`
	if err := writeFile(filepath.Join(config.OutputDir, "crm-property-schema.html"), page("CRM property schema", "", header, overviewBody)); err != nil {
		return err
	}
	if err := writeProviderIndex(config.OutputDir, "resources", "Resources", resources, header); err != nil {
		return err
	}
	if err := writeProviderIndex(config.OutputDir, "data-sources", "Data sources", dataSources, header); err != nil {
		return err
	}
	for _, entry := range resources {
		if err := writeProviderPage(config.OutputDir, "resources", entry, header, resourceLifecycle(entry.Name)); err != nil {
			return err
		}
	}
	for _, entry := range dataSources {
		if err := writeProviderPage(config.OutputDir, "data-sources", entry, header, "<h2>Lifecycle</h2><p>Observational only. Reads property definitions without adopting, mutating, archiving, restoring, or reading CRM record values.</p>"); err != nil {
			return err
		}
	}
	if err := writeModuleIndex(config.OutputDir, modules, header); err != nil {
		return err
	}
	for _, module := range modules {
		if err := writeModulePage(config.OutputDir, module, header); err != nil {
			return err
		}
	}
	return ValidateLinks(config.OutputDir)
}

func discoverResources(ctx context.Context, registrar providerRegistrar, providerRepo string) ([]providerType, error) {
	entries := make([]providerType, 0)
	for _, factory := range registrar.Resources(ctx) {
		entry := factory()
		var metadata resource.MetadataResponse
		entry.Metadata(ctx, resource.MetadataRequest{}, &metadata)
		var response resource.SchemaResponse
		entry.Schema(ctx, resource.SchemaRequest{}, &response)
		if response.Diagnostics.HasError() {
			return nil, fmt.Errorf("resource schema %s returned diagnostics", metadata.TypeName)
		}
		entries = append(entries, providerType{Name: metadata.TypeName, Description: response.Schema.Description, Attributes: resourceAttributes(response.Schema.Attributes), Example: resourceExample(providerRepo, metadata.TypeName)})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	return entries, nil
}

func discoverDataSources(ctx context.Context, registrar providerRegistrar, providerRepo string) ([]providerType, error) {
	entries := make([]providerType, 0)
	for _, factory := range registrar.DataSources(ctx) {
		entry := factory()
		var metadata datasource.MetadataResponse
		entry.Metadata(ctx, datasource.MetadataRequest{}, &metadata)
		var response datasource.SchemaResponse
		entry.Schema(ctx, datasource.SchemaRequest{}, &response)
		if response.Diagnostics.HasError() {
			return nil, fmt.Errorf("data source schema %s returned diagnostics", metadata.TypeName)
		}
		entries = append(entries, providerType{Name: metadata.TypeName, Description: response.Schema.Description, Attributes: dataSourceAttributes(response.Schema.Attributes), Example: dataSourceExample(providerRepo, metadata.TypeName)})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	return entries, nil
}

type schemaAttribute interface {
	GetDescription() string
	IsRequired() bool
	IsOptional() bool
	IsComputed() bool
}

func schemaAttributes[T schemaAttribute](attributes map[string]T) []attributeDoc {
	result := make([]attributeDoc, 0, len(attributes))
	for name, attribute := range attributes {
		result = append(result, attributeDoc{Name: name, Description: attribute.GetDescription(), Mode: attributeMode(attribute.IsRequired(), attribute.IsOptional(), attribute.IsComputed())})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

func resourceAttributes(attributes map[string]resourceschema.Attribute) []attributeDoc {
	return schemaAttributes(attributes)
}

func dataSourceAttributes(attributes map[string]datasourceschema.Attribute) []attributeDoc {
	return schemaAttributes(attributes)
}

func attributeMode(required, optional, computed bool) string {
	parts := make([]string, 0, 2)
	if required {
		parts = append(parts, "required")
	}
	if optional {
		parts = append(parts, "optional")
	}
	if computed {
		parts = append(parts, "computed")
	}
	return strings.Join(parts, ", ")
}

func discoverModules(demoRepo string) ([]moduleDoc, error) {
	root := filepath.Join(demoRepo, "modules")
	directories, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("discover consumer modules: %w", err)
	}
	modules := make([]moduleDoc, 0)
	for _, directory := range directories {
		if !directory.IsDir() {
			continue
		}
		path := filepath.Join(root, directory.Name())
		files, err := filepath.Glob(filepath.Join(path, "*.tf"))
		if err != nil || len(files) == 0 {
			continue
		}
		module := moduleDoc{Name: directory.Name(), Sources: make(map[string]string), Usage: make(map[string]string)}
		for _, file := range files {
			contents, readErr := os.ReadFile(file)
			if readErr != nil {
				return nil, readErr
			}
			text := string(contents)
			module.Sources[filepath.Base(file)] = text
			for _, match := range blockNamePattern.FindAllStringSubmatch(text, -1) {
				if match[1] == "variable" {
					module.Variables = append(module.Variables, match[2])
				} else {
					module.Outputs = append(module.Outputs, match[2])
				}
			}
			for _, match := range resourcePattern.FindAllStringSubmatch(text, -1) {
				module.Resources = append(module.Resources, match[1])
			}
		}
		sort.Strings(module.Variables)
		sort.Strings(module.Resources)
		sort.Strings(module.Outputs)
		rootFiles, globErr := filepath.Glob(filepath.Join(demoRepo, "*.tf"))
		if globErr != nil {
			return nil, globErr
		}
		usesModule := false
		rootSources := make(map[string]string, len(rootFiles))
		for _, file := range rootFiles {
			contents, readErr := os.ReadFile(file)
			if readErr != nil {
				return nil, readErr
			}
			rootSources[filepath.Base(file)] = string(contents)
			usesModule = usesModule || strings.Contains(string(contents), "./modules/"+module.Name)
		}
		if usesModule {
			module.Usage = rootSources
		}
		modules = append(modules, module)
	}
	sort.Slice(modules, func(i, j int) bool { return modules[i].Name < modules[j].Name })
	return modules, nil
}

func resourceExample(repo, name string) string {
	short := strings.TrimPrefix(name, "hubspot_")
	return readOptional(filepath.Join(repo, "examples", "resources", name, "resource.tf"), filepath.Join(repo, "examples", short, "main.tf"))
}

func dataSourceExample(repo, name string) string {
	return readOptional(filepath.Join(repo, "examples", "data-sources", name, "data-source.tf"))
}

func readOptional(paths ...string) string {
	for _, path := range paths {
		if contents, err := os.ReadFile(path); err == nil {
			return string(contents)
		}
	}
	return ""
}

func writeProviderIndex(output, directory, title string, entries []providerType, header string) error {
	var body strings.Builder
	fmt.Fprintf(&body, "<h1>%s</h1><ul>", template.HTMLEscapeString(title))
	for _, entry := range entries {
		fmt.Fprintf(&body, `<li><a href="%s.html">%s</a></li>`, entry.Name, entry.Name)
	}
	body.WriteString(`</ul><p><a href="../index.html">Portal home</a></p>`)
	return writeFile(filepath.Join(output, directory, "index.html"), page(title, "../", header, body.String()))
}

func writeProviderPage(output, directory string, entry providerType, header, lifecycle string) error {
	var body strings.Builder
	fmt.Fprintf(&body, "<h1>%s</h1><p>%s</p><h2>Schema</h2><table><thead><tr><th>Attribute</th><th>Mode</th><th>Description</th></tr></thead><tbody>", entry.Name, template.HTMLEscapeString(entry.Description))
	for _, attribute := range entry.Attributes {
		fmt.Fprintf(&body, "<tr><td><code>%s</code></td><td>%s</td><td>%s</td></tr>", attribute.Name, attribute.Mode, template.HTMLEscapeString(attribute.Description))
	}
	body.WriteString("</tbody></table>")
	if entry.Example != "" {
		fmt.Fprintf(&body, `<h2>Example</h2><pre><code>%s</code></pre>`, template.HTMLEscapeString(entry.Example))
	}
	if strings.HasPrefix(directory, "resources") {
		body.WriteString(`<h2>Import</h2><p>Use exact <code>object_type/name</code> identity.</p>`)
	}
	body.WriteString(lifecycle)
	body.WriteString(`<p><a href="index.html">Back to index</a> · <a href="../crm-property-schema.html">Surface overview</a></p>`)
	return writeFile(filepath.Join(output, directory, entry.Name+".html"), page(entry.Name, "../", header, body.String()))
}

func writeModuleIndex(output string, modules []moduleDoc, header string) error {
	var body strings.Builder
	body.WriteString("<h1>Consumer modules</h1><ul>")
	for _, module := range modules {
		fmt.Fprintf(&body, `<li><a href="%s.html">%s</a></li>`, module.Name, module.Name)
	}
	body.WriteString(`</ul><p><a href="../index.html">Portal home</a></p>`)
	return writeFile(filepath.Join(output, "modules", "index.html"), page("Consumer modules", "../", header, body.String()))
}

func writeModulePage(output string, module moduleDoc, header string) error {
	var body strings.Builder
	fmt.Fprintf(&body, "<h1>%s module</h1>", module.Name)
	writeStringList(&body, "Typed inputs", module.Variables)
	writeStringList(&body, "Resources", module.Resources)
	writeStringList(&body, "Outputs", module.Outputs)
	body.WriteString("<h2>Validation and dependency contract</h2><p>Inputs and validations come from the module HCL below. Property group references create implicit creation and property-first teardown ordering.</p>")
	writeSourceFiles(&body, "Complete usage", module.Usage)
	writeSourceFiles(&body, "Module source", module.Sources)
	body.WriteString(`<p><a href="index.html">Back to index</a> · <a href="../crm-property-schema.html">Surface overview</a></p>`)
	return writeFile(filepath.Join(output, "modules", module.Name+".html"), page(module.Name+" module", "../", header, body.String()))
}

func writeSourceFiles(body *strings.Builder, title string, sources map[string]string) {
	if len(sources) == 0 {
		return
	}
	fmt.Fprintf(body, "<h2>%s</h2>", title)
	filenames := make([]string, 0, len(sources))
	for name := range sources {
		filenames = append(filenames, name)
	}
	sort.Strings(filenames)
	for _, name := range filenames {
		fmt.Fprintf(body, "<h3>%s</h3><pre><code>%s</code></pre>", name, template.HTMLEscapeString(sources[name]))
	}
}

func writeStringList(body *strings.Builder, title string, values []string) {
	fmt.Fprintf(body, "<h2>%s</h2><ul>", title)
	for _, value := range values {
		fmt.Fprintf(body, "<li><code>%s</code></li>", value)
	}
	body.WriteString("</ul>")
}

func resourceLifecycle(name string) string {
	if name == "hubspot_property_group" {
		return "<h2>Lifecycle</h2><p>Refresh observes label and ordering drift. Destroy archives only after active properties are gone. Active absence permits immediate name reuse.</p>"
	}
	return "<h2>Lifecycle</h2><p>Refresh observes scalar and option drift. Destroy archives, confirms the tombstone, and permits immediate same-name creation. Option-value removal does not migrate CRM record values.</p>"
}

func provenanceHeader(metadata manifest) string {
	providerState := "clean"
	if metadata.Provider.Dirty {
		providerState = "dirty"
	}
	demoState := "clean"
	if metadata.Demo.Dirty {
		demoState = "dirty"
	}
	return fmt.Sprintf(`<header><strong>v%s %s</strong><span>provider %s (%s)</span><span>demo %s (%s)</span><span>generated %s</span></header>`, metadata.Version, metadata.State, metadata.Provider.Commit, providerState, metadata.Demo.Commit, demoState, metadata.GeneratedAt)
}

func page(title, prefix, header, body string) string {
	return "<!doctype html>\n<html lang=\"en\"><head><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width,initial-scale=1\"><title>" + template.HTMLEscapeString(title) + "</title><link rel=\"stylesheet\" href=\"" + prefix + "assets/style.css\"></head><body>" + header + "<main>" + body + "</main></body></html>\n"
}

func writeFile(path, contents string) error {
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func gitProvenance(repo string) (Provenance, error) {
	commit, err := gitOutput(repo, "rev-parse", "HEAD")
	if err != nil {
		return Provenance{}, err
	}
	timestamp, err := gitOutput(repo, "show", "-s", "--format=%cI", "HEAD")
	if err != nil {
		return Provenance{}, err
	}
	dirty, err := gitOutput(repo, "status", "--porcelain")
	if err != nil {
		return Provenance{}, err
	}
	return Provenance{Commit: commit, Timestamp: timestamp, Dirty: dirty != ""}, nil
}

func gitOutput(repo string, arguments ...string) (string, error) {
	command := exec.Command("git", append([]string{"-C", repo}, arguments...)...)
	output, err := command.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func ValidateLinks(root string) error {
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".html" {
			return nil
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, match := range hrefPattern.FindAllStringSubmatch(string(contents), -1) {
			href := strings.Split(strings.Split(match[1], "#")[0], "?")[0]
			if href == "" || strings.Contains(href, "://") || strings.HasPrefix(href, "mailto:") {
				continue
			}
			target := filepath.Clean(filepath.Join(filepath.Dir(path), filepath.FromSlash(href)))
			if _, err := os.Stat(target); err != nil {
				return fmt.Errorf("broken link %s in %s", match[1], path)
			}
		}
		return nil
	})
}

func CompareTrees(first, second string) error {
	firstFiles, err := treeContents(first)
	if err != nil {
		return err
	}
	secondFiles, err := treeContents(second)
	if err != nil {
		return err
	}
	if len(firstFiles) != len(secondFiles) {
		return fmt.Errorf("portal regeneration changed file count from %d to %d", len(firstFiles), len(secondFiles))
	}
	for name, contents := range firstFiles {
		if secondContents, ok := secondFiles[name]; !ok || secondContents != contents {
			return fmt.Errorf("portal regeneration changed %s", name)
		}
	}
	return nil
}

func treeContents(root string) (map[string]string, error) {
	files := make(map[string]string)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return walkErr
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[relative] = string(contents)
		return nil
	})
	return files, err
}

func containsProviderType(entries []providerType, name string) bool {
	for _, entry := range entries {
		if entry.Name == name {
			return true
		}
	}
	return false
}
func containsModule(entries []moduleDoc, name string) bool {
	for _, entry := range entries {
		if entry.Name == name {
			return true
		}
	}
	return false
}
func namesOfProviderTypes(entries []providerType) []string {
	names := make([]string, len(entries))
	for index, entry := range entries {
		names[index] = entry.Name
	}
	return names
}
func namesOfModules(entries []moduleDoc) []string {
	names := make([]string, len(entries))
	for index, entry := range entries {
		names[index] = entry.Name
	}
	return names
}

const portalCSS = `:root{font-family:system-ui,sans-serif;color:#172033;background:#f7f8fb}body{margin:0}header{display:flex;gap:1rem;flex-wrap:wrap;background:#172033;color:white;padding:1rem 2rem}main{max-width:72rem;margin:auto;padding:2rem}a{color:#2854b8}table{border-collapse:collapse;width:100%}th,td{border:1px solid #c7ccda;padding:.6rem;text-align:left;vertical-align:top}pre{overflow:auto;background:white;border:1px solid #dce0ea;padding:1rem;white-space:pre-wrap}.prose{line-height:1.5}code{font-family:ui-monospace,monospace}`
