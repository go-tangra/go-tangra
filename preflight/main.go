package preflight

import (
	"context"
	"flag"
	"fmt"
	"io"
)

// Plan builds a module's checks from its configuration file. It loads and
// validates the configuration itself and turns each problem into a (Static)
// check, so a broken configuration still yields a full report.
type Plan func(ctx context.Context, configPath string) []Check

// Main implements `<module> preflight -config <path> [-json]`: it builds the
// checks with plan, runs them and writes the report to stdout. It returns
// the exit code: 0 when nothing failed, 1 when a check failed, 2 on a usage
// error.
func Main(ctx context.Context, module string, args []string, stdout, stderr io.Writer, defaultConfig string, plan Plan) int {
	fs := flag.NewFlagSet(module+" preflight", flag.ContinueOnError)
	fs.SetOutput(stderr)
	path := fs.String("config", defaultConfig, "configuration file")
	asJSON := fs.Bool("json", false, "write the report as JSON")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() > 0 {
		_, _ = fmt.Fprintf(stderr, "%s preflight: unexpected arguments %v\n", module, fs.Args())
		return 2
	}
	rep := Report{Module: module, Config: *path, Results: Run(ctx, plan(ctx, *path))}
	write := rep.WriteText
	if *asJSON {
		write = rep.WriteJSON
	}
	if err := write(stdout); err != nil {
		_, _ = fmt.Fprintf(stderr, "%s preflight: %v\n", module, err)
		return 1
	}
	return rep.ExitCode()
}
