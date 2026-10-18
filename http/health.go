package http

import (
	"errors"
	"maps"
	"net/http"
	"sync"

	"github.com/rs/zerolog"

	diojson "github.com/dioad/net/http/json"
)

// ErrNotAHealthSource is returned by RegisterHealthSource for a value that
// implements none of LivenessResource, ReadinessResource and StatusResource.
var ErrNotAHealthSource = errors.New("health source implements none of Live, Ready and Status")

// HealthRegistry manages the collection and aggregation of resource health and status.
// It is safe for concurrent use, so servers sharing one (see NewOperationalServer)
// can register sources while either is serving.
type HealthRegistry struct {
	logger zerolog.Logger

	mu          sync.RWMutex
	sources     map[string]any
	metadataMap map[string]any
}

// NewHealthRegistry creates a new HealthRegistry.
func NewHealthRegistry(logger zerolog.Logger) *HealthRegistry {
	return &HealthRegistry{
		logger:      logger,
		sources:     make(map[string]any),
		metadataMap: make(map[string]any),
	}
}

// Register adds a resource to the health registry at the given path. Server's
// AddResource calls it for every resource it mounts; the resource's Live, Ready
// and Status, where it implements them, are aggregated by /health/live,
// /health/ready and /status.
func (h *HealthRegistry) Register(path string, r Resource) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.sources[path] = r
}

// RegisterHealthSource adds source to the /health/live, /health/ready and
// /status aggregates under name, without mounting any route. source must
// implement at least one of LivenessResource, ReadinessResource and
// StatusResource; unlike Register it needs no Handler. Use it for a health
// check that has nothing to serve itself, such as a service's readiness
// gate. name shares a namespace with the paths of registered resources, and
// registering the same name again replaces the earlier source.
func (h *HealthRegistry) RegisterHealthSource(name string, source any) error {
	switch source.(type) {
	case LivenessResource, ReadinessResource, StatusResource:
	default:
		return ErrNotAHealthSource
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.sources[name] = source
	return nil
}

// AddStaticMetadata adds static metadata to be included in status responses.
func (h *HealthRegistry) AddStaticMetadata(key string, value any) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.metadataMap[key] = value
}

// snapshot returns copies of the registered sources and metadata, so the
// aggregate handlers run the checks without holding the lock.
func (h *HealthRegistry) snapshot() (map[string]any, map[string]any) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return maps.Clone(h.sources), maps.Clone(h.metadataMap)
}

// aggregateLivenessHandler checks all LivenessResource implementations.
func (h *HealthRegistry) aggregateLivenessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		httpStatus := http.StatusOK
		resourceErrors := make(map[string]string)

		sources, _ := h.snapshot()
		for path, source := range sources {
			if sr, ok := source.(LivenessResource); ok {
				err := sr.Live(r.Context())
				if err != nil {
					httpStatus = http.StatusInternalServerError
					h.logger.Error().Err(err).Str("path", path).Msg("resource not alive")
					resourceErrors[path] = err.Error()
				}
			}
		}

		res := diojson.NewResponseFromRequest(w, r)
		res.Data(httpStatus, map[string]any{
			"live":   httpStatus == http.StatusOK,
			"errors": resourceErrors,
		})

		logEvent := h.logger.Debug()
		logEvent.Int("status_code", httpStatus).
			Bool("live", httpStatus == http.StatusOK).
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Msg("liveness request processed")
	}
}

// aggregateReadinessHandler checks all ReadinessResource implementations.
func (h *HealthRegistry) aggregateReadinessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		httpStatus := http.StatusOK
		resourceReadiness := make(map[string]any)
		resourceErrors := make(map[string]string)

		sources, _ := h.snapshot()
		for path, source := range sources {
			if rr, ok := source.(ReadinessResource); ok {
				ready, err := rr.Ready(r.Context())
				if err != nil {
					httpStatus = http.StatusServiceUnavailable
					h.logger.Error().Err(err).Str("path", path).Msg("error checking resource readiness")
					resourceErrors[path] = err.Error()

					continue
				}
				resourceReadiness[path] = ready
			}
		}

		res := diojson.NewResponseFromRequest(w, r)
		res.Data(httpStatus, map[string]any{
			"ready":   httpStatus == http.StatusOK,
			"details": resourceReadiness,
			"errors":  resourceErrors,
		})

		logEvent := h.logger.Debug()
		logEvent.Int("status_code", httpStatus).
			Int("resource_count", len(resourceReadiness)).
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Msg("readiness request processed")
	}
}

// aggregateStatusHandler checks all StatusResource implementations.
func (h *HealthRegistry) aggregateStatusHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		statusMap := make(map[string]any)
		httpStatus := http.StatusOK

		resourceStatus := make(map[string]any)
		resourceErrors := make(map[string]string)
		sources, metadata := h.snapshot()
		for path, source := range sources {
			if sr, ok := source.(StatusResource); ok {
				status, err := sr.Status(r.Context())
				if err != nil {
					httpStatus = http.StatusInternalServerError
					h.logger.Error().Err(err).Str("path", path).Msg("error getting resource status")
					resourceErrors[path] = err.Error()

					continue
				}
				resourceStatus[path] = status
			}
		}
		statusMap["Routes"] = resourceStatus
		statusMap["Metadata"] = metadata
		statusMap["Errors"] = resourceErrors

		res := diojson.NewResponseFromRequest(w, r)
		res.Data(httpStatus, statusMap)

		logEvent := h.logger.Debug()
		logEvent.Int("status_code", httpStatus).
			Int("resource_count", len(resourceStatus)).
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Msg("status request processed")
	}
}
