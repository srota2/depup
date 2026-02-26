package analyzers

import "encoding/json"

// NpmAnalyzer implements Analyzer for npm's package-lock.json.
type NpmAnalyzer struct{}

func init() {
	RegisterAnalyzer("npm", &NpmAnalyzer{})
}

func (n *NpmAnalyzer) LockFileName() string {
	return "package-lock.json"
}

func (n *NpmAnalyzer) FallbackFileName() string {
	return "package.json"
}

func (n *NpmAnalyzer) ParseFallbackDeps(data []byte) (map[string]string, error) {
	return parsePackageJSON(data)
}

// packageLockJSON represents the top-level structure of package-lock.json (v2/v3).
type packageLockJSON struct {
	Packages map[string]struct {
		Version string `json:"version"`
	} `json:"packages"`
	// v1 fallback
	Dependencies map[string]struct {
		Version string `json:"version"`
	} `json:"dependencies"`
}

func (n *NpmAnalyzer) ParseDeps(data []byte) (map[string]string, error) {
	var lock packageLockJSON
	if err := json.Unmarshal(data, &lock); err != nil {
		return nil, err
	}

	deps := make(map[string]string)

	// v2/v3: "packages" map — keys are paths like "" or "node_modules/foo"
	if len(lock.Packages) > 0 {
		for key, pkg := range lock.Packages {
			if key == "" || pkg.Version == "" {
				continue
			}
			name := extractPkgName(key)
			if name != "" {
				deps[name] = pkg.Version
			}
		}
	}

	// v1 fallback: "dependencies" map
	if len(deps) == 0 && len(lock.Dependencies) > 0 {
		for name, dep := range lock.Dependencies {
			if dep.Version != "" {
				deps[name] = dep.Version
			}
		}
	}

	return deps, nil
}

// extractPkgName extracts the npm package name from a node_modules path.
// E.g. "node_modules/@scope/pkg" → "@scope/pkg", "node_modules/foo" → "foo".
func extractPkgName(path string) string {
	const prefix = "node_modules/"
	// Find the last occurrence of "node_modules/"
	last := -1
	for i := 0; i+len(prefix) <= len(path); i++ {
		if path[i:i+len(prefix)] == prefix {
			last = i
		}
	}
	if last < 0 {
		return ""
	}
	name := path[last+len(prefix):]
	if name == "" {
		return ""
	}
	return name
}

// packageJSON represents the top-level structure of package.json.
type packageJSON struct {
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

// parsePackageJSON parses a package.json file and extracts dependencies.
// Used as fallback by both npm and yarn analyzers.
func parsePackageJSON(data []byte) (map[string]string, error) {
	var pkg packageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, err
	}

	deps := make(map[string]string)
	for name, version := range pkg.Dependencies {
		if name != "" && version != "" {
			deps[name] = version
		}
	}
	for name, version := range pkg.DevDependencies {
		if name != "" && version != "" {
			deps[name] = version
		}
	}

	return deps, nil
}
