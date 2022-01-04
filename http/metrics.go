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
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type metrics struct {
	requestCounter  *prometheus.CounterVec
	requestDuration *prometheus.HistogramVec
	responseSize    *prometheus.HistogramVec
}

func newMetrics(r prometheus.Registerer) *metrics {
	m := &metrics{
		requestCounter: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "prometheus_http_requests_total",
				Help: "Counter of HTTP requests.",
			},
			[]string{"handler", "code"},
		),
		requestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "prometheus_http_request_duration_seconds",
				Help:    "Histogram of latencies for HTTP requests.",
				Buckets: []float64{.1, .2, .4, 1, 3, 8, 20, 60, 120},
			},
			[]string{"handler"},
		),
		responseSize: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "prometheus_http_response_size_bytes",
				Help:    "Histogram of response size for HTTP requests.",
				Buckets: prometheus.ExponentialBuckets(100, 10, 8),
			},
			[]string{"handler"},
		),
	}

	if r != nil {
		r.MustRegister(m.requestCounter, m.requestDuration, m.responseSize)
	}
	return m
}

func (m *metrics) instrumentHandlerWithPrefix(prefix string) func(handlerName string, handler http.HandlerFunc) http.HandlerFunc {
	return func(handlerName string, handler http.HandlerFunc) http.HandlerFunc {
		return m.instrumentHandler(prefix+handlerName, handler)
	}
}

func (m *metrics) instrumentHandler(handlerName string, handler http.HandlerFunc) http.HandlerFunc {
	return promhttp.InstrumentHandlerCounter(
		m.requestCounter.MustCurryWith(prometheus.Labels{"handler": handlerName}),
		promhttp.InstrumentHandlerDuration(
			m.requestDuration.MustCurryWith(prometheus.Labels{"handler": handlerName}),
			promhttp.InstrumentHandlerResponseSize(
				m.responseSize.MustCurryWith(prometheus.Labels{"handler": handlerName}),
				handler,
			),
		),
	)
}
