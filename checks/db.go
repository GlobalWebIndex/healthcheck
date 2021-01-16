package checks

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/pkg/errors"
)

// DatabasePingCheck returns a Check that validates connectivity to a
// database/sql.DB using Ping().
func DatabasePingCheck(database *sql.DB, timeout time.Duration) Check {
	return func() error {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		if database == nil {
			return fmt.Errorf("database is nil")
		}
		return database.PingContext(ctx)
	}
}

// DatabaseSelectCheck returns a Check that validates connectivity to a
// database using `SELECT 1` query execution.
func DatabaseSelectCheck(client *sql.DB, timeout time.Duration) Check {
	return func() error {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		res, err := client.ExecContext(ctx, "SELECT 1")
		if err != nil {
			return errors.Wrap(err, "SQL healthcheck failed for SELECT 1")
		}
		rows, err := res.RowsAffected()
		if err != nil {
			return errors.Wrap(err, "SQL healthcheck failed, rows affected error")
		}
		if rows != 1 {
			return errors.Errorf("SQL healthcheck failed, incorrect DB result (expected 1 row, got %d)", rows)
		}
		return nil
	}
}
