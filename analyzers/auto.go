package analyzers

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// AutoAnalyze iterates through all registered analyzers, skipping those whose
// lock file does not exist in dir, and returns the merged results sorted by
// days descending.
// When both "yarn" and "npm" analyzers produce results, only yarn is kept
// (yarn.lock is the authoritative lock file in yarn-based projects).
func AutoAnalyze(dir string) ([]DepAge, error) {
	var merged = make([]DepAge, 0)

	// Sort analyzer names for deterministic ordering
	names := make([]string, 0, len(Registry))
	for name := range Registry {
		names = append(names, name)
	}
	sort.Strings(names)

	// Collect results per analyzer so we can decide which to keep
	resultsByName := make(map[string][]DepAge)

	for _, name := range names {
		analyzer := Registry[name]
		lockPath := filepath.Join(dir, analyzer.LockFileName())

		if _, err := os.Stat(lockPath); err != nil {
			// Lock file does not exist — skip this analyzer
			continue
		}

		results, err := AnalyzeDeps(dir, analyzer)
		if err != nil {
			return nil, fmt.Errorf("analyzer %s failed: %w", name, err)
		}
		if len(results) > 0 {
			resultsByName[name] = results
		}
	}

	// If both yarn and npm produced results, prefer yarn
	if _, hasYarn := resultsByName["yarn"]; hasYarn {
		delete(resultsByName, "npm")
	}

	for _, name := range names {
		if results, ok := resultsByName[name]; ok {
			merged = append(merged, results...)
		}
	}

	sort.Slice(merged, func(i, j int) bool {
		return merged[i].Days > merged[j].Days
	})

	return merged, nil
}
