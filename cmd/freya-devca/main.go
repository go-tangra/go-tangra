// Command freya-devca writes a throw-away CA and per-service SVIDs for local
// development and the quickstart. Never use its output outside a workstation.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/go-tangra/go-tangra/v4/freyatest/testutil"
)

func main() {
	out := flag.String("out", ".dev/ca", "output directory")
	td := flag.String("trust-domain", "example.org", "trust domain")
	services := flag.String("services", "orders,inventory", "comma-separated service names")
	ttl := flag.Duration("ttl", 24*time.Hour, "SVID lifetime")
	flag.Parse()

	ca, err := testutil.NewCA(*td)
	if err != nil {
		fail(err)
	}
	for _, name := range strings.Split(*services, ",") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		crt, err := ca.Issue(name, testutil.IssueOptions{NotAfter: time.Now().Add(*ttl)})
		if err != nil {
			fail(err)
		}
		c, k, b, err := ca.WriteSVID(*out, name, crt)
		if err != nil {
			fail(err)
		}
		fmt.Printf("wrote %s %s (bundle %s)\n", c, k, b)
	}
	fmt.Fprintln(os.Stderr, "WARNING: development CA only; never deploy these files")
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "freya-devca:", err)
	os.Exit(1)
}
