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

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type MetricSet struct {
	RequestCounter  *prometheus.CounterVec
	RequestDuration *prometheus.HistogramVec
	RequestSize     *prometheus.HistogramVec
	ResponseSize    *prometheus.HistogramVec
	InFlightGauge   prometheus.Gauge
}

func NewMetricSet(r *prometheus.Registry) *MetricSet {
	m := &MetricSet{
		RequestCounter: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "dioad_net_http_requests_total",
				Help: "Counter of HTTP requests.",
			},
			[]string{"handler", "code"},
		),
		RequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "dioad_net_http_request_duration_seconds",
				Help:    "Histogram of latencies for HTTP requests.",
				Buckets: []float64{.1, .2, .4, 1, 3, 8, 20, 60, 120},
			},
			[]string{"handler"},
		),
		RequestSize: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "dioad_net_http_request_size_bytes",
				Help:    "Histogram of request size for HTTP requests.",
				Buckets: prometheus.ExponentialBuckets(100, 10, 8),
			},
			[]string{"handler"},
		),
		ResponseSize: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "dioad_net_http_response_size_bytes",
				Help:    "Histogram of response size for HTTP requests.",
				Buckets: prometheus.ExponentialBuckets(100, 10, 8),
			},
			[]string{"handler"},
		),
		InFlightGauge: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "dioad_net_http_in_flight_requests",
				Help: "Gauge of requests currently being served by the wrapped handler.",
			},
		),
	}

	return m
}

func (m *MetricSet) Register(r prometheus.Registerer) {
	// if r != nil {
	r.MustRegister(
		m.RequestCounter,
		m.RequestDuration,
		m.ResponseSize,
		m.RequestSize,
		m.InFlightGauge)
	// }
}

// Middleware - to make it a middleware for mux probably a better way.
// TODO: need to extract this from this struct to remove the coupling with mux
func (m *MetricSet) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		route := mux.CurrentRoute(r)
		path, _ := route.GetPathTemplate()
		promhttp.InstrumentHandlerInFlight(
			m.InFlightGauge,
			promhttp.InstrumentHandlerCounter(
				m.RequestCounter.MustCurryWith(prometheus.Labels{"handler": path}),
				promhttp.InstrumentHandlerDuration(
					m.RequestDuration.MustCurryWith(prometheus.Labels{"handler": path}),
					promhttp.InstrumentHandlerResponseSize(
						m.ResponseSize.MustCurryWith(prometheus.Labels{"handler": path}),
						promhttp.InstrumentHandlerRequestSize(
							m.RequestSize.MustCurryWith(prometheus.Labels{"handler": path}),
							next),
					),
				),
			)).ServeHTTP(w, r)
	})
}
