package http

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/dioad/net/http/auth"
	"github.com/dioad/net/http/pprof"
	"github.com/gorilla/mux"
	"github.com/pires/go-proxyproto"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/weaveworks/common/middleware"
)

// Config ...
type Config struct {
	ListenAddress           string
	EnablePrometheusMetrics bool
	EnableDebug             bool
	EnableStatus            bool
	EnableProxyProtocol     bool
	TLSConfig               *tls.Config
	AuthConfig              auth.AuthenticationServerConfig
}

// Server ...
type Server struct {
	Config          Config
	ListenAddress   string
	Router          *mux.Router
	accessLogWriter io.Writer
	accessLogger    zerolog.Logger
	ResourceMap     map[string]Resource
	server          *http.Server
	serverInitOnce  sync.Once
	metricSet       *MetricSet
	instrument      *middleware.Instrument
	rootResource    RootResource
}

func newDefaultServer(config Config) *Server {
	r := prometheus.NewRegistry()
	m := NewMetricSet(r)
	rtr := mux.NewRouter()

	return &Server{
		Config:        config,
		ListenAddress: config.ListenAddress,
		Router:        rtr,
		ResourceMap:   make(map[string]Resource, 0),
		metricSet:     m}
}

// NewServer ...
func NewServer(config Config, accessLogWriter io.Writer) *Server {
	server := newDefaultServer(config)
	server.accessLogWriter = accessLogWriter

	return server
}

func NewServerWithLogger(config Config, accessLogger zerolog.Logger) *Server {
	server := newDefaultServer(config)
	server.accessLogger = accessLogger

	return server
}

func (s *Server) ConfigureTelemetryInstrument(i middleware.Instrument) {
	s.instrument = &i
	s.Router.Use(s.instrument.Wrap)
}

func (s *Server) ConfigurePrometheusRegistry(r prometheus.Registerer) {
	s.metricSet.Register(r)
}

// AddResource ...
func (s *Server) AddResource(pathPrefix string, r Resource) {
	subrouter := s.Router.PathPrefix(pathPrefix).Subrouter()
	if s.instrument != nil {
		subrouter.Use(s.instrument.Wrap)
	}
	if rp, ok := r.(PathResource); ok {
		rp.RegisterRoutesWithPrefix(subrouter, pathPrefix)
	} else if rp, ok := r.(DefaultResource); ok {
		rp.RegisterRoutes(subrouter)
	}
	s.ResourceMap[pathPrefix] = r
}

func (s *Server) AddRootResource(r RootResource) {
	s.rootResource = r
	// s.AddResource("/", r)
}

func (s *Server) handler() http.Handler {
	s.addDefaultHandlers()

	logHandler := ZerologStructuredLogHandler(s.accessLogger)
	if s.accessLogWriter != nil {
		logHandler = DefaultCombinedLogHandler(s.accessLogWriter)
	}

	// s.addDefaultHandlers()
	if s.rootResource != nil {
		//s.rootResource.RegisterRoutes(s.Router.Path("/").Subrouter())
		//s.AddResource("/", s.rootResource)

		s.AddHandler("/", s.rootResource.Index())
	}

	// uncomment if ensure that 404's get picked up by metrics
	// s.Router.NotFoundHandler = s.Router.NewRoute().HandlerFunc(http.NotFound).GetHandler()
	return logHandler(s.Router)
}

func (s *Server) AddHandler(path string, handler http.Handler) {
	s.AddHandlerFunc(path, handler.ServeHTTP)
}

func (s *Server) AddHandlerFunc(path string, handler http.HandlerFunc) {
	h := handler
	//if s.Config.EnablePrometheusMetrics {
	if s.instrument != nil {
		h = s.instrument.Wrap(h).ServeHTTP
	}
	//}
	s.Router.HandleFunc(path, h)
}

func (s *Server) addDefaultHandlers() {
	if s.Config.EnablePrometheusMetrics {
		s.Router.Handle("/metrics", promhttp.Handler())
	}

	if s.Config.EnableDebug {
		s.AddResource("/debug", pprof.NewResource(log.Logger))
	}

	if s.Config.EnableStatus {
		s.AddHandler("/status", s.aggregateStatusHandler())
	}
}

func (s *Server) aggregateStatusHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		statusMap := make(map[string]interface{}, 0)
		httpStatus := http.StatusOK

		for path, resource := range s.ResourceMap {
			if sr, ok := resource.(StatusResource); ok {

				status, err := sr.Status()
				if err != nil {
					httpStatus = http.StatusInternalServerError
				}
				statusMap[path] = status
			}
		}

		w.Header().Set("Content-Type", "text/json; charset=utf-8") // normal header

		w.WriteHeader(httpStatus)

		encoder := json.NewEncoder(w)
		encoder.Encode(statusMap)
	}
}

func (s *Server) ListenAndServe() error {
	return s.ListenAndServeTLS(s.Config.TLSConfig)
}

func (s *Server) initialiseServer() {
	s.serverInitOnce.Do(func() {
		s.server = &http.Server{
			ReadTimeout:  time.Minute,
			WriteTimeout: time.Minute,
			Handler:      s.handler(),
			Addr:         s.ListenAddress,
		}
	})
}

// ListenAndServe ...
func (s *Server) ListenAndServeTLS(tlsConfig *tls.Config) error {
	s.initialiseServer()
	s.server.TLSConfig = tlsConfig

	if tlsConfig != nil {
		return s.server.ListenAndServeTLS("", "")
	} else {
		return s.server.ListenAndServe()
	}
}

func (s *Server) Serve(ln net.Listener) error {
	s.initialiseServer()
	s.server.TLSConfig = s.Config.TLSConfig

	if s.Config.EnableProxyProtocol {
		ln = &proxyproto.Listener{
			Listener:          ln,
			ReadHeaderTimeout: 10 * time.Second,
		}
	}

	if s.Config.TLSConfig != nil {
		return s.server.ServeTLS(ln, "", "")
	}

	return s.server.Serve(ln)
}

func (s *Server) ServeTLS(ln net.Listener) error {
	return s.Serve(ln)
}

func (s *Server) RegisterOnShutdown(f func()) {
	s.initialiseServer()
	s.server.RegisterOnShutdown(f)
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.initialiseServer()
	return s.server.Shutdown(ctx)
}
