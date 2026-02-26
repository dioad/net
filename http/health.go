package http

import (
	"net/http"

	"github.com/rs/zerolog"

	diojson "github.com/dioad/net/http/json"
)

// HealthRegistry manages the collection and aggregation of resource health and status.
type HealthRegistry struct {
	logger      zerolog.Logger
	resources   map[string]Resource
	metadataMap map[string]any
}

// NewHealthRegistry creates a new HealthRegistry.
func NewHealthRegistry(logger zerolog.Logger) *HealthRegistry {
	return &HealthRegistry{
		logger:      logger,
		resources:   make(map[string]Resource),
		metadataMap: make(map[string]any),
	}
}

// Register adds a resource to the health registry at the given path.
func (h *HealthRegistry) Register(path string, r Resource) {
	h.resources[path] = r
}

// AddStaticMetadata adds static metadata to be included in status responses.
func (h *HealthRegistry) AddStaticMetadata(key string, value any) {
	h.metadataMap[key] = value
}

// aggregateLivenessHandler checks all LivenessResource implementations
func (h *HealthRegistry) aggregateLivenessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		httpStatus := http.StatusOK

		for path, resource := range h.resources {
			if sr, ok := resource.(LivenessResource); ok {
				err := sr.Live()
				if err != nil {
					httpStatus = http.StatusInternalServerError
					h.logger.Error().Err(err).Str("path", path).Msg("resource not alive")
					break
				}
			}
		}

		res := diojson.NewResponseWithLogger(w, r, h.logger)
		res.Data(httpStatus, map[string]any{
			"live": httpStatus == http.StatusOK,
		})

		logEvent := h.logger.Debug()
		logEvent.Int("status_code", httpStatus).
			Bool("live", httpStatus == http.StatusOK).
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Msg("liveness request processed")
	}
}

// aggregateReadinessHandler checks all ReadinessResource implementations
func (h *HealthRegistry) aggregateReadinessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		httpStatus := http.StatusOK
		resourceReadiness := make(map[string]any)
		resourceErrors := make(map[string]string)

		for path, resource := range h.resources {
			if rr, ok := resource.(ReadinessResource); ok {
				ready, err := rr.Ready()
				if err != nil {
					httpStatus = http.StatusServiceUnavailable
					h.logger.Error().Err(err).Str("path", path).Msg("error checking resource readiness")
					resourceErrors[path] = err.Error()
					continue
				}
				resourceReadiness[path] = ready
			}
		}

		res := diojson.NewResponseWithLogger(w, r, h.logger)
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

// aggregateStatusHandler checks all StatusResource implementations
func (h *HealthRegistry) aggregateStatusHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		statusMap := make(map[string]any)
		httpStatus := http.StatusOK

		resourceStatus := make(map[string]any)
		resourceErrors := make(map[string]string)
		for path, resource := range h.resources {
			if sr, ok := resource.(StatusResource); ok {
				status, err := sr.Status()
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
		statusMap["Metadata"] = h.metadataMap
		statusMap["Errors"] = resourceErrors

		res := diojson.NewResponseWithLogger(w, r, h.logger)
		res.Data(httpStatus, statusMap)

		logEvent := h.logger.Debug()
		logEvent.Int("status_code", httpStatus).
			Int("resource_count", len(resourceStatus)).
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Msg("status request processed")
	}
}
