package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"

	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"

	"github.com/GlobalWebIndex/healthcheck/checks"
)

type grpcHandler struct {
	http.ServeMux

	globalContext context.Context
	globalCancel  context.CancelFunc

	checksMutex     sync.RWMutex
	readinessChecks map[string]error
	livenessChecks  map[string]error

	grpcDepsMutex    sync.RWMutex
	grpcHealthServer *health.Server
	grpcDeps         map[string]bool

	log *zap.Logger
}

// GrpcOption represents option for NewGrpcHandler.
type GrpcHandlerOption func(*grpcHandler)

// WithZapLogger sets zap logger
func WithZapLogger(log *zap.Logger) GrpcHandlerOption {
	return func(g *grpcHandler) {
		g.log = log
	}
}

func NewGrpcHandler(hs *health.Server, opts ...GrpcHandlerOption) GrpcHandler {
	// set up global context with cancel
	gctx, cancel := context.WithCancel(context.Background())

	h := &grpcHandler{
		livenessChecks:   make(map[string]error),
		readinessChecks:  make(map[string]error),
		grpcHealthServer: hs,
		grpcDeps:         make(map[string]bool),
		globalContext:    gctx,
		globalCancel:     cancel,
	}

	for _, opt := range opts {
		opt(h)
	}

	h.grpcHealthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_NOT_SERVING)

	h.Handle("/live", http.HandlerFunc(h.LiveEndpoint))
	h.Handle("/ready", http.HandlerFunc(h.ReadyEndpoint))

	return h
}

func (s *grpcHandler) Close() {
	s.globalCancel()
}

func (s *grpcHandler) AddGrpcReadinessCheck(name string, grpcClient grpc_health_v1.HealthClient) error {

	s.grpcDepsMutex.Lock()
	defer s.grpcDepsMutex.Unlock()

	if _, ok := s.grpcDeps[name]; ok {
		return fmt.Errorf("grpc readiness check '%s' already exists", name)
	}

	// we start in failed state
	s.grpcDeps[name] = false

	stream, err := grpcClient.Watch(s.globalContext, &grpc_health_v1.HealthCheckRequest{})
	if err != nil {
		return err
	}

	go func() {
		for {
			resp, err := stream.Recv()
			switch {
			case err == io.EOF:
				break

			case err != nil:
				if s.log != nil {
					s.log.Warn("Readiness check for gRPC service failed on `stream.Recv()`",
						zap.String("name", name), zap.Error(err))
				}
				return
			}

			s.grpcDepsMutex.Lock()

			switch {
			// grpc dep was KO, now it is fine
			case resp.Status == grpc_health_v1.HealthCheckResponse_SERVING && !s.grpcDeps[name]:
				if s.log != nil {
					s.log.Debug("Grpc readiness check (watch): NOT_SERVING --> SERVING", zap.String("name", name))
				}
				s.grpcDeps[name] = true

				s.grpcDepsMutex.Unlock()

				// we must check a) other grpc deps b) readiness/liveness checks before setting
				// serving status back to normal

				ok := true
				ok = ok && s.areGrpcDepsOk()
				ok = ok && s.areChecksOk()

				if !ok {
					continue
				}

				s.grpcHealthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)

			// grpc dep was fine, now it is KO
			case resp.Status == grpc_health_v1.HealthCheckResponse_NOT_SERVING && s.grpcDeps[name]:
				if s.log != nil {
					s.log.Debug("Grpc readiness check (watch): SERVING --> NOT_SERVING", zap.String("name", name))
				}
				s.grpcDeps[name] = false

				s.grpcDepsMutex.Unlock()

				s.grpcHealthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_NOT_SERVING)

			default:
				if s.log != nil {
					s.log.Debug("Grpc readiness check (watch): unexpected status received", zap.String("name", name), zap.String("received_status", resp.Status.String()))
				}
				s.grpcDepsMutex.Unlock()
			}
		}
	}()

	return nil
}

// areChecksOk returns true only when all readiness and liveness check
// function's last executions had no errors. Otherwise, false is returned.
func (s *grpcHandler) areChecksOk() (ok bool) {
	ok = true
	s.checksMutex.RLock()
	defer s.checksMutex.RUnlock()

	for _, checkResult := range s.readinessChecks {
		if checkResult != nil {
			ok = false
			return
		}
	}
	for _, checkResult := range s.livenessChecks {
		if checkResult != nil {
			ok = false
			return
		}
	}
	return
}

func (s *grpcHandler) readinessOkWithResults(results map[string]string) (ok bool) {
	ok = true

	s.checksMutex.RLock()
	defer s.checksMutex.RUnlock()

	for name, checkResult := range s.readinessChecks {
		if checkResult != nil {
			ok = false
			results[name] = checkResult.Error()
		} else {
			results[name] = "OK"
		}
	}
	return
}

func (s *grpcHandler) livenessOkWithResults(results map[string]string) (ok bool) {
	ok = true

	s.checksMutex.RLock()
	defer s.checksMutex.RUnlock()

	for name, checkResult := range s.livenessChecks {
		if checkResult != nil {
			ok = false
			results[name] = checkResult.Error()
		} else {
			results[name] = "OK"
		}
	}
	return
}

func (s *grpcHandler) grpcDepsOkWithResults(results map[string]string) (ok bool) {
	ok = true

	s.grpcDepsMutex.RLock()
	defer s.grpcDepsMutex.RUnlock()

	for name, checkResult := range s.grpcDeps {
		if !checkResult {
			ok = false
			results[name] = "grpc service is down"
		} else {
			results[name] = "OK"
		}
	}
	return
}

// areGrpcDepsOk returns true only when last health checks of all grpc
// dependencies were succefull.
func (s *grpcHandler) areGrpcDepsOk() (ok bool) {
	ok = true

	s.grpcDepsMutex.RLock()
	defer s.grpcDepsMutex.RUnlock()

	for _, checkResult := range s.grpcDeps {
		if !checkResult {
			ok = false
			break
		}
	}
	return
}

func (s *grpcHandler) AddReadinessCheck(name string, check checks.Check, interval time.Duration) error {
	s.checksMutex.Lock()
	defer s.checksMutex.Unlock()

	if _, ok := s.readinessChecks[name]; ok {
		return fmt.Errorf("readiness check '%s' already exists", name)
	}

	// we start in failed state
	s.readinessChecks[name] = errors.New("placeholder")

	checks.AsyncWithContext(s.globalContext,
		func() error {
			err := check()

			s.checksMutex.Lock()

			switch {
			// check was fine, now it is KO
			case s.readinessChecks[name] == nil && err != nil:
				if s.log != nil {
					s.log.Debug("Readiness check: OK -> FAILED", zap.String("name", name))
				}

				s.readinessChecks[name] = err
				s.checksMutex.Unlock()

				// we can set serving status immediately
				s.grpcHealthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_NOT_SERVING)

			// check was KO, now it's fine
			case s.readinessChecks[name] != nil && err == nil:
				if s.log != nil {
					s.log.Debug("Readiness check: FAILED -> OK", zap.String("name", name))
				}

				s.readinessChecks[name] = err
				s.checksMutex.Unlock()

				// we must check readiness/liveness checks and grpc deps before setting serving status
				ok := true
				ok = ok && s.areChecksOk()
				ok = ok && s.areGrpcDepsOk()

				if ok {
					s.grpcHealthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
				}

			default:
				s.checksMutex.Unlock()
			}
			return nil
		}, interval)
	return nil
}

func (s *grpcHandler) AddLivenessCheck(name string, check checks.Check, interval time.Duration) error {
	s.checksMutex.Lock()
	defer s.checksMutex.Unlock()

	if _, ok := s.livenessChecks[name]; ok {
		return fmt.Errorf("liveness check '%s' already exists", name)
	}

	// we start in failed state
	s.livenessChecks[name] = errors.New("placeholder")

	checks.AsyncWithContext(s.globalContext,
		func() error {
			err := check()

			s.checksMutex.Lock()

			switch {
			// check was fine, now it is KO
			case s.livenessChecks[name] == nil && err != nil:
				if s.log != nil {
					s.log.Debug("Liveness check: OK -> FAILED", zap.String("name", name))
				}

				s.livenessChecks[name] = err
				s.checksMutex.Unlock()

				// we can set serving status immediately
				s.grpcHealthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_NOT_SERVING)

			// check was KO, now it's fine
			case s.livenessChecks[name] != nil && err == nil:
				if s.log != nil {
					s.log.Debug("Liveness check: FAILED -> OK", zap.String("name", name))
				}

				s.livenessChecks[name] = err
				s.checksMutex.Unlock()

				// we must check readiness/liveness checks and grpc deps before setting serving status
				ok := true
				ok = ok && s.areChecksOk()
				ok = ok && s.areGrpcDepsOk()

				if ok {
					s.grpcHealthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
				}

			default:
				s.checksMutex.Unlock()
			}
			return nil
		}, interval)
	return nil
}

func (s *grpcHandler) ReadyEndpoint(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	results := make(map[string]string, len(s.livenessChecks)+len(s.readinessChecks))
	status := http.StatusOK

	ok := true
	ok = ok && s.livenessOkWithResults(results)
	ok = ok && s.readinessOkWithResults(results)
	ok = ok && s.grpcDepsOkWithResults(results)

	if !ok {
		status = http.StatusServiceUnavailable
	}

	// write out the response code and content type header
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)

	// unless ?full=1, return an empty body. Kubernetes only cares about the
	// HTTP status code, so we won't waste bytes on the full body.
	if r.URL.Query().Get("full") != "1" {
		_, _ = w.Write([]byte("{}\n"))
		return
	}

	// otherwise, write the JSON body ignoring any encoding errors (which
	// shouldn't really be possible since we're encoding a map[string]string).
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "    ")
	_ = encoder.Encode(results)
}

func (s *grpcHandler) LiveEndpoint(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	results := make(map[string]string, len(s.livenessChecks)+len(s.readinessChecks))
	status := http.StatusOK

	ok := true
	ok = ok && s.livenessOkWithResults(results)

	if !ok {
		status = http.StatusServiceUnavailable
	}

	// write out the response code and content type header
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)

	// unless ?full=1, return an empty body. Kubernetes only cares about the
	// HTTP status code, so we won't waste bytes on the full body.
	if r.URL.Query().Get("full") != "1" {
		_, _ = w.Write([]byte("{}\n"))
		return
	}

	// otherwise, write the JSON body ignoring any encoding errors (which
	// shouldn't really be possible since we're encoding a map[string]string).
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "    ")
	_ = encoder.Encode(results)

}
