package analyzers

import (
	"encoding/xml"
	"fmt"
)

// PomAnalyzer implements Analyzer for Maven's pom.xml.
type PomAnalyzer struct{}

func init() {
	RegisterAnalyzer("pom", &PomAnalyzer{})
}

func (p *PomAnalyzer) LockFileName() string {
	return "pom.xml"
}

// pomProject represents the top-level <project> element in a pom.xml.
type pomProject struct {
	XMLName              xml.Name      `xml:"project"`
	Dependencies         []pomDep      `xml:"dependencies>dependency"`
	DependencyManagement []pomDep      `xml:"dependencyManagement>dependencies>dependency"`
	Properties           pomProperties `xml:"properties"`
}

// pomDep represents a single <dependency> element.
type pomDep struct {
	GroupID    string `xml:"groupId"`
	ArtifactID string `xml:"artifactId"`
	Version    string `xml:"version"`
}

// pomProperties holds the raw XML content of <properties> for variable resolution.
type pomProperties struct {
	Inner []byte `xml:",innerxml"`
}

func (p *PomAnalyzer) ParseDeps(data []byte) (map[string]string, error) {
	var project pomProject
	if err := xml.Unmarshal(data, &project); err != nil {
		return nil, err
	}

	// Build properties map for ${...} variable resolution
	props := parseProperties(project.Properties.Inner)

	deps := make(map[string]string)

	// Collect from <dependencies>
	for _, d := range project.Dependencies {
		addPomDep(deps, d, props)
	}

	// Collect from <dependencyManagement>
	for _, d := range project.DependencyManagement {
		addPomDep(deps, d, props)
	}

	return deps, nil
}

// addPomDep adds a dependency to the map, resolving ${...} property references
// in groupId, artifactId, and version.
func addPomDep(deps map[string]string, d pomDep, props map[string]string) {
	groupID := resolveProperty(d.GroupID, props)
	artifactID := resolveProperty(d.ArtifactID, props)
	version := resolveProperty(d.Version, props)
	if groupID == "" || artifactID == "" || version == "" {
		return
	}
	key := fmt.Sprintf("%s:%s", groupID, artifactID)
	deps[key] = version
}

// parseProperties parses the raw inner XML of <properties> into a string map.
func parseProperties(inner []byte) map[string]string {
	props := make(map[string]string)
	if len(inner) == 0 {
		return props
	}

	// Wrap in a root element for valid XML parsing
	wrapped := append([]byte("<p>"), inner...)
	wrapped = append(wrapped, []byte("</p>")...)

	type prop struct {
		XMLName xml.Name
		Value   string `xml:",chardata"`
	}
	type wrapper struct {
		Props []prop `xml:",any"`
	}

	var w wrapper
	if err := xml.Unmarshal(wrapped, &w); err != nil {
		return props
	}

	for _, p := range w.Props {
		props[p.XMLName.Local] = p.Value
	}
	return props
}

// resolveProperty replaces ${property.name} with the value from properties.
// Handles simple single-level references only.
func resolveProperty(value string, props map[string]string) string {
	if len(value) < 4 || value[0] != '$' || value[1] != '{' || value[len(value)-1] != '}' {
		return value
	}
	propName := value[2 : len(value)-1]
	if resolved, ok := props[propName]; ok {
		return resolved
	}
	// Return the unresolved property reference as-is (still useful for tracking changes)
	return value
}
