package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"sort"
	"strings"

	"depup/analyzers"

	"github.com/urfave/cli/v3"
)

func main() {
	cmd := &cli.Command{
		Name:  "depup",
		Usage: "Show how long ago each dependency was last updated",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "format",
				Aliases: []string{"f"},
				Usage:   "Output format: text or json",
				Value:   "text",
			},
			&cli.StringFlag{
				Name:    "output",
				Aliases: []string{"o"},
				Usage:   "Write output to `FILE` instead of stdout",
			},
		},
		Commands: buildSubcommands(),
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}

// buildSubcommands creates a cli subcommand for every registered analyzer,
// plus the special "auto" subcommand that runs all applicable analyzers.
func buildSubcommands() []*cli.Command {
	names := make([]string, 0, len(analyzers.Registry))
	for name := range analyzers.Registry {
		names = append(names, name)
	}
	sort.Strings(names)

	cmds := make([]*cli.Command, 0, len(names)+1)

	// "auto" subcommand — cycles through all analyzers and merges results
	cmds = append(cmds, &cli.Command{
		Name:      "auto",
		Usage:     "Auto-detect and analyze all supported dependency files",
		UsageText: "depup auto <directory>",
		Action:    makeAutoAction(),
	})

	for _, name := range names {
		analyzer := analyzers.Registry[name]
		cmds = append(cmds, &cli.Command{
			Name:      name,
			Usage:     fmt.Sprintf("Analyze %s dependencies", name),
			UsageText: fmt.Sprintf("depup %s <directory>", name),
			Action:    makeAction(analyzer),
		})
	}
	return cmds
}

// makeAction returns the cli action that validates the directory and runs the analysis.
func makeAction(analyzer analyzers.Analyzer) cli.ActionFunc {
	return func(ctx context.Context, cmd *cli.Command) error {
		if cmd.Args().Len() != 1 {
			return fmt.Errorf("expected exactly 1 argument (directory), got %d", cmd.Args().Len())
		}
		dir := cmd.Args().First()

		info, err := os.Stat(dir)
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("directory %q does not exist", dir)
			}
			return fmt.Errorf("cannot access %q: %w", dir, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("%q is not a directory", dir)
		}

		f, err := os.Open(dir)
		if err != nil {
			return fmt.Errorf("directory %q is not readable: %w", dir, err)
		}
		f.Close()

		results, err := analyzers.AnalyzeDeps(dir, analyzer)
		if err != nil {
			return err
		}

		return printResults(results, cmd.Root().String("format"), cmd.Root().String("output"))
	}
}

// makeAutoAction returns the cli action for the "auto" subcommand that runs
// all applicable analyzers and merges results.
func makeAutoAction() cli.ActionFunc {
	return func(ctx context.Context, cmd *cli.Command) error {
		if cmd.Args().Len() != 1 {
			return fmt.Errorf("expected exactly 1 argument (directory), got %d", cmd.Args().Len())
		}
		dir := cmd.Args().First()

		info, err := os.Stat(dir)
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("directory %q does not exist", dir)
			}
			return fmt.Errorf("cannot access %q: %w", dir, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("%q is not a directory", dir)
		}

		results, err := analyzers.AutoAnalyze(dir)
		if err != nil {
			return err
		}

		return printResults(results, cmd.Root().String("format"), cmd.Root().String("output"))
	}
}

// jsonOutput wraps dependency results with summary statistics.
type jsonOutput struct {
	Max     int                `json:"max"`
	Min     int                `json:"min"`
	Med     int                `json:"med"`
	Details []analyzers.DepAge `json:"details"`
}

// printResults outputs the dependency analysis results in the specified format.
// If outputFile is non-empty the output is written to that file; otherwise stdout is used.
func printResults(results []analyzers.DepAge, outputFormat, outputFile string) error {
	var w *os.File
	if outputFile != "" {
		f, err := os.Create(outputFile)
		if err != nil {
			return fmt.Errorf("cannot create output file %q: %w", outputFile, err)
		}
		defer f.Close()
		w = f
	} else {
		w = os.Stdout
	}

	switch outputFormat {
	case "json":
		out := jsonOutput{Details: results}
		if len(results) > 0 {
			sum := 0
			out.Min = results[0].Days
			out.Max = results[0].Days
			for _, d := range results {
				if d.Days > out.Max {
					out.Max = d.Days
				}
				if d.Days < out.Min {
					out.Min = d.Days
				}
				sum += d.Days
			}
			out.Med = int(math.Round(float64(sum) / float64(len(results))))
		}
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		if err := enc.Encode(out); err != nil {
			return fmt.Errorf("failed to encode JSON: %w", err)
		}
	case "text":
		const nameWidth = 55
		const versionWidth = 20
		const totalWidth = nameWidth + 1 + versionWidth + 1 + 4 // spaces + "DAYS"
		fmt.Fprintf(w, "%-*s %-*s %s\n", nameWidth, "DEPENDENCY", versionWidth, "VERSION", "DAYS")
		fmt.Fprintln(w, strings.Repeat("-", totalWidth))
		for _, dep := range results {
			name := dep.Name
			if len(name) > nameWidth {
				name = name[:nameWidth-3] + "..."
			}
			version := dep.Version
			if len(version) > versionWidth {
				version = version[:versionWidth-3] + "..."
			}
			fmt.Fprintf(w, "%-*s %-*s %4d\n", nameWidth, name, versionWidth, version, dep.Days)
		}
		if len(results) > 0 {
			sum := 0
			minDays := results[0].Days
			maxDays := results[0].Days
			for _, d := range results {
				if d.Days > maxDays {
					maxDays = d.Days
				}
				if d.Days < minDays {
					minDays = d.Days
				}
				sum += d.Days
			}
			med := int(math.Round(float64(sum) / float64(len(results))))
			fmt.Fprintln(w, strings.Repeat("-", totalWidth))
			fmt.Fprintf(w, "Max: %d days | Min: %d days | Avg: %d days | Count: %d\n", maxDays, minDays, med, len(results))
		}
	default:
		return fmt.Errorf("unknown output format %q (supported: text, json)", outputFormat)
	}
	return nil
}
