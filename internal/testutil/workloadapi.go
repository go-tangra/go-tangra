package testutil

import (
	"crypto/x509"
	"errors"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/spiffe/go-spiffe/v2/proto/spiffe/workload"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// WorkloadAPI is a fake SPIFFE Workload API server on a unix socket. It issues
// an X509-SVID for one service with a configurable TTL and re-issues it at
// half-life, like SPIRE does.
type WorkloadAPI struct {
	workload.UnimplementedSpiffeWorkloadAPIServer
	t    *testing.T
	dir  string
	path string
	name string
	ttl  time.Duration
	mu   sync.Mutex
	ca   *CA
	srv  *grpc.Server
	lis  net.Listener
	wg   sync.WaitGroup
}

// StartWorkloadAPI starts a fake agent for trustDomain issuing SVIDs for name.
func StartWorkloadAPI(t *testing.T, trustDomain, name string, ttl time.Duration) *WorkloadAPI {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "wl")
	if err != nil {
		t.Fatal(err)
	}
	w := &WorkloadAPI{t: t, dir: dir, path: filepath.Join(dir, "agent.sock"), name: name, ttl: ttl, ca: MustCA(trustDomain)}
	w.Start()
	t.Cleanup(func() { w.Stop(); _ = os.RemoveAll(dir) })
	return w
}

// Addr returns the unix:// address for workloadapi.WithAddr.
func (w *WorkloadAPI) Addr() string { return "unix://" + w.path }

// CA returns the current issuing CA.
func (w *WorkloadAPI) CA() *CA { w.mu.Lock(); defer w.mu.Unlock(); return w.ca }

// ShareCA makes this agent issue from the same CA as other (one trust domain).
func (w *WorkloadAPI) ShareCA(other *WorkloadAPI) { w.mu.Lock(); w.ca = other.CA(); w.mu.Unlock() }

// RotateCA replaces the CA; the next SVID and bundle come from the new one.
func (w *WorkloadAPI) RotateCA() { w.mu.Lock(); w.ca = MustCA(w.ca.TrustDomain); w.mu.Unlock() }

// Start (re)binds the socket and serves.
func (w *WorkloadAPI) Start() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.srv != nil {
		return
	}
	_ = os.Remove(w.path)
	lis, err := net.Listen("unix", w.path)
	if err != nil {
		w.t.Fatal(err)
	}
	w.lis = lis
	w.srv = grpc.NewServer()
	workload.RegisterSpiffeWorkloadAPIServer(w.srv, w)
	srv := w.srv
	w.wg.Add(1)
	go func() { defer w.wg.Done(); _ = srv.Serve(lis) }()
}

// Stop shuts the agent down; clients see the stream end and fail to reconnect.
func (w *WorkloadAPI) Stop() {
	w.mu.Lock()
	srv := w.srv
	w.srv = nil
	w.mu.Unlock()
	if srv != nil {
		srv.Stop()
		w.wg.Wait()
	}
}

func (w *WorkloadAPI) issue() (*workload.X509SVIDResponse, error) {
	w.mu.Lock()
	ca := w.ca
	ttl := w.ttl
	w.mu.Unlock()
	crt, err := ca.Issue(w.name, IssueOptions{NotBefore: time.Now().Add(-5 * time.Second), NotAfter: time.Now().Add(ttl)})
	if err != nil {
		return nil, err
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(crt.PrivateKey)
	if err != nil {
		return nil, err
	}
	var chain []byte
	for _, der := range crt.Certificate {
		chain = append(chain, der...)
	}
	return &workload.X509SVIDResponse{Svids: []*workload.X509SVID{{
		SpiffeId: crt.Leaf.URIs[0].String(), X509Svid: chain, X509SvidKey: keyDER, Bundle: ca.Cert.Raw,
	}}}, nil
}

// FetchX509SVID implements the Workload API streaming RPC.
func (w *WorkloadAPI) FetchX509SVID(_ *workload.X509SVIDRequest, stream grpc.ServerStreamingServer[workload.X509SVIDResponse]) error {
	md, _ := metadata.FromIncomingContext(stream.Context())
	if v := md.Get("workload.spiffe.io"); len(v) == 0 || v[0] != "true" {
		return errors.New("security header missing")
	}
	for {
		resp, err := w.issue()
		if err != nil {
			return err
		}
		if err := stream.Send(resp); err != nil {
			return err
		}
		w.mu.Lock()
		half := w.ttl / 2
		w.mu.Unlock()
		select {
		case <-stream.Context().Done():
			return nil
		case <-time.After(half):
		}
	}
}

// FetchX509Bundles implements the bundle stream (bundle only, re-sent on CA change).
func (w *WorkloadAPI) FetchX509Bundles(_ *workload.X509BundlesRequest, stream grpc.ServerStreamingServer[workload.X509BundlesResponse]) error {
	for {
		ca := w.CA()
		if err := stream.Send(&workload.X509BundlesResponse{Bundles: map[string][]byte{"spiffe://" + ca.TrustDomain: ca.Cert.Raw}}); err != nil {
			return err
		}
		select {
		case <-stream.Context().Done():
			return nil
		case <-time.After(time.Second):
		}
	}
}
