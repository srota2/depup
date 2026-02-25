package analyzers

import (
	"strings"
)

// YarnAnalyzer implements Analyzer for Yarn's yarn.lock.
type YarnAnalyzer struct{}

func init() {
	RegisterAnalyzer("yarn", &YarnAnalyzer{})
}

func (y *YarnAnalyzer) LockFileName() string {
	return "yarn.lock"
}

func (y *YarnAnalyzer) ParseDeps(data []byte) (map[string]string, error) {
	deps := make(map[string]string)
	lines := strings.Split(string(data), "\n")

	var currentNames []string

	for _, line := range lines {
		// Skip comments and empty lines
		if strings.HasPrefix(strings.TrimSpace(line), "#") || strings.TrimSpace(line) == "" {
			continue
		}

		// A top-level entry (not indented) starts a new dependency block.
		// yarn.lock v1: "pkg@^1.0.0", "pkg@~2.0.0":
		// yarn.lock v2: "pkg@npm:^1.0.0":
		if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			currentNames = parseYarnEntryNames(line)
			continue
		}

		// Indented line inside a block — look for the version field
		trimmed := strings.TrimSpace(line)

		// v1 format:   version "1.2.3"
		if strings.HasPrefix(trimmed, "version ") {
			version := strings.TrimPrefix(trimmed, "version ")
			version = strings.Trim(version, "\"")
			for _, name := range currentNames {
				deps[name] = version
			}
			currentNames = nil
			continue
		}

		// v2 (berry) format:   version: 1.2.3
		if strings.HasPrefix(trimmed, "version: ") {
			version := strings.TrimPrefix(trimmed, "version: ")
			version = strings.Trim(version, "\"")
			for _, name := range currentNames {
				deps[name] = version
			}
			currentNames = nil
			continue
		}
	}

	return deps, nil
}

// parseYarnEntryNames extracts the package name(s) from a yarn.lock header line.
// Examples:
//
//	"@babel/core@^7.0.0", "@babel/core@^7.1.0":  →  ["@babel/core"]
//	lodash@^4.17.0:                               →  ["lodash"]
//	"pkg@npm:^1.0.0":                             →  ["pkg"]
func parseYarnEntryNames(line string) []string {
	line = strings.TrimSuffix(strings.TrimSpace(line), ":")

	// The header may list multiple version constraints separated by ", "
	parts := strings.Split(line, ", ")

	seen := make(map[string]bool)
	var names []string

	for _, part := range parts {
		part = strings.Trim(part, "\"")
		name := extractYarnPkgName(part)
		if name != "" && !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}

	return names
}

// extractYarnPkgName extracts the package name from a spec like
// "@scope/pkg@^1.0.0" or "pkg@npm:^1.0.0" → "@scope/pkg" or "pkg".
func extractYarnPkgName(spec string) string {
	// Handle scoped packages: @scope/pkg@version
	if strings.HasPrefix(spec, "@") {
		// Find the second '@' which separates name from version constraint
		idx := strings.Index(spec[1:], "@")
		if idx < 0 {
			return spec // no version constraint
		}
		return spec[:idx+1]
	}

	// Unscoped: pkg@version
	idx := strings.Index(spec, "@")
	if idx < 0 {
		return spec
	}
	return spec[:idx]
}
