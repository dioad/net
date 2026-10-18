package http

import (
	"crypto/tls"
	"sort"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

// OperationalConfig configures an operational server built by
// NewOperationalServer.
type OperationalConfig struct {
	// ListenAddress is the address to listen on, e.g. ":8125".
	ListenAddress string
	// EnablePrometheusMetrics serves /metrics.
	EnablePrometheusMetrics bool
	// EnableStatus serves /status.
	EnableStatus bool
	// EnableDebug serves /debug (pprof). It exposes memory contents, so keep
	// it off unless debugging.
	EnableDebug bool
	// TLSConfig, when set, serves the operational endpoints over TLS, for
	// example with client-certificate verification where the listener is
	// reachable beyond the platform.
	TLSConfig *tls.Config
	// EnableProxyProtocol accepts a PROXY protocol header, for a listener a
	// proxy forwards to. Leave it off for a platform check or scraper that
	// connects directly: with the default ProxyProtocolPolicy (require) a
	// connection without a header is refused.
	EnableProxyProtocol bool
	// ProxyProtocolPolicy applies when EnableProxyProtocol is set; see
	// Config.ProxyProtocolPolicy.
	ProxyProtocolPolicy ProxyProtocolPolicy
}

// NewOperationalServer returns a server that serves primary's operational
// endpoints on a listener of their own: /health/live and /health/ready
// always, and /metrics, /status and /debug as cfg says.
//
// It is for a platform health check and metrics scraper that cannot present
// credentials, while primary keeps authenticating its own routes. It shares
// primary's health registry, so a resource added to primary at any point is
// reported here, and its /metrics serves primary's request metrics with the
// process-wide default registry. It inherits none of primary's middlewares,
// authentication or resources, and none of its TLS or PROXY protocol
// settings: it serves plain HTTP with no PROXY protocol unless cfg says
// otherwise. Bind it where only the platform can reach it, or give it TLS
// with client verification.
//
// opts are applied last, so WithLogger or WithMetricsGatherers can override
// the defaults.
func NewOperationalServer(primary *Server, cfg OperationalConfig, opts ...ServerOption) *Server {
	s := NewServer(Config{
		ListenAddress:           cfg.ListenAddress,
		EnablePrometheusMetrics: cfg.EnablePrometheusMetrics,
		EnableStatus:            cfg.EnableStatus,
		EnableDebug:             cfg.EnableDebug,
		EnableHealth:            true,
		TLSConfig:               cfg.TLSConfig,
		EnableProxyProtocol:     cfg.EnableProxyProtocol,
		ProxyProtocolPolicy:     cfg.ProxyProtocolPolicy,
	})
	s.HealthRegistry = primary.HealthRegistry
	// The operational server's own request metrics are left out: they carry
	// the same series names as primary's, and a probe of a path primary also
	// serves would otherwise make the gather fail on a duplicate series.
	s.metricsGatherers = prometheus.Gatherers{dedupGatherer{primary.metricSet.registry, prometheus.DefaultGatherer}}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// dedupGatherer gathers from each gatherer in turn and keeps the first copy
// of any series that more than one returns, where prometheus.Gatherers would
// fail the whole gather. That happens when a caller has also registered a
// server's metric set in the default registry with WithPrometheusRegistry.
type dedupGatherer []prometheus.Gatherer

// Gather implements prometheus.Gatherer.
func (g dedupGatherer) Gather() ([]*dto.MetricFamily, error) {
	families := map[string]*dto.MetricFamily{}
	seen := map[string]bool{}
	var errs prometheus.MultiError
	for _, gatherer := range g {
		got, err := gatherer.Gather()
		if err != nil {
			errs.Append(err)
		}
		for _, family := range got {
			name := family.GetName()
			merged, ok := families[name]
			if !ok {
				merged = &dto.MetricFamily{Name: family.Name, Help: family.Help, Type: family.Type, Unit: family.Unit}
				families[name] = merged
			}
			for _, metric := range family.GetMetric() {
				key := name + "\x00" + labelSignature(metric.GetLabel())
				if seen[key] {
					continue
				}
				seen[key] = true
				merged.Metric = append(merged.Metric, metric)
			}
		}
	}

	out := make([]*dto.MetricFamily, 0, len(families))
	for _, family := range families {
		out = append(out, family)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].GetName() < out[j].GetName() })
	return out, errs.MaybeUnwrap()
}

// labelSignature identifies a series within a family by its label pairs.
func labelSignature(labels []*dto.LabelPair) string {
	pairs := make([]string, 0, len(labels))
	for _, l := range labels {
		pairs = append(pairs, l.GetName()+"="+l.GetValue())
	}
	sort.Strings(pairs)
	return strings.Join(pairs, ",")
}
