package integration

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/go-freya/freya"
	"github.com/go-freya/freya/config"
	"github.com/go-freya/freya/internal/testutil"
)

var (
	binDir       string
	inventoryBin string
	ordersBin    string
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "freya-int-")
	if err != nil {
		panic(err)
	}
	binDir = dir
	inventoryBin = filepath.Join(dir, "inventory")
	ordersBin = filepath.Join(dir, "orders")
	for _, b := range []struct{ out, pkg string }{{inventoryBin, "../../examples/two-services/inventory"}, {ordersBin, "../../examples/two-services/orders"}} {
		cmd := exec.Command("go", "build", "-o", b.out, b.pkg)
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintln(os.Stderr, "build failed:", err)
			os.Exit(1)
		}
	}
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

// fixture holds a CA plus on-disk SVIDs for named services.
type fixture struct {
	t   *testing.T
	ca  *testutil.CA
	dir string
}

func newFixture(t *testing.T, services ...string) *fixture {
	t.Helper()
	f := &fixture{t: t, ca: testutil.MustCA("example.org"), dir: t.TempDir()}
	for _, s := range services {
		f.issue(s)
	}
	return f
}

func (f *fixture) issue(name string) {
	crt := f.ca.MustIssue(name, testutil.IssueOptions{})
	if _, _, _, err := f.ca.WriteSVID(f.dir, name, crt); err != nil {
		f.t.Fatal(err)
	}
}

func (f *fixture) config(name string) config.Config {
	cfg := config.Default()
	cfg.ServiceName = name
	cfg.TrustDomain = "example.org"
	cfg.Env = "test"
	cfg.Identity.Provider = config.ProviderFile
	cfg.Identity.File = config.FileIdentity{
		Cert: filepath.Join(f.dir, name+".pem"), Key: filepath.Join(f.dir, name+".key"), Bundle: filepath.Join(f.dir, "ca.pem"),
	}
	cfg.Server.GRPCAddr = "127.0.0.1:0"
	cfg.Admin.Addr = "127.0.0.1:0"
	return cfg
}

func (f *fixture) writeYAML(name string, cfg config.Config, policy string) string {
	// Kept deliberately simple: the example binaries read YAML through config.Load.
	body := fmt.Sprintf(`service_name: %s
trust_domain: example.org
env: test
identity:
  provider: file
  file:
    cert: %s
    key: %s
    bundle: %s
authz:
  source: file
  path: %s
server:
  grpc_addr: 127.0.0.1:0
admin:
  addr: 127.0.0.1:0
`, cfg.ServiceName, cfg.Identity.File.Cert, cfg.Identity.File.Key, cfg.Identity.File.Bundle, policy)
	p := filepath.Join(f.dir, name+".yaml")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		f.t.Fatal(err)
	}
	return p
}

func (f *fixture) writePolicy(name, body string) string {
	p := filepath.Join(f.dir, name)
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		f.t.Fatal(err)
	}
	return p
}

// startApp builds and runs an in-process App for name; returns it and its gRPC host:port.
func (f *fixture) startApp(t *testing.T, cfg config.Config, opts ...freya.Option) (*freya.App, string, context.CancelFunc) {
	t.Helper()
	app, err := freya.New(cfg, opts...)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = app.Run(ctx) }()
	ep, err := app.GRPC().Endpoint()
	if err != nil {
		t.Fatal(err)
	}
	stop := func() { cancel(); app.Close() }
	// Wait until the listener accepts.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		c, err := net.DialTimeout("tcp", ep.Host, 200*time.Millisecond)
		if err == nil {
			c.Close()
			return app, ep.Host, stop
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("server did not accept connections")
	return nil, "", stop
}

// proxy is a TCP relay that can record and tamper with traffic.
type proxy struct {
	ln       net.Listener
	target   string
	mu       sync.Mutex
	c2s, s2c []byte
	tamper   bool
	flipped  bool
}

func newProxy(t *testing.T, target string, tamper bool) *proxy {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	p := &proxy{ln: ln, target: target, tamper: tamper}
	go p.serve()
	t.Cleanup(func() { ln.Close() })
	return p
}

func (p *proxy) addr() string { return p.ln.Addr().String() }

func (p *proxy) serve() {
	for {
		c, err := p.ln.Accept()
		if err != nil {
			return
		}
		go p.handle(c)
	}
}

func (p *proxy) handle(c net.Conn) {
	defer c.Close()
	s, err := net.Dial("tcp", p.target)
	if err != nil {
		return
	}
	defer s.Close()
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); p.pump(c, s, true) }()
	go func() { defer wg.Done(); p.pump(s, c, false) }()
	wg.Wait()
}

func (p *proxy) pump(src io.Reader, dst io.Writer, clientToServer bool) {
	buf := make([]byte, 32*1024)
	for {
		n, err := src.Read(buf)
		if n > 0 {
			chunk := append([]byte(nil), buf[:n]...)
			p.mu.Lock()
			if clientToServer {
				// Flip one byte inside the first client record sent after the server
				// has replied (i.e. inside encrypted handshake/application data).
				if p.tamper && !p.flipped && len(p.s2c) > 0 && len(chunk) > 6 {
					chunk[6] ^= 0xff
					p.flipped = true
				}
				p.c2s = append(p.c2s, chunk...)
			} else {
				p.s2c = append(p.s2c, chunk...)
			}
			p.mu.Unlock()
			if _, werr := dst.Write(chunk); werr != nil {
				return
			}
		}
		if err != nil {
			return
		}
	}
}

func (p *proxy) captured() (c2s, s2c []byte) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]byte(nil), p.c2s...), append([]byte(nil), p.s2c...)
}
