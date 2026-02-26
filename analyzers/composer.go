package analyzers

import (
	"encoding/json"
	"strings"
)

// ComposerAnalyzer implements Analyzer for PHP Composer's composer.lock.
type ComposerAnalyzer struct{}

func init() {
	RegisterAnalyzer("composer", &ComposerAnalyzer{})
}

func (c *ComposerAnalyzer) LockFileName() string {
	return "composer.lock"
}

func (c *ComposerAnalyzer) FallbackFileName() string {
	return "composer.json"
}

func (c *ComposerAnalyzer) ParseFallbackDeps(data []byte) (map[string]string, error) {
	return parseComposerJSON(data)
}

// composerLock represents the top-level structure of composer.lock.
type composerLock struct {
	Packages    []composerPackage `json:"packages"`
	PackagesDev []composerPackage `json:"packages-dev"`
}

// composerPackage represents a single entry in the packages array.
type composerPackage struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

func (c *ComposerAnalyzer) ParseDeps(data []byte) (map[string]string, error) {
	var lock composerLock
	if err := json.Unmarshal(data, &lock); err != nil {
		return nil, err
	}

	deps := make(map[string]string)

	for _, pkg := range lock.Packages {
		if pkg.Name != "" && pkg.Version != "" {
			deps[pkg.Name] = normalizeComposerVersion(pkg.Version)
		}
	}

	for _, pkg := range lock.PackagesDev {
		if pkg.Name != "" && pkg.Version != "" {
			deps[pkg.Name] = normalizeComposerVersion(pkg.Version)
		}
	}

	return deps, nil
}

// normalizeComposerVersion strips the leading "v" prefix that Composer
// conventionally adds to semantic versions (e.g. "v1.2.3" → "1.2.3").
func normalizeComposerVersion(v string) string {
	return strings.TrimPrefix(v, "v")
}

// composerJSON represents the top-level structure of composer.json.
type composerJSON struct {
	Require    map[string]string `json:"require"`
	RequireDev map[string]string `json:"require-dev"`
}

// parseComposerJSON parses a composer.json file and extracts dependencies.
// Platform requirements (e.g. "php", "ext-*") are excluded.
func parseComposerJSON(data []byte) (map[string]string, error) {
	var cj composerJSON
	if err := json.Unmarshal(data, &cj); err != nil {
		return nil, err
	}

	deps := make(map[string]string)
	for name, version := range cj.Require {
		if isComposerPackage(name) && version != "" {
			deps[name] = version
		}
	}
	for name, version := range cj.RequireDev {
		if isComposerPackage(name) && version != "" {
			deps[name] = version
		}
	}

	return deps, nil
}

// isComposerPackage returns true if the name looks like a real Composer package
// (vendor/package format) and not a platform requirement like "php" or "ext-json".
func isComposerPackage(name string) bool {
	return strings.Contains(name, "/")
}
