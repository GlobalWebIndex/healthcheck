package checks

import (
	"context"
	"database/sql"
	"time"

	"github.com/pkg/errors"
)

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
