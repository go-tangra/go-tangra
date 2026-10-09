package preflight

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestRunKeepsOrderAndNames(t *testing.T) {
	checks := []Check{
		{Name: "slow", Run: func(ctx context.Context) Result { time.Sleep(20 * time.Millisecond); return Passf("ok") }},
		Static("static", Warnf("careful").WithFix("do %s", "this")),
		{Name: "panics", Run: func(context.Context) Result { panic("boom") }},
		{Name: "no status", Run: func(context.Context) Result { return Result{Detail: "?"} }},
		{Name: "named", Run: func(context.Context) Result { return Result{Name: "ignored", Status: Skip} }},
	}
	got := Run(context.Background(), checks)
	want := []struct {
		name   string
		status Status
	}{{"slow", Pass}, {"static", Warn}, {"panics", Fail}, {"no status", Fail}, {"named", Skip}}
	if len(got) != len(want) {
		t.Fatalf("got %d results", len(got))
	}
	for i, w := range want {
		if got[i].Name != w.name || got[i].Status != w.status {
			t.Errorf("result %d = %+v, want %s %s", i, got[i], w.name, w.status)
		}
	}
	if got[1].Fix != "do this" || !strings.Contains(got[2].Detail, "boom") {
		t.Fatalf("fix/panic detail: %+v %+v", got[1], got[2])
	}
}

func TestRunBoundsEachCheck(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	got := Run(ctx, []Check{{Name: "waits", Run: func(ctx context.Context) Result {
		<-ctx.Done()
		return Failf("%v", ctx.Err())
	}}})
	if got[0].Status != Fail {
		t.Fatalf("got %+v", got[0])
	}
}

func sample() Report {
	return Report{Module: "demo", Config: "c.yaml", Results: []Result{
		{Name: "config", Status: Pass, Detail: "loaded"},
		{Name: "token: expiry", Status: Fail, Detail: "expired 3 min ago", Fix: "mint one"},
		{Name: "db", Status: Warn, Detail: "slow"},
		{Name: "x", Status: Skip, Detail: "n/a"},
	}}
}

func TestReportSummaryAndExitCode(t *testing.T) {
	r := sample()
	if s := r.Summary(); s != (Summary{Pass: 1, Warn: 1, Fail: 1, Skip: 1}) {
		t.Fatalf("summary %+v", s)
	}
	if r.OK() || r.ExitCode() != 1 {
		t.Fatal("a FAIL must fail the report")
	}
	r.Results[1].Status = Warn
	if !r.OK() || r.ExitCode() != 0 {
		t.Fatal("warnings must not fail the report")
	}
	if (Report{Results: []Result{{Status: "BOGUS"}}}).OK() {
		t.Fatal("an unknown status counts as a failure")
	}
}

func TestWriteText(t *testing.T) {
	var b bytes.Buffer
	if err := sample().WriteText(&b); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	for _, want := range []string{
		"demo preflight (c.yaml)",
		"  PASS  config         loaded\n",
		"  FAIL  token: expiry  expired 3 min ago\n",
		"                       fix: mint one\n",
		"FAILED: 1 passed, 1 warnings, 1 failed, 1 skipped",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("text output lacks %q:\n%s", want, out)
		}
	}
}

func TestWriteJSON(t *testing.T) {
	var b bytes.Buffer
	if err := sample().WriteJSON(&b); err != nil {
		t.Fatal(err)
	}
	var got struct {
		Module  string   `json:"module"`
		OK      bool     `json:"ok"`
		Summary Summary  `json:"summary"`
		Results []Result `json:"results"`
	}
	if err := json.Unmarshal(b.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Module != "demo" || got.OK || got.Summary.Fail != 1 || len(got.Results) != 4 || got.Results[1].Fix != "mint one" {
		t.Fatalf("json: %+v", got)
	}
	if strings.Contains(b.String(), `"fix": ""`) {
		t.Fatal("empty fix must be omitted")
	}
}

func TestMainCommand(t *testing.T) {
	plan := func(_ context.Context, path string) []Check {
		st := Pass
		if path == "bad.yaml" {
			st = Fail
		}
		return []Check{Static("config", Result{Status: st, Detail: path})}
	}
	cases := []struct {
		name string
		args []string
		code int
		out  string
	}{
		{"default config passes", nil, 0, "PASS  config  def.yaml"},
		{"failing check", []string{"-config", "bad.yaml"}, 1, "FAIL  config  bad.yaml"},
		{"json", []string{"-json", "-config", "x.yaml"}, 0, `"ok": true`},
		{"unknown flag", []string{"-nope"}, 2, ""},
		{"stray argument", []string{"extra"}, 2, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			code := Main(context.Background(), "demo", tc.args, &out, &errOut, "def.yaml", plan)
			if code != tc.code {
				t.Fatalf("exit %d, want %d (stderr %q)", code, tc.code, errOut.String())
			}
			if !strings.Contains(out.String(), tc.out) {
				t.Fatalf("stdout %q lacks %q", out.String(), tc.out)
			}
		})
	}
}

func TestHuman(t *testing.T) {
	cases := map[time.Duration]string{
		12 * time.Second:     "12 s",
		-47 * time.Minute:    "47 min",
		90 * time.Minute:     "90 min",
		5 * time.Hour:        "5 h",
		10 * 24 * time.Hour:  "10 days",
		119*time.Minute + 59: "119 min",
	}
	for d, want := range cases {
		if got := human(d); got != want {
			t.Errorf("human(%s) = %q, want %q", d, got, want)
		}
	}
}

// Offline skips only the network checks, in place; everything else runs.
func TestOffline(t *testing.T) {
	contacted := false
	checks := []Check{
		Static("config", Passf("ok")),
		{Name: "reach: auth", Network: true, Run: func(context.Context) Result { contacted = true; return Failf("refused") }},
		Static("file", Failf("missing")),
	}
	got := Run(context.Background(), Offline(checks))
	if contacted {
		t.Fatal("an offline run must not contact other hosts")
	}
	want := []Status{Pass, Skip, Fail}
	for i, r := range got {
		if r.Status != want[i] || r.Name != checks[i].Name {
			t.Fatalf("result %d = %+v, want %s %s", i, r, checks[i].Name, want[i])
		}
	}
	if !strings.Contains(got[1].Detail, "offline") {
		t.Fatalf("skip detail %q", got[1].Detail)
	}
	if !TCPDial("x", "127.0.0.1:1", time.Second).Network || !HTTPSProbe("y", "https://127.0.0.1:1/", nil, time.Second).Network {
		t.Fatal("the framework's dial and HTTPS checks are network checks")
	}
}

func TestMainOffline(t *testing.T) {
	plan := func(context.Context, string) []Check {
		return []Check{Static("config", Passf("ok")), {Name: "reach: lcm", Network: true, Run: func(context.Context) Result { return Failf("refused") }}}
	}
	var out, errOut bytes.Buffer
	if code := Main(context.Background(), "demo", []string{"-offline"}, &out, &errOut, "def.yaml", plan); code != 0 {
		t.Fatalf("exit %d with only a network failure offline: %s", code, out.String())
	}
	if !strings.Contains(out.String(), "SKIP  reach: lcm") {
		t.Fatalf("stdout %q", out.String())
	}
	out.Reset()
	if code := Main(context.Background(), "demo", nil, &out, &errOut, "def.yaml", plan); code != 1 {
		t.Fatalf("online exit %d, want 1", code)
	}
}
