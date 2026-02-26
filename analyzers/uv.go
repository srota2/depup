package analyzers

import (
	"strings"
)

// UvAnalyzer implements Analyzer for Python's uv package manager (uv.lock).
type UvAnalyzer struct{}

func init() {
	RegisterAnalyzer("uv", &UvAnalyzer{})
}

func (u *UvAnalyzer) LockFileName() string {
	return "uv.lock"
}

func (u *UvAnalyzer) ParseDeps(data []byte) (map[string]string, error) {
	deps := make(map[string]string)
	lines := strings.Split(string(data), "\n")

	var curName, curVersion string
	var curVirtual bool
	inPackage := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Detect [[package]] section header
		if trimmed == "[[package]]" {
			// Flush the previous package block
			if inPackage {
				flushUvPackage(deps, curName, curVersion, curVirtual)
			}
			curName = ""
			curVersion = ""
			curVirtual = false
			inPackage = true
			continue
		}

		// A new section header (e.g. [options], [[package.wheels]]) that is not
		// [[package]] ends the current package block.
		if strings.HasPrefix(trimmed, "[") {
			if inPackage {
				flushUvPackage(deps, curName, curVersion, curVirtual)
				inPackage = false
			}
			continue
		}

		if !inPackage {
			continue
		}

		// Parse key = value lines inside [[package]]
		key, value, ok := parseTomlKV(trimmed)
		if !ok {
			continue
		}

		switch key {
		case "name":
			curName = value
		case "version":
			curVersion = value
		case "source":
			// Skip the project itself — its source contains "virtual" or "editable"
			if strings.Contains(value, "virtual") || strings.Contains(value, "editable") {
				curVirtual = true
			}
		}
	}

	// Flush the last package block
	if inPackage {
		flushUvPackage(deps, curName, curVersion, curVirtual)
	}

	return deps, nil
}

// flushUvPackage adds a parsed package to deps unless it's the project's own
// virtual/editable source entry.
func flushUvPackage(deps map[string]string, name, version string, virtual bool) {
	if name == "" || version == "" || virtual {
		return
	}
	deps[name] = version
}

// parseTomlKV extracts a bare key = "quoted-value" pair from a TOML line.
// It returns the key, unquoted value, and true on success. For inline tables
// or non-string values the raw value (unquoted) is returned.
func parseTomlKV(line string) (string, string, bool) {
	idx := strings.Index(line, "=")
	if idx < 0 {
		return "", "", false
	}
	key := strings.TrimSpace(line[:idx])
	val := strings.TrimSpace(line[idx+1:])

	// Remove surrounding quotes if present
	if len(val) >= 2 && val[0] == '"' && val[len(val)-1] == '"' {
		val = val[1 : len(val)-1]
	}

	return key, val, true
}
