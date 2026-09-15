//go:build integration

package timescale

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func startTimescale(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "timescale/timescaledb:latest-pg16",
			ExposedPorts: []string{"5432/tcp"},
			Env:          map[string]string{"POSTGRES_PASSWORD": "test", "POSTGRES_DB": "audit"},
			WaitingFor:   wait.ForListeningPort("5432/tcp").WithStartupTimeout(2 * time.Minute),
		},
		Started: true,
	})
	if err != nil {
		t.Skipf("testcontainers unavailable: %v", err)
	}
	t.Cleanup(func() { _ = c.Terminate(ctx) })
	host, _ := c.Host(ctx)
	port, _ := c.MappedPort(ctx, "5432/tcp")
	return "postgres://postgres:test@" + host + ":" + port.Port() + "/audit?sslmode=disable"
}

func TestSinkAgainstTimescale(t *testing.T) {
	dsn := startTimescale(t)
	ctx := context.Background()
	if err := Migrate(ctx, dsn); err != nil {
		t.Fatal(err)
	}
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	var n int
	if err := conn.QueryRow(ctx, "SELECT count(*) FROM timescaledb_information.hypertables WHERE hypertable_name='audit_events'").Scan(&n); err != nil || n != 1 {
		t.Fatalf("hypertable: %d %v", n, err)
	}
	if err := conn.QueryRow(ctx, "SELECT count(*) FROM timescaledb_information.jobs WHERE hypertable_name='audit_events' AND proc_name IN ('policy_retention','policy_compression')").Scan(&n); err != nil || n != 2 {
		t.Fatalf("policies: %d %v", n, err)
	}
	// INSERT-only role.
	_, _ = conn.Exec(ctx, "CREATE ROLE freya_sink LOGIN PASSWORD 'sink'; GRANT INSERT ON audit_events TO freya_sink")
	sink, err := New(ctx, dsn, Options{BatchSize: 10, FlushInterval: 100 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 25; i++ {
		sink.Emit(ctx, ev(i))
	}
	sink.Close()
	if err := conn.QueryRow(ctx, "SELECT count(*) FROM audit_events").Scan(&n); err != nil || n != 25 {
		t.Fatalf("rows: %d %v", n, err)
	}
	if sink.Written() != 25 || sink.Dropped() != 0 {
		t.Fatalf("written=%d dropped=%d", sink.Written(), sink.Dropped())
	}
	// The sink role cannot delete or update.
	sinkConn, err := pgx.Connect(ctx, "postgres://freya_sink:sink@"+dsn[len("postgres://postgres:test@"):])
	if err != nil {
		t.Fatal(err)
	}
	defer sinkConn.Close(ctx)
	if _, err := sinkConn.Exec(ctx, "DELETE FROM audit_events"); err == nil {
		t.Fatal("sink role must not be able to delete")
	}
}
