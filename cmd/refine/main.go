// Command refine is a fast, concurrent line-deduplication and sorting tool.
//
// main is intentionally tiny: parse flags into a Config, configure styling,
// hand off to the engine, and render the result. All real logic lives in the
// internal packages so it can be tested and benchmarked in isolation.
package main

import (
	"fmt"
	"os"

	"github.com/yourpwnguy/refine/internal/cli"
	"github.com/yourpwnguy/refine/internal/engine"
	"github.com/yourpwnguy/refine/internal/report"
	"github.com/yourpwnguy/refine/internal/style"
	"github.com/yourpwnguy/refine/internal/term"
)

// main is the CLI entry point. It parses arguments into a Config, configures styling based
// on whether stderr is a terminal, hands the work to the engine, and renders the result. All
// real logic lives in the internal packages so this stays a thin wiring layer.
func main() {
	cfg, err := cli.Parse(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, style.Red("refine: ")+err.Error())
		os.Exit(2)
	}

	// Color only when stderr is a terminal and the user hasn't opted out.
	style.Configure(cfg.NoColor, term.IsTerminal(os.Stderr))

	if cfg.Help {
		cli.PrintHelp()
		return
	}
	if cfg.Version {
		cli.PrintVersion()
		return
	}

	summary, err := engine.Run(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, style.Red("refine: ")+err.Error())
		os.Exit(1)
	}

	// Stats always go to stderr so stdout stays clean for piped line data.
	if cfg.JSON {
		if err := report.RenderJSON(summary, os.Stderr); err != nil {
			fmt.Fprintln(os.Stderr, style.Red("refine: ")+err.Error())
			os.Exit(1)
		}
		return
	}
	report.RenderHuman(summary, os.Stderr)
}
