package http

// Copyright 2015 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var rateLimitRequests = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Subsystem: "http",
		Name:      "dioad_net_http_rate_limit_requests_total",
		Help:      "Count of requests evaluated by rate limiter",
	},
	[]string{"result"},
)

type MetricSet struct {
	RequestCounter    *prometheus.CounterVec
	RequestDuration   *prometheus.HistogramVec
	RequestSize       *prometheus.HistogramVec
	ResponseSize      *prometheus.HistogramVec
	InFlightGauge     prometheus.Gauge
	RateLimitRequests *prometheus.CounterVec
	registry          *prometheus.Registry
}

func NewMetricSet(r *prometheus.Registry) *MetricSet {
	m := &MetricSet{
		registry: r,
		RequestCounter: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "dioad_net_http_requests_total",
				Help: "Counter of HTTP requests.",
			},
			[]string{"route", "code", "method"},
		),
		RequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "dioad_net_http_request_duration_seconds",
				Help:    "Histogram of latencies for HTTP requests.",
				Buckets: []float64{.1, .2, .4, 1, 3, 8, 20, 60, 120},
			},
			[]string{"route", "method"},
		),
		RequestSize: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "dioad_net_http_request_size_bytes",
				Help:    "Histogram of request size for HTTP requests.",
				Buckets: prometheus.ExponentialBuckets(100, 10, 8),
			},
			[]string{"route", "method"},
		),
		ResponseSize: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "dioad_net_http_response_size_bytes",
				Help:    "Histogram of response size for HTTP requests.",
				Buckets: prometheus.ExponentialBuckets(100, 10, 8),
			},
			[]string{"route", "method"},
		),
		InFlightGauge: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "dioad_net_http_in_flight_requests",
				Help: "Gauge of requests currently being served by the wrapped handler.",
			},
		),
		RateLimitRequests: rateLimitRequests,
	}

	return m
}

func (m *MetricSet) Register(r prometheus.Registerer) {
	r.MustRegister(
		m.RequestCounter,
		m.RequestDuration,
		m.ResponseSize,
		m.RequestSize,
		m.InFlightGauge,
	)
	if err := r.Register(m.RateLimitRequests); err != nil {
		if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
			panic(err)
		}
	}
}

// Middleware instruments the handler with prometheus metrics.
// It uses the provided ServeMux to derive the matched route pattern for the
// "route" label, preventing high-cardinality Prometheus series that would
// result from using raw URL paths.
func (m *MetricSet) Middleware(mux *http.ServeMux, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Use mux.Handler to derive the matched pattern before the mux routes
		// the request. This avoids high-cardinality label values that would occur
		// if we fell back to r.URL.Path (r.Pattern is empty outside the mux).
		route := r.URL.Path
		if mux != nil {
			if _, pattern := mux.Handler(r); pattern != "" {
				route = pattern
			}
		}

		labels := prometheus.Labels{
			"route": route,
		}
		promhttp.InstrumentHandlerInFlight(
			m.InFlightGauge,
			promhttp.InstrumentHandlerCounter(
				m.RequestCounter.MustCurryWith(labels),
				promhttp.InstrumentHandlerDuration(
					m.RequestDuration.MustCurryWith(labels),
					promhttp.InstrumentHandlerResponseSize(
						m.ResponseSize.MustCurryWith(labels),
						promhttp.InstrumentHandlerRequestSize(
							m.RequestSize.MustCurryWith(labels),
							next),
					),
				),
			)).ServeHTTP(w, r)
	})
}
