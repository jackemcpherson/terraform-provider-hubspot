// Copyright (c) 2026 jackemcpherson
// SPDX-License-Identifier: MPL-2.0

package docsportal

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"golang.org/x/net/html"
)

var hrefPattern = regexp.MustCompile(`href="([^"]+)"`)

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

// ValidateRenderedHTML parses every generated page and verifies the structural
// landmarks required by browsers and assistive technology.
func ValidateRenderedHTML(root string) error {
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".html" {
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		document, parseErr := html.Parse(file)
		closeErr := file.Close()
		if parseErr != nil {
			return fmt.Errorf("render %s: %w", path, parseErr)
		}
		if closeErr != nil {
			return closeErr
		}
		found := map[string]bool{}
		var visit func(*html.Node)
		visit = func(node *html.Node) {
			if node.Type == html.ElementNode {
				found[node.Data] = true
			}
			for child := node.FirstChild; child != nil; child = child.NextSibling {
				visit(child)
			}
		}
		visit(document)
		for _, element := range []string{"html", "head", "title", "body", "header", "main", "h1"} {
			if !found[element] {
				return fmt.Errorf("rendered page %s is missing <%s>", path, element)
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

// TreeDigest returns a stable digest of every relative path and byte in a
// generated portal tree. It excludes filesystem metadata and traversal order.
func TreeDigest(root string) (string, error) {
	files, err := treeContents(root)
	if err != nil {
		return "", err
	}
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	digest := sha256.New()
	for _, name := range names {
		_, _ = digest.Write([]byte(name))
		_, _ = digest.Write([]byte{0})
		_, _ = digest.Write([]byte(files[name]))
		_, _ = digest.Write([]byte{0})
	}
	return fmt.Sprintf("%x", digest.Sum(nil)), nil
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
