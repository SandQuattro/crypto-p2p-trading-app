package database

import (
	"clean-arch-template/config"
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/multitracer"
	"github.com/jackc/pgx/v5/pgxpool"
)

func setupPoolConfig(cfg *config.Config, pg *Postgres, poolConfig *pgxpool.Config) {
	poolConfig.MinConns = 1 // Ensure we keep at least one connection for health checks
	poolConfig.MaxConns = pg.maxPoolSize
	poolConfig.ConnConfig.ConnectTimeout = time.Duration(pg.connTimeout) * time.Second
	poolConfig.HealthCheckPeriod = time.Duration(pg.healthCheckPeriod) * time.Minute

	if cfg.Debug {
		poolConfig.ConnConfig.Tracer = &multitracer.Tracer{
			QueryTracers: []pgx.QueryTracer{
				&SQLQueryTracer{},
			},
			ConnectTracers: []pgx.ConnectTracer{
				&ConnectTracer{},
			},
		}

		poolConfig.BeforeAcquire = func(ctx context.Context, conn *pgx.Conn) bool {
			slog.Debug("[PGPOOL] attempting to acquire connection")
			return true
		}

		poolConfig.AfterRelease = func(conn *pgx.Conn) bool {
			slog.Debug("[PGPOOL] connection released to pool")
			return true
		}

		// Add health check logging
		poolConfig.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
			slog.Debug("[PGPOOL] health check: new connection established")
			return nil
		}

		// Log when connection is being checked
		poolConfig.BeforeClose = func(conn *pgx.Conn) {
			slog.Debug("[PGPOOL] health check: connection being closed")
		}
	}

}

type SQLQueryTracer struct {
}

func (t *SQLQueryTracer) TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	// pgx uses SELECT 1 for health checks
	if data.SQL == "SELECT 1" {
		slog.Debug("[PGPOOL] Health check ping started",
			"conn_id", fmt.Sprintf("%p", conn),
			"pid", conn.PgConn().PID(),
		)
	} else {
		slog.Debug("[PGPOOL] SQL query started", "sql", data.SQL, "args", fmt.Sprintf("%v", data.Args))
	}
	return ctx
}

func (t *SQLQueryTracer) TraceQueryEnd(_ context.Context, conn *pgx.Conn, data pgx.TraceQueryEndData) {
	if data.Err != nil {
		slog.Error("[PGPOOL] SQL query failed", "error", data.Err)
		return
	}
	// Check if this was a health check ping (CommandTag will be "SELECT 1")
	if data.CommandTag.String() == "SELECT 1" {
		slog.Debug("[PGPOOL] Health check ping completed",
			"conn_id", fmt.Sprintf("%p", conn),
			"pid", conn.PgConn().PID(),
			"command", data.CommandTag,
		)
	} else {
		slog.Debug("[PGPOOL] SQL query completed", "rows_affected", data.CommandTag.RowsAffected())
	}

}

type ConnectTracer struct {
}

func (t *ConnectTracer) TraceConnectStart(ctx context.Context, data pgx.TraceConnectStartData) context.Context {
	slog.Debug("[PGPOOL] Connecting to database")
	return ctx
}

func (t *ConnectTracer) TraceConnectEnd(_ context.Context, data pgx.TraceConnectEndData) {
	if data.Err != nil {
		slog.Error("[PGPOOL] Connecting to database failed", "error", data.Err)
		return
	}
	slog.Debug("[PGPOOL] Connected to database")
}
