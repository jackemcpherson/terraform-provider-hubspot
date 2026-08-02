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
	return nil
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

const portalCSS = `:root{font-family:system-ui,sans-serif;color:#172033;background:#f7f8fb}body{margin:0}header{display:flex;gap:1rem;flex-wrap:wrap;background:#172033;color:white;padding:1rem 2rem}main{max-width:72rem;margin:auto;padding:2rem}a{color:#2854b8}table{border-collapse:collapse;width:100%}th,td{border:1px solid #c7ccda;padding:.6rem;text-align:left;vertical-align:top}pre{overflow:auto;background:white;border:1px solid #dce0ea;padding:1rem;white-space:pre-wrap}.prose{line-height:1.5}code{font-family:ui-monospace,monospace}`
