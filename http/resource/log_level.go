package resource

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog"

	dnh "github.com/dioad/net/http"
)

// type Duration time.Duration
//
// func (d Duration) MarshalJSON() ([]byte, error) {
// 	return json.Marshal(time.Duration(d).String())
// }
//
// // UnmarshalJSON sets the Duration from JSON
// func (d *Duration) UnmarshalJSON(data []byte) error {
// 	if string(data) == "null" {
// 		return nil
// 	}
//
// 	var dstr string
// 	if err := json.Unmarshal(data, &dstr); err != nil {
// 		return err
// 	}
// 	tt, err := time.ParseDuration(dstr)
// 	if err != nil {
// 		return err
// 	}
// 	*d = Duration(tt)
// 	return nil
// }

// LogLevelResource is an HTTP resource that allows getting and setting the global log level.
type LogLevelResource struct {
	LogSetter dnh.LogLevelSetter
	Logger    zerolog.Logger
}

// LogLevelPost represents the request body for setting a new log level.
type LogLevelPost struct {
	Level    string `json:"level"`
	Duration string `json:"duration"`
}

// LogLevelGet represents the response body for getting the current log level.
type LogLevelGet struct {
	DefaultLevel string `json:"default_level,omitempty"`
	Level        string `json:"level"`
	ExpiresAt    string `json:"expires_at,omitempty"` // Optional field, can be omitted
}

// LogLevelResourceStatus represents the status of the log level resource.
type LogLevelResourceStatus struct {
	Status string
}

// PostIndex returns an HTTP handler for setting the log level.
func (dr *LogLevelResource) PostIndex() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req LogLevelPost
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		duration, err := time.ParseDuration(req.Duration)
		if err != nil {
			http.Error(w, "Invalid duration format", http.StatusBadRequest)
			return
		}

		expiry, err := dr.LogSetter.SetLogLevelWithDuration(req.Level, duration)
		if err != nil {
			dr.Logger.Error().Err(err).Msg("Failed to set log level")
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		resp := LogLevelGet{
			DefaultLevel: dr.LogSetter.OriginalLogLevel(),
			Level:        dr.LogSetter.CurrentLogLevel(),
			ExpiresAt:    expiry.Format(time.RFC3339),
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			dr.Logger.Error().Err(err).Msg("Failed to encode response")
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	}
}

// GetIndex returns an HTTP handler for getting the current log level.
func (dr *LogLevelResource) GetIndex() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp := LogLevelGet{
			Level: dr.LogSetter.CurrentLogLevel(),
		}

		if !dr.LogSetter.ExpiresAt().IsZero() {
			resp.ExpiresAt = dr.LogSetter.ExpiresAt().Format(time.RFC3339)
			resp.DefaultLevel = dr.LogSetter.OriginalLogLevel()
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			dr.Logger.Error().Err(err).Msg("Failed to encode response")
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	}
}

// func (dr *LogLevelResource) Mux() *http.ServeMux {
// 	m := http.NewServeMux()
// 	m.HandleFunc("GET /{$}", dr.GetIndex())
// 	m.HandleFunc("POST /{$}", dr.PostIndex())
// 	return m
// }

// RegisterRoutes registers the log level resource routes on the provided router.
func (dr *LogLevelResource) RegisterRoutes(parentRouter *mux.Router) {
	parentRouter.HandleFunc("", dr.GetIndex()).Methods("GET")
	parentRouter.HandleFunc("/", dr.GetIndex()).Methods("GET")
	parentRouter.HandleFunc("", dr.PostIndex()).Methods("POST")
	parentRouter.HandleFunc("/", dr.PostIndex()).Methods("POST")
}

// Status returns the status of the log level resource.
func (dr *LogLevelResource) Status() (interface{}, error) {
	return LogLevelResourceStatus{
		Status: "OK",
	}, nil
}

// NewLogLevelResource creates a new log level resource.
func NewLogLevelResource(logger zerolog.Logger) *LogLevelResource {
	return &LogLevelResource{
		LogSetter: dnh.NewZeroLogLevelSetter(),
		Logger:    logger,
	}
}
