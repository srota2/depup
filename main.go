package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"

	"depup/analyzers"

	"github.com/urfave/cli/v3"
)

func main() {
	cmd := &cli.Command{
		Name:     "depup",
		Usage:    "Show how long ago each dependency was last updated",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "output",
				Aliases: []string{"o"},
				Usage: "Output format: text or json",
				Value: "text",
			},
		},
		Commands: buildSubcommands(),
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}

// buildSubcommands creates a cli subcommand for every registered analyzer.
func buildSubcommands() []*cli.Command {
	names := make([]string, 0, len(analyzers.Registry))
	for name := range analyzers.Registry {
		names = append(names, name)
	}
	sort.Strings(names)

	cmds := make([]*cli.Command, 0, len(names))
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

		outputFormat := cmd.Root().String("output")
		switch outputFormat {
		case "json":
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			if err := enc.Encode(results); err != nil {
				return fmt.Errorf("failed to encode JSON: %w", err)
			}
		case "text":
			const nameWidth = 75
			const totalWidth = nameWidth + 1 + 4 // space + "DAYS"
			fmt.Printf("%-*s %s\n", nameWidth, "DEPENDENCY", "DAYS")
			fmt.Println(strings.Repeat("-", totalWidth))
			for _, dep := range results {
				name := dep.Name
				if len(name) > nameWidth {
					name = name[:nameWidth-3] + "..."
				}
				fmt.Printf("%-*s %4d\n", nameWidth, name, dep.Days)
			}
		default:
			return fmt.Errorf("unknown output format %q (supported: text, json)", outputFormat)
		}
		return nil
	}
}
