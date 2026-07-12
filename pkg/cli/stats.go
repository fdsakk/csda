package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/akiver/cs-demo-analyzer/pkg/api"
	"github.com/akiver/cs-demo-analyzer/pkg/api/constants"
)

type stringListFlag []string

func (values *stringListFlag) String() string         { return strings.Join(*values, ",") }
func (values *stringListFlag) Set(value string) error { *values = append(*values, value); return nil }

type statsFlags struct {
	database     string
	demos        stringListFlag
	demoDirs     stringListFlag
	recursive    bool
	jobs         int
	source       string
	force        bool
	confirmTicks int
	output       string
	format       string
	configPath   string
}

func addBuildFlags(fs *flag.FlagSet, values *statsFlags) {
	fs.StringVar(&values.database, "db", "player-stats.db", "SQLite player statistics database path")
	fs.Var(&values.demos, "demo", "Demo file path; may be repeated")
	fs.Var(&values.demoDirs, "demo-dir", "Folder containing demos; may be repeated")
	fs.BoolVar(&values.recursive, "recursive", true, "Scan demo folders recursively")
	fs.IntVar(&values.jobs, "jobs", 4, "Number of demos analyzed concurrently")
	fs.StringVar(&values.source, "source", "", "Force demo source")
	fs.BoolVar(&values.force, "force", false, "Re-analyze demos already present in the database")
	fs.IntVar(&values.confirmTicks, "visibility-confirmation-ticks", 3, "Consecutive spotted ticks required to confirm exposure")
}

func addReportFlags(fs *flag.FlagSet, values *statsFlags) {
	fs.StringVar(&values.output, "output", "stats-report", "Report output folder")
	fs.StringVar(&values.format, "format", "csv", "Report format: csv or json")
	fs.StringVar(&values.configPath, "config", "", "Optional JSON suspicion threshold configuration")
}

func (values statsFlags) buildOptions() api.PlayerStatsBuildOptions {
	return api.PlayerStatsBuildOptions{
		DatabasePath: values.database, DemoPaths: values.demos, DemoDirectories: values.demoDirs,
		Recursive: values.recursive, Jobs: values.jobs, Source: constants.DemoSource(values.source),
		Force: values.force, VisibilityConfirmationTicks: values.confirmTicks,
	}
}

func (values statsFlags) reportOptions() (api.PlayerStatsReportOptions, error) {
	config := api.DefaultSuspicionConfig()
	if values.configPath != "" {
		data, err := os.ReadFile(values.configPath)
		if err != nil {
			return api.PlayerStatsReportOptions{}, err
		}
		if err := json.Unmarshal(data, &config); err != nil {
			return api.PlayerStatsReportOptions{}, fmt.Errorf("invalid suspicion config: %w", err)
		}
	}
	return api.PlayerStatsReportOptions{DatabasePath: values.database, OutputPath: values.output, Format: constants.ExportFormat(values.format), Config: config}, nil
}

func runStats(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: csda stats <ingest|report|build> [options]")
		return 2
	}
	command := args[0]
	values := statsFlags{}
	fs := flag.NewFlagSet("csda stats "+command, flag.ContinueOnError)
	switch command {
	case "ingest":
		addBuildFlags(fs, &values)
	case "report":
		fs.StringVar(&values.database, "db", "player-stats.db", "SQLite player statistics database path")
		addReportFlags(fs, &values)
	case "build":
		addBuildFlags(fs, &values)
		addReportFlags(fs, &values)
	default:
		fmt.Fprintf(os.Stderr, "unknown stats command %q; valid commands: ingest, report, build\n", command)
		return 2
	}
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	if values.source != "" {
		if err := api.ValidateDemoSource(constants.DemoSource(values.source)); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 2
		}
	}
	ctx := context.Background()
	failed := false
	if command == "ingest" || command == "build" {
		result, err := api.BuildPlayerStatsDatabase(ctx, values.buildOptions())
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		fmt.Printf("Imported: %d, skipped: %d, failed: %d\n", result.Imported, result.Skipped, result.Failed)
		for _, item := range result.Errors {
			fmt.Fprintf(os.Stderr, "%s: %s\n", item.Path, item.Error)
		}
		failed = result.Failed > 0
	}
	if command == "report" || command == "build" {
		options, err := values.reportOptions()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if err := api.ExportPlayerStatsReport(ctx, options); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		fmt.Println("Report exported to " + values.output)
	}
	if failed {
		return 1
	}
	return 0
}
