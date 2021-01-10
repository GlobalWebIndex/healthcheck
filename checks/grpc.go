package checks

import (
	"context"
	"time"

	"github.com/pkg/errors"

	"google.golang.org/grpc/health/grpc_health_v1"
)

// CheckRedis returns a Check function to check for readiness of given grpc
// service.
func CheckGRPC(grpcClient grpc_health_v1.HealthClient) Check {
	return func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		resp, err := grpcClient.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
		if err != nil {
			return errors.Wrap(err, "gRPC healthcheck failed")
		}
		if resp.Status != grpc_health_v1.HealthCheckResponse_SERVING {
			return errors.Errorf(
				"gRPC healthcheck failed, bad status: %s",
				grpc_health_v1.HealthCheckResponse_ServingStatus_name[int32(resp.Status)],
			)
		}
		return nil
	}
}
