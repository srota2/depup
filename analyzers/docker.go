package analyzers

import (
	"strings"
)

// DockerAnalyzer implements Analyzer for Dockerfiles.
// It tracks image versions from FROM instructions, ignoring images
// that use :latest or have no explicit tag (implicit latest).
type DockerAnalyzer struct{}

func init() {
	RegisterAnalyzer("docker", &DockerAnalyzer{})
}

func (d *DockerAnalyzer) LockFileName() string {
	return "Dockerfile"
}

func (d *DockerAnalyzer) ParseDeps(data []byte) (map[string]string, error) {
	deps := make(map[string]string)
	lines := strings.Split(string(data), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Match FROM instructions (case-insensitive)
		if !strings.HasPrefix(strings.ToUpper(line), "FROM ") {
			continue
		}

		// FROM [--platform=...] image[:tag|@digest] [AS name]
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		// Skip the FROM keyword and any --flags
		imageRef := ""
		for _, f := range fields[1:] {
			if strings.HasPrefix(f, "--") {
				continue
			}
			imageRef = f
			break
		}
		if imageRef == "" {
			continue
		}

		// Handle AS alias — stop before it
		imageRef = strings.ToLower(imageRef)

		// Handle digest references (image@sha256:...) — track them as versioned
		if strings.Contains(imageRef, "@") {
			parts := strings.SplitN(imageRef, "@", 2)
			image := parts[0]
			digest := parts[1]
			if image != "" && digest != "" {
				deps[image] = digest
			}
			continue
		}

		// Handle tag references (image:tag)
		if strings.Contains(imageRef, ":") {
			parts := strings.SplitN(imageRef, ":", 2)
			image := parts[0]
			tag := parts[1]

			// Skip latest tags
			if tag == "latest" || tag == "" {
				continue
			}

			// Skip build args / variable references like ${VAR}
			if strings.Contains(imageRef, "${") || strings.Contains(imageRef, "$") {
				continue
			}

			if image != "" {
				deps[image] = tag
			}
			continue
		}

		// No tag specified → implicit latest → skip
	}

	return deps, nil
}
