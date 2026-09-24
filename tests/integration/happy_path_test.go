package integration

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/go-tangra/go-tangra/v4/freyatest/testutil"
)

const policyOrdersToInventory = `version: it-1
rules:
  - id: orders-to-inventory
    from: ["spiffe://example.org/svc/orders"]
    to: ["inventory"]
    operations: ["/inventory.v1.Inventory/Reserve"]
    effect: allow
`

func TestHappyPath(t *testing.T) {
	f := newFixture(t, "inventory", "orders")
	policy := f.writePolicy("policy.yaml", policyOrdersToInventory)
	invCfg := f.writeYAML("inventory", f.config("inventory"), policy)
	ordCfg := f.writeYAML("orders", f.config("orders"), policy)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	inv := exec.CommandContext(ctx, inventoryBin, "--config", invCfg)
	invOut := &testutil.LogCapture{}
	inv.Stdout = invOut
	stderr, err := inv.StderrPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := inv.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { cancel(); _ = inv.Wait() }()
	addr := ""
	sc := bufio.NewScanner(stderr)
	deadline := time.After(10 * time.Second)
	for addr == "" {
		got := make(chan string, 1)
		go func() {
			for sc.Scan() {
				if strings.HasPrefix(sc.Text(), "listening grpcs://") {
					got <- strings.TrimPrefix(sc.Text(), "listening grpcs://")
					return
				}
			}
			got <- ""
		}()
		select {
		case addr = <-got:
			if addr == "" {
				t.Fatalf("inventory exited before listening: %s", invOut.String())
			}
		case <-deadline:
			t.Fatal("inventory did not report its endpoint")
		}
	}
	go func() { _, _ = io.Copy(io.Discard, stderr) }()

	ord := exec.CommandContext(ctx, ordersBin, "--config", ordCfg, "--inventory", addr, "--sku", "widget-7")
	out, err := ord.CombinedOutput()
	if err != nil {
		t.Fatalf("orders failed: %v\n%s", err, out)
	}
	cid := ""
	for _, line := range strings.Split(string(out), "\n") {
		var m map[string]any
		if json.Unmarshal([]byte(line), &m) == nil && m["msg"] == "call ok" {
			if m["peer"] != "spiffe://example.org/svc/inventory" || m["reserved_by"] != "orders" {
				t.Fatalf("orders log: %v", m)
			}
			cid, _ = m["x-request-id"].(string)
		}
	}
	if cid == "" {
		t.Fatalf("no 'call ok' line in orders output:\n%s", out)
	}
	// The callee logged the same correlation ID and the verified caller identity.
	deadline2 := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline2) {
		for _, line := range strings.Split(invOut.String(), "\n") {
			var m map[string]any
			if json.Unmarshal([]byte(line), &m) == nil && m["msg"] == "reserve" {
				if m["x-request-id"] != cid || m["peer"] != "spiffe://example.org/svc/orders" || m["sku"] != "widget-7" {
					t.Fatalf("inventory log: %v", m)
				}
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("inventory never logged the reserve:\n%s", invOut.String())
}
