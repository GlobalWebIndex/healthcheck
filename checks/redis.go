package checks

import (
	"github.com/pkg/errors"

	"github.com/go-redis/redis"
)

// CheckRedis returns a Check function that validates Redis connection.
func CheckRedis(client redis.Cmdable) Check {
	return func() error {
		err := client.Ping().Err()
		if err != nil {
			err = errors.Wrap(err, "Redis healthcheck failed")
		}
		return err
	}
}
