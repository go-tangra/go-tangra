package preflight

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

// Status is the outcome of one check.
type Status string

// Check outcomes. Only Fail makes the report fail.
const (
	Pass Status = "PASS"
	Warn Status = "WARN"
	Fail Status = "FAIL"
	Skip Status = "SKIP"
)

// Result is the outcome of one check: a one-line detail and, where the remedy
// is obvious, a fix hint. Neither may contain a secret value.
type Result struct {
	Name   string `json:"name"`
	Status Status `json:"status"`
	Detail string `json:"detail"`
	Fix    string `json:"fix,omitempty"`
}

// Passf builds a PASS result. Passf, Warnf, Failf and Skipf leave the fix
// hint empty (chain WithFix to add one); Run fills in the check's name.
func Passf(format string, a ...any) Result {
	return Result{Status: Pass, Detail: fmt.Sprintf(format, a...)}
}

// Warnf builds a WARN result.
func Warnf(format string, a ...any) Result {
	return Result{Status: Warn, Detail: fmt.Sprintf(format, a...)}
}

// Failf builds a FAIL result.
func Failf(format string, a ...any) Result {
	return Result{Status: Fail, Detail: fmt.Sprintf(format, a...)}
}

// Skipf builds a SKIP result.
func Skipf(format string, a ...any) Result {
	return Result{Status: Skip, Detail: fmt.Sprintf(format, a...)}
}

// WithFix returns r with the fix hint set.
func (r Result) WithFix(format string, a ...any) Result {
	r.Fix = fmt.Sprintf(format, a...)
	return r
}

// Check is one named, read-only probe. Run must honour ctx.
type Check struct {
	Name string
	Run  func(ctx context.Context) Result
}

// Static is a check whose outcome is already known (e.g. a configuration
// error found while building the plan).
func Static(name string, r Result) Check {
	return Check{Name: name, Run: func(context.Context) Result { return r }}
}

// DefaultTimeout bounds each check run by Run.
const DefaultTimeout = 10 * time.Second

// Run executes the checks concurrently, each bounded by DefaultTimeout, and
// returns their results in the order given. A check that panics fails.
func Run(ctx context.Context, checks []Check) []Result {
	out := make([]Result, len(checks))
	var wg sync.WaitGroup
	for i, c := range checks {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if p := recover(); p != nil {
					out[i] = Failf("check panicked: %v", p)
					out[i].Name = c.Name
				}
			}()
			cctx, cancel := context.WithTimeout(ctx, DefaultTimeout)
			defer cancel()
			r := c.Run(cctx)
			r.Name = c.Name
			if r.Status == "" {
				r.Status = Fail
			}
			out[i] = r
		}()
	}
	wg.Wait()
	return out
}

// Report is the full preflight outcome of a module.
type Report struct {
	Module  string   `json:"module"`
	Config  string   `json:"config,omitempty"`
	Results []Result `json:"results"`
}

// Summary counts results by status.
type Summary struct {
	Pass int `json:"pass"`
	Warn int `json:"warn"`
	Fail int `json:"fail"`
	Skip int `json:"skip"`
}

// Summary counts the results by status.
func (r Report) Summary() Summary {
	var s Summary
	for _, res := range r.Results {
		switch res.Status {
		case Pass:
			s.Pass++
		case Warn:
			s.Warn++
		case Skip:
			s.Skip++
		default:
			s.Fail++
		}
	}
	return s
}

// OK reports whether no check failed (warnings do not fail).
func (r Report) OK() bool { return r.Summary().Fail == 0 }

// ExitCode is 0 when OK, 1 otherwise.
func (r Report) ExitCode() int {
	if r.OK() {
		return 0
	}
	return 1
}

// WriteJSON writes the report, its summary and ok flag as indented JSON.
func (r Report) WriteJSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(struct {
		Report
		OK      bool    `json:"ok"`
		Summary Summary `json:"summary"`
	}{r, r.OK(), r.Summary()})
}

// WriteText writes the human-readable checklist.
func (r Report) WriteText(w io.Writer) error {
	var b strings.Builder
	fmt.Fprintf(&b, "%s preflight", r.Module)
	if r.Config != "" {
		fmt.Fprintf(&b, " (%s)", r.Config)
	}
	b.WriteString("\n\n")
	width := 0
	for _, res := range r.Results {
		width = max(width, len(res.Name))
	}
	for _, res := range r.Results {
		fmt.Fprintf(&b, "  %-4s  %-*s  %s\n", res.Status, width, res.Name, res.Detail)
		if res.Fix != "" {
			fmt.Fprintf(&b, "        %-*s  fix: %s\n", width, "", res.Fix)
		}
	}
	s := r.Summary()
	verdict := "OK"
	if !r.OK() {
		verdict = "FAILED"
	}
	fmt.Fprintf(&b, "\n%s: %d passed, %d warnings, %d failed, %d skipped\n", verdict, s.Pass, s.Warn, s.Fail, s.Skip)
	_, err := io.WriteString(w, b.String())
	return err
}
