// Copyright 2017 by the contributors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package handlers

import (
	"net/http"
	"time"

	"github.com/GlobalWebIndex/healthcheck/checks"
	"google.golang.org/grpc/health/grpc_health_v1"
)

// Handler is an http.Handler with additional methods that register health and
// readiness checks. It handles handle "/live" and "/ready" HTTP
// endpoints.
type Handler interface {
	// The Handler is an http.Handler, so it can be exposed directly and handle
	// /live and /ready endpoints.
	http.Handler

	// AddLivenessCheck adds a check that indicates that this instance of the
	// application should be destroyed or restarted. A failed liveness check
	// indicates that this instance is unhealthy, not some upstream dependency.
	// Every liveness check is also included as a readiness check.
	// Error is returned in case check `name` has already been added.
	AddLivenessCheck(name string, check checks.Check) error

	// AddReadinessCheck adds a check that indicates that this instance of the
	// application is currently unable to serve requests because of an upstream
	// or some transient failure. If a readiness check fails, this instance
	// should no longer receiver requests, but should not be restarted or
	// destroyed.
	// Error is returned in case check `name` has already been added.
	AddReadinessCheck(name string, check checks.Check) error

	// LiveEndpoint is the HTTP handler for just the /live endpoint, which is
	// useful if you need to attach it into your own HTTP handler tree.
	LiveEndpoint(http.ResponseWriter, *http.Request)

	// ReadyEndpoint is the HTTP handler for just the /ready endpoint, which is
	// useful if you need to attach it into your own HTTP handler tree.
	ReadyEndpoint(http.ResponseWriter, *http.Request)
}

// GrpcHandler is similar to Handler, but has support for gRPC:
//  * `AddGrpcReadinessCheck` opens a stream (Watch rpc of Health service)
//    and listens for health status changes of a gRPC dependency
//  * sets correct serving status of gRPC Health server automatically.
// Another difference is, that readiness and liveness checks are executed
// periodically and not only on `/live` or `/ready` HTTP GET requests.
type GrpcHandler interface {
	// following items are copied from previous `Handler` interface with
	// the same meaning, except that an additional parameter for specifying
	// check execution interval was added
	http.Handler
	AddLivenessCheck(name string, check checks.Check, interval time.Duration) error
	AddReadinessCheck(name string, check checks.Check, interval time.Duration) error
	LiveEndpoint(http.ResponseWriter, *http.Request)
	ReadyEndpoint(http.ResponseWriter, *http.Request)

	// AddGrpcReadinessCheck opens a stream (Watch rpc of Health service) and
	// listens for health status changes of a gRPC dependency.
	AddGrpcReadinessCheck(name string, grpcClient grpc_health_v1.HealthClient) error

	// Close performs cleanup on all background checks and resources
	Close()
}
