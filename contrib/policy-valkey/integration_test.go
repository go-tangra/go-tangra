//go:build integration

package valkey

import (
	"context"
	"testing"
	"time"

	"github.com/go-freya/freya/identity"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	valkeygo "github.com/valkey-io/valkey-go"
)

func startValkey(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image: "valkey/valkey:8", ExposedPorts: []string{"6379/tcp"},
			WaitingFor: wait.ForListeningPort("6379/tcp").WithStartupTimeout(time.Minute),
		},
		Started: true,
	})
	if err != nil {
		t.Skipf("testcontainers unavailable: %v", err)
	}
	t.Cleanup(func() { _ = c.Terminate(ctx) })
	host, _ := c.Host(ctx)
	port, _ := c.MappedPort(ctx, "6379/tcp")
	return host + ":" + port.Port()
}

func TestAgainstValkey(t *testing.T) {
	addr := startValkey(t)
	ctx := context.Background()
	kv, err := NewClient(Config{Addresses: []string{addr}, AllowPlaintext: true})
	if err != nil {
		t.Fatal(err)
	}
	defer kv.Close()
	admin, _ := valkeygo.NewClient(valkeygo.ClientOption{InitAddress: []string{addr}})
	defer admin.Close()
	set := func(k, v string) {
		if err := admin.Do(ctx, admin.B().Set().Key(k).Value(v).Build()).Error(); err != nil {
			t.Fatal(err)
		}
	}
	set(docKey("example.org"), doc1)
	set(versionKey("example.org"), "one")
	src, err := NewSource(ctx, kv, "example.org", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	wctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	ch, _ := src.Watch(wctx)
	time.Sleep(300 * time.Millisecond)
	set(docKey("example.org"), doc2)
	set(versionKey("example.org"), "two")
	start := time.Now()
	_ = admin.Do(ctx, admin.B().Publish().Channel(ChangedChannel).Message("two").Build()).Error()
	select {
	case p := <-ch:
		if p.Version != "two" || time.Since(start) > 2*time.Second {
			t.Fatalf("reload %+v after %s", p, time.Since(start))
		}
	case <-wctx.Done():
		t.Fatal("no reload")
	}
	id := identity.ForService("example.org", "orders")
	if err := admin.Do(ctx, admin.B().Set().Key(revokedKey(id.String(), "")).Value("1").Ex(time.Minute).Build()).Error(); err != nil {
		t.Fatal(err)
	}
	if ok, err := NewRevocationChecker(kv).IsRevoked(ctx, id, "9"); !ok || err != nil {
		t.Fatalf("revoked=%v err=%v", ok, err)
	}
}
