package analyzers

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// DepAge holds a dependency name, its current version, and the number of days since its last version change.
type DepAge struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Days    int    `json:"days"`
}

// Analyzer defines the interface that each package manager must implement.
type Analyzer interface {
	// LockFileName returns the name of the lock/manifest file tracked in git.
	LockFileName() string

	// ParseDeps extracts dependency name → version from file contents.
	ParseDeps(data []byte) (map[string]string, error)
}

// Registry maps subcommand names to their Analyzer implementations.
var Registry = map[string]Analyzer{}

// RegisterAnalyzer registers an analyzer under a given subcommand name.
func RegisterAnalyzer(name string, a Analyzer) {
	Registry[name] = a
}

// AnalyzeDeps opens the git repo at dir and uses the given Analyzer to parse
// the lock file, walk git history, and return dependencies sorted by days
// since last version change (descending).
func AnalyzeDeps(dir string, analyzer Analyzer) ([]DepAge, error) {
	// 1. Open the git repository
	repo, err := git.PlainOpen(dir)
	if err != nil {
		return nil, fmt.Errorf("not a git repository: %w", err)
	}

	lockFile := analyzer.LockFileName()

	// 2. Parse current lock file from the working tree
	lockPath := filepath.Join(dir, lockFile)
	data, err := os.ReadFile(lockPath)
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %w", lockFile, err)
	}

	currentDeps, err := analyzer.ParseDeps(data)
	if err != nil {
		return nil, fmt.Errorf("cannot parse %s: %w", lockFile, err)
	}

	// 3. Walk git log for lock file changes
	lastChanged, oldestDate := findLastChanged(repo, lockFile, currentDeps, analyzer)

	// 4. Build result sorted by days descending
	now := time.Now()
	results := make([]DepAge, 0, len(currentDeps))
	for name := range currentDeps {
		var days int
		if t, ok := lastChanged[name]; ok {
			days = int(math.Floor(now.Sub(t).Hours() / 24))
		} else if !oldestDate.IsZero() {
			days = int(math.Floor(now.Sub(oldestDate).Hours() / 24))
		}
		results = append(results, DepAge{Name: name, Version: currentDeps[name], Days: days})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Days > results[j].Days
	})

	return results, nil
}

// findLastChanged walks git history and for each current dependency,
// finds the most recent commit date where that dependency's version differs
// from the current version.
func findLastChanged(repo *git.Repository, lockFile string, currentDeps map[string]string, analyzer Analyzer) (map[string]time.Time, time.Time) {
	result := make(map[string]time.Time)
	resolved := make(map[string]bool)
	var oldestDate time.Time

	head, err := repo.Head()
	if err != nil {
		return result, oldestDate
	}

	logIter, err := repo.Log(&git.LogOptions{
		From:     head.Hash(),
		FileName: stringPtr(lockFile),
	})
	if err != nil {
		return result, oldestDate
	}
	defer logIter.Close()

	var prevDeps map[string]string
	var prevDate time.Time
	first := true

	_ = logIter.ForEach(func(c *object.Commit) error {
		if len(resolved) == len(currentDeps) {
			return fmt.Errorf("done")
		}

		commitDeps, err := parseDepsFromCommit(c, lockFile, analyzer)
		if err != nil {
			return nil
		}

		if first {
			prevDeps = commitDeps
			prevDate = c.Author.When
			first = false
			return nil
		}

		for name, currentVer := range currentDeps {
			if resolved[name] {
				continue
			}
			prevVer, prevExists := prevDeps[name]
			oldVer, oldExists := commitDeps[name]

			if prevExists && prevVer == currentVer {
				if !oldExists || oldVer != currentVer {
					result[name] = prevDate
					resolved[name] = true
				}
			}
		}

		prevDeps = commitDeps
		prevDate = c.Author.When
		oldestDate = c.Author.When
		return nil
	})

	for name, currentVer := range currentDeps {
		if resolved[name] {
			continue
		}
		if prevDeps != nil {
			if ver, exists := prevDeps[name]; exists && ver == currentVer {
				result[name] = prevDate
				resolved[name] = true
			}
		}
	}

	return result, oldestDate
}

// parseDepsFromCommit reads and parses the lock file from a specific commit.
func parseDepsFromCommit(c *object.Commit, lockFile string, analyzer Analyzer) (map[string]string, error) {
	tree, err := c.Tree()
	if err != nil {
		return nil, err
	}

	file, err := tree.File(lockFile)
	if err != nil {
		return nil, err
	}

	contents, err := file.Contents()
	if err != nil {
		return nil, err
	}

	return analyzer.ParseDeps([]byte(contents))
}

func stringPtr(s string) *string {
	return &s
}
