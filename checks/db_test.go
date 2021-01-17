package checks_test

import (
	"errors"
	"testing"
	"time"

	_ "github.com/lib/pq"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/GlobalWebIndex/healthcheck/checks"
)

func TestDatabasePingCheck(t *testing.T) {
	assert.Error(t, checks.DatabasePingCheck(nil, 1*time.Second)(), "nil DB should fail")

	db, _, err := sqlmock.New()
	assert.NoError(t, err)
	assert.NoError(t, checks.DatabasePingCheck(db, 1*time.Second)(), "ping should succeed")
}

func TestDatabaseSelectCheck(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	t.Run("Successful", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(err)
		defer db.Close()

		mock.ExpectExec("SELECT 1").WillReturnResult(sqlmock.NewResult(1, 1))

		err = checks.DatabaseSelectCheck(db, 1*time.Second)()
		assert.NoError(err)

		assert.NoError(mock.ExpectationsWereMet())
	})

	t.Run("Failure", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(err)
		defer db.Close()

		mock.ExpectExec("SELECT 1").WillReturnError(errors.New("driver: bad connection"))

		err = checks.DatabaseSelectCheck(db, 1*time.Second)()
		assert.Error(err)

		assert.NoError(mock.ExpectationsWereMet())
	})
}
