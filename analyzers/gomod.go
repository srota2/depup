package analyzers

import (
	"strings"
)

// GoModAnalyzer implements Analyzer for Go's go.mod.
type GoModAnalyzer struct{}

func init() {
	RegisterAnalyzer("gomod", &GoModAnalyzer{})
}

func (g *GoModAnalyzer) LockFileName() string {
	return "go.mod"
}

func (g *GoModAnalyzer) ParseDeps(data []byte) (map[string]string, error) {
	deps := make(map[string]string)
	lines := strings.Split(string(data), "\n")

	inRequireBlock := false

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}

		// Detect require block start/end
		if strings.HasPrefix(line, "require (") || line == "require (" {
			inRequireBlock = true
			continue
		}
		if inRequireBlock && line == ")" {
			inRequireBlock = false
			continue
		}

		// Single-line require: require module/path v1.2.3
		if strings.HasPrefix(line, "require ") && !inRequireBlock {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				mod := parts[1]
				ver := parts[2]
				if !isIndirect(line) {
					deps[mod] = ver
				}
			}
			continue
		}

		// Inside require block: module/path v1.2.3
		if inRequireBlock {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				mod := parts[0]
				ver := parts[1]
				if !isIndirect(line) {
					deps[mod] = ver
				}
			}
		}
	}

	return deps, nil
}

// isIndirect checks if a go.mod require line has the // indirect comment.
func isIndirect(line string) bool {
	return strings.Contains(line, "// indirect")
}
