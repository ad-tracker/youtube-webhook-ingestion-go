package testutil

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	testDatabase = "youtube_webhooks_test"
	testUser     = "test"
	testPassword = "test"
)

// TestDatabase represents a test database instance.
type TestDatabase struct {
	Pool      *pgxpool.Pool
	Container *postgres.PostgresContainer
	ConnStr   string
}

// SetupTestDatabase creates a PostgreSQL container, runs migrations, and returns a connection pool.
func SetupTestDatabase(t *testing.T) *TestDatabase {
	ctx := context.Background()

	// Create PostgreSQL container
	pgContainer, err := postgres.Run(ctx,
		"postgres:17-alpine",
		postgres.WithDatabase(testDatabase),
		postgres.WithUsername(testUser),
		postgres.WithPassword(testPassword),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second)),
	)
	require.NoError(t, err)

	// Get connection string
	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	// Run migrations
	migrationsPath, err := filepath.Abs("../../../migrations")
	require.NoError(t, err)

	m, err := migrate.New(
		fmt.Sprintf("file://%s", migrationsPath),
		connStr,
	)
	require.NoError(t, err)

	err = m.Up()
	require.NoError(t, err)
	m.Close()

	// Create connection pool
	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err)

	// Verify connection
	err = pool.Ping(ctx)
	require.NoError(t, err)

	return &TestDatabase{
		Pool:      pool,
		Container: pgContainer,
		ConnStr:   connStr,
	}
}

// Cleanup closes the pool and terminates the container.
func (td *TestDatabase) Cleanup(t *testing.T) {
	ctx := context.Background()

	if td.Pool != nil {
		td.Pool.Close()
	}

	if td.Container != nil {
		err := td.Container.Terminate(ctx)
		require.NoError(t, err)
	}
}

// TruncateTables truncates all tables in the database for test isolation.
func (td *TestDatabase) TruncateTables(t *testing.T) {
	ctx := context.Background()

	// Disable triggers temporarily to avoid constraint issues
	_, err := td.Pool.Exec(ctx, "SET session_replication_role = replica;")
	require.NoError(t, err)

	// Truncate all tables
	_, err = td.Pool.Exec(ctx, `
		TRUNCATE TABLE video_updates, videos, channels, webhook_events, pubsub_subscriptions RESTART IDENTITY CASCADE;
	`)
	require.NoError(t, err)

	// Re-enable triggers
	_, err = td.Pool.Exec(ctx, "SET session_replication_role = DEFAULT;")
	require.NoError(t, err)
}
