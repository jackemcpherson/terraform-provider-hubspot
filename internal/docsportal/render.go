// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

package docsportal

import (
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func resetOutput(output string) error {
	if err := os.RemoveAll(output); err != nil {
		return fmt.Errorf("reset portal output: %w", err)
	}
	for _, directory := range []string{"assets", "resources", "data-sources", "modules"} {
		if err := os.MkdirAll(filepath.Join(output, directory), 0o755); err != nil {
			return err
		}
	}
	return writeFile(filepath.Join(output, "assets", "style.css"), portalCSS)
}

func renderPortal(config Config, metadata manifest, resources, dataSources []providerType, modules []moduleDoc) error {
	header := provenanceHeader(metadata)
	if err := writeFile(filepath.Join(config.OutputDir, "index.html"), page("HubSpot configuration portal", "", header, `<h1>HubSpot configuration surfaces</h1><ul><li><a href="crm-property-schema.html">CRM property schema</a></li><li><a href="form-definition.html">Form definition</a></li><li><a href="files-configuration.html">Files configuration</a></li><li><a href="account-membership.html">Account membership</a></li><li><a href="crm-user-profile.html">CRM user profile</a></li><li><a href="product-definition.html">Product definition</a></li></ul><nav><a href="resources/index.html">Resources</a> · <a href="data-sources/index.html">Data sources</a> · <a href="modules/index.html">Consumer modules</a> · <a href="provenance.json">Provenance</a></nav>`)); err != nil {
		return err
	}
	if err := writeSurfaceOverview(config, "crm-property-schema", "CRM property schema", `<a href="resources/index.html">Provider resources</a> · <a href="data-sources/index.html">Property discovery</a> · <a href="modules/crm-schema.html">crm-schema module</a>`, header); err != nil {
		return err
	}
	if err := writeSurfaceOverview(config, "form-definition", "Form definition", `<a href="resources/hubspot_form_definition.html">hubspot_form_definition resource</a> · <a href="modules/form-definition.html">form-definition module</a>`, header); err != nil {
		return err
	}
	if err := writeSurfaceOverview(config, "files-configuration", "Files configuration", `<a href="resources/hubspot_file_folder.html">hubspot_file_folder resource</a> · <a href="resources/hubspot_file.html">hubspot_file resource</a> · <a href="modules/files-configuration.html">files-configuration module</a>`, header); err != nil {
		return err
	}
	if err := writeSurfaceOverview(config, "account-membership", "Account membership", `<a href="resources/hubspot_account_membership.html">hubspot_account_membership resource</a> · <a href="modules/account-membership.html">account-membership module</a>`, header); err != nil {
		return err
	}
	if err := writeSurfaceOverview(config, "crm-user-profile", "CRM user profile", `<a href="resources/hubspot_crm_user_profile.html">hubspot_crm_user_profile resource</a> · <a href="modules/crm-user-profile.html">crm-user-profile module</a>`, header); err != nil {
		return err
	}
	if err := writeSurfaceOverview(config, "product-definition", "Product definition", `<a href="resources/hubspot_product.html">hubspot_product resource</a> · <a href="modules/product-definition.html">product-definition module</a>`, header); err != nil {
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
	return nil
}

func writeSurfaceOverview(config Config, slug, title, links, header string) error {
	overview, err := os.ReadFile(filepath.Join(config.ProviderRepo, "docs", "surfaces", slug+".md"))
	if err != nil {
		return fmt.Errorf("%s overview: %w", title, err)
	}
	body := `<h1>` + template.HTMLEscapeString(title) + `</h1><pre class="prose">` + template.HTMLEscapeString(string(overview)) + `</pre><p>` + links + `</p>`
	return writeFile(filepath.Join(config.OutputDir, slug+".html"), page(title, "", header, body))
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
		body.WriteString(resourceImport(entry.Name))
	}
	body.WriteString(lifecycle)
	body.WriteString(`<p><a href="index.html">Back to index</a> · <a href="../` + providerSurface(entry.Name) + `.html">Surface overview</a></p>`)
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
	body.WriteString(moduleContract(module.Name))
	if module.Guide != "" {
		fmt.Fprintf(&body, `<h2>Module guidance</h2><pre class="prose">%s</pre>`, template.HTMLEscapeString(module.Guide))
	}
	writeSourceFiles(&body, "Complete usage", module.Usage)
	writeSourceFiles(&body, "Module source", module.Sources)
	body.WriteString(`<p><a href="index.html">Back to index</a> · <a href="../` + moduleSurface(module.Name) + `.html">Surface overview</a></p>`)
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
	if name == "hubspot_product" {
		return "<h2>Lifecycle</h2><p>Refresh, recovery, PATCH, import, and archival use only the exact generated Product ID. PATCH contains changed managed properties only; semantic decimal normalization sends no write. Destroy verifies active absence and the same archived identity.</p>"
	}
	if name == "hubspot_crm_user_profile" {
		return "<h2>Lifecycle</h2><p>Create waits for a unique Settings-to-CRM identity join, then writes only changed managed properties with time zone before working hours. Refresh verifies both identities. Destroy performs no remote write and retains profile values.</p>"
	}
	if name == "hubspot_account_membership" {
		return "<h2>Lifecycle</h2><p>Refresh uses the canonical Settings user ID. Name PUT fails closed around activation and current assignments. Destroy requires the local removal opt-in, exact ID and email, non-Super-Admin status, and verified account-membership absence without deleting global identity.</p>"
	}
	if name == "hubspot_file_folder" {
		return "<h2>Lifecycle</h2><p>This Files configuration resource refreshes and asynchronously renames or moves by exact generated ID. Destroy refuses active direct children, verifies active absence, and never invokes cascade deletion. HubSpot may retain the folder in Trash after active name reuse.</p>"
	}
	if name == "hubspot_file" {
		return "<h2>Lifecycle</h2><p>This Files configuration resource observes metadata and content drift by exact generated ID. PATCH changes metadata and PUT replaces bytes in place. Destroy verifies active absence while HubSpot-managed Trash retention may continue.</p>"
	}
	if name == "hubspot_form_definition" {
		return "<h2>Lifecycle</h2><p>Refresh and bounded PATCH use the exact generated UUID. Unsupported structure fails closed. Destroy verifies active absence and the same Archived form definition; external archive or complete disappearance plans a new UUID.</p>"
	}
	if name == "hubspot_property_group" {
		return "<h2>Lifecycle</h2><p>Refresh observes label and ordering drift. Destroy archives only after active properties are gone. Active absence permits immediate name reuse.</p>"
	}
	return "<h2>Lifecycle</h2><p>Refresh observes scalar and option drift. Destroy archives, confirms the tombstone, and permits immediate same-name creation. Option-value removal does not migrate CRM record values.</p>"
}

func resourceImport(name string) string {
	if name == "hubspot_product" {
		return `<h2>Import</h2><p>Import one active Product by its exact numeric generated Product ID. SKU and name are never identity. Supported nonempty optional values are adopted.</p>`
	}
	if name == "hubspot_crm_user_profile" {
		return `<h2>Import</h2><p>Import by canonical numeric CRM user ID or explicit <code>membership:Settings-ID</code>. State always stores the canonical account-specific CRM ID.</p>`
	}
	if name == "hubspot_account_membership" {
		return `<h2>Import</h2><p>Import one membership by canonical numeric Settings user ID or explicit <code>email:address</code> lookup. State always stores the canonical ID and records welcome delivery and removal opt-in as false.</p>`
	}
	if name == "hubspot_file_folder" {
		return `<h2>Import</h2><p>Import one active File folder by its exact non-zero decimal generated ID. Names and paths are observations and are never import identity.</p>`
	}
	if name == "hubspot_file" {
		return `<h2>Import</h2><p>Import one active Managed file by its exact non-zero decimal generated ID. Configuration must still provide the sensitive local source path and reviewed SHA-256 digest.</p>`
	}
	if name == "hubspot_form_definition" {
		return `<h2>Import</h2><p>Import one supported active form by its exact lowercase generated UUID. Names, URLs, composite identifiers, and Archived form definitions are rejected without mutation.</p>`
	}
	return `<h2>Import</h2><p>Use exact <code>object_type/name</code> identity.</p>`
}

func providerSurface(name string) string {
	if name == "hubspot_product" {
		return "product-definition"
	}
	if name == "hubspot_crm_user_profile" {
		return "crm-user-profile"
	}
	if name == "hubspot_account_membership" {
		return "account-membership"
	}
	if name == "hubspot_file_folder" || name == "hubspot_file" {
		return "files-configuration"
	}
	if name == "hubspot_form_definition" {
		return "form-definition"
	}
	return "crm-property-schema"
}

func moduleSurface(name string) string {
	if name == "product-definition" {
		return "product-definition"
	}
	if name == "crm-user-profile" {
		return "crm-user-profile"
	}
	if name == "account-membership" {
		return "account-membership"
	}
	if name == "files-configuration" {
		return "files-configuration"
	}
	if name == "form-definition" {
		return "form-definition"
	}
	return "crm-property-schema"
}

func moduleContract(name string) string {
	if name == "product-definition" {
		return "<h2>Validation and dependency contract</h2><p>Stable map keys own generated Product identities. Null optionals remain unmanaged, empty strings clear them, and decimal strings preserve configured intent across remote normalization.</p>"
	}
	if name == "crm-user-profile" {
		return "<h2>Validation and dependency contract</h2><p>Stable map keys own CRM profile management relationships. Account-membership IDs create explicit ordering. Null properties remain unmanaged; working hours require a managed time zone.</p>"
	}
	if name == "account-membership" {
		return "<h2>Validation and dependency contract</h2><p>Stable map keys own canonical Settings identities. Each entry requires an explicit welcome-email choice; removal defaults off. Email or welcome changes replace membership, while configured names remain guarded global-identity updates.</p>"
	}
	if name == "files-configuration" {
		return "<h2>Validation and dependency contract</h2><p>Stable map keys own generated identities. One module instance manages one hierarchy level; generated folder ID references compose deeper levels and produce file-first, leaf-first teardown edges. Rename a key only with an explicit moved block.</p>"
	}
	if name == "form-definition" {
		return "<h2>Validation and dependency contract</h2><p>Stable map keys own generated identities; names are unique mutable presentation within one module instance. Defaults and typed overrides come from the HCL below. The built-in contacts email Property definition is a semantic prerequisite, not a crm-schema module dependency.</p>"
	}
	return "<h2>Validation and dependency contract</h2><p>Inputs and validations come from the module HCL below. Property group references create implicit creation and property-first teardown ordering.</p>"
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

const portalCSS = `:root{font-family:system-ui,sans-serif;color:#172033;background:#f7f8fb}body{margin:0}header{display:flex;gap:1rem;flex-wrap:wrap;background:#172033;color:white;padding:1rem 2rem}main{max-width:72rem;margin:auto;padding:2rem}a{color:#2854b8}table{border-collapse:collapse;width:100%}th,td{border:1px solid #c7ccda;padding:.6rem;text-align:left;vertical-align:top}pre{overflow:auto;background:white;border:1px solid #dce0ea;padding:1rem;white-space:pre-wrap}.prose{line-height:1.5}code{font-family:ui-monospace,monospace}`
