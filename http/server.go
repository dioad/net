package http

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	stdlog "log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/dioad/filter"
	"github.com/gorilla/mux"
	"github.com/pires/go-proxyproto"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/weaveworks/common/middleware"

	"github.com/dioad/net/http/auth"
	"github.com/dioad/net/http/authz/jwt"
	"github.com/dioad/net/http/pprof"
	"github.com/dioad/net/oidc"
)

// Config ...
type Config struct {
	ListenAddress           string
	EnablePrometheusMetrics bool
	EnableDebug             bool
	EnableStatus            bool
	EnableProxyProtocol     bool
	TLSConfig               *tls.Config
	AuthConfig              auth.ServerConfig
}

// Server ...
type Server struct {
	Config            Config
	Router            *mux.Router
	Logger            zerolog.Logger
	ResourceMap       map[string]Resource
	server            *http.Server
	serverInitOnce    sync.Once
	metricSet         *MetricSet
	instrument        *middleware.Instrument
	rootResource      RootResource
	ListenAddr        net.Addr
	LogHandler        HandlerWrapper
	metadataStatusMap map[string]any
}

func newDefaultServer(config Config) *Server {
	r := prometheus.NewRegistry()
	m := NewMetricSet(r)
	rtr := mux.NewRouter()

	server := &Server{
		Config:            config,
		Router:            rtr,
		ResourceMap:       make(map[string]Resource),
		metricSet:         m,
		metadataStatusMap: make(map[string]any),
	}

	return server
}

type ServerOption func(*Server)

func WithLogWriter(w io.Writer) ServerOption {
	return func(s *Server) {
		if w != nil {
			s.LogHandler = DefaultCombinedLogHandler(w)
		}
	}
}

func WithLogger(l zerolog.Logger) ServerOption {
	return func(s *Server) {
		s.Logger = l
		s.LogHandler = ZerologStructuredLogHandler(l)
	}
}

func WithOAuth2Validator(v []oidc.ValidatorConfig) ServerOption {
	return func(s *Server) {
		validator, err := oidc.NewMultiValidatorFromConfig(v)
		if err == nil {
			s.Use(jwt.NewHandler(validator).Wrap)
		}
	}
}

func WithServerAuth(cfg auth.ServerConfig) ServerOption {
	return func(s *Server) {
		h, err := auth.NewHandler(&cfg)
		if err != nil {
			s.Logger.Fatal().Err(err).Msg("error creating auth handler.")
			return
		}
		s.Use(h.Wrap)
	}
}

// NewServer ...
func NewServer(config Config, opts ...ServerOption) *Server {
	server := newDefaultServer(config)

	for _, opt := range opts {
		opt(server)
	}

	return server
}

// Deprecated: Use NewServer with WithLogger instead
func NewServerWithLogger(config Config, logger zerolog.Logger) *Server {
	return NewServer(config, WithLogger(logger))
}

func WithTelemetryInstrument(i middleware.Instrument) ServerOption {
	return func(s *Server) {
		s.instrument = &i
		s.Use(s.instrument.Wrap)
	}
}

func WithPrometheusRegistry(r prometheus.Registerer) ServerOption {
	return func(s *Server) {
		s.metricSet.Register(r)
	}
}

// AddResource ...
func (s *Server) AddResource(pathPrefix string, r Resource, middlewares ...mux.MiddlewareFunc) {
	nonNilMiddlewares := filter.FilterSlice(middlewares, func(m mux.MiddlewareFunc) bool {
		return m != nil
	})

	subrouter := s.Router.PathPrefix(pathPrefix).Subrouter()
	subrouter.Use(nonNilMiddlewares...)

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

	if s.rootResource != nil {
		s.AddHandler("/{path:.*}", s.rootResource.Index())
	}

	var router http.Handler
	router = s.Router

	if s.LogHandler != nil {
		router = s.LogHandler(router)
	}

	// uncomment if ensure that 404's get picked up by metrics
	// s.Router.NotFoundHandler = s.Router.NewRoute().HandlerFunc(http.NotFound).GetHandler()
	return router
}

func (s *Server) AddHandler(path string, handler http.Handler) {
	s.AddHandlerFunc(path, handler.ServeHTTP)
}

func (s *Server) AddHandlerFunc(path string, handler http.HandlerFunc) {
	h := handler

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

func (s *Server) Use(middlewares ...mux.MiddlewareFunc) {
	nonNilMiddlewares := filter.FilterSlice(middlewares, func(m mux.MiddlewareFunc) bool {
		return m != nil
	})
	s.Router.Use(nonNilMiddlewares...)
}

func (s *Server) AddStatusStaticMetadataItem(key string, value any) {
	s.metadataStatusMap[key] = value
}

func (s *Server) aggregateStatusHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		statusMap := make(map[string]any)
		httpStatus := http.StatusOK

		routeStatusMap := make(map[string]any)
		for path, resource := range s.ResourceMap {
			if sr, ok := resource.(StatusResource); ok {

				status, err := sr.Status()
				if err != nil {
					httpStatus = http.StatusInternalServerError
				}
				routeStatusMap[path] = status
			}
		}
		statusMap["Routes"] = routeStatusMap
		statusMap["Metadata"] = s.metadataStatusMap

		w.Header().Set("Content-Type", "text/json; charset=utf-8") // normal header

		w.WriteHeader(httpStatus)

		encoder := json.NewEncoder(w)
		err := encoder.Encode(statusMap)
		if err != nil {
			s.Logger.Error().Err(err).Msg("error calling json.Encode")
		}
	}
}

func (s *Server) initialiseServer() {

	s.serverInitOnce.Do(func() {
		l := stdlog.New(s.Logger.With().Str("level", "error").Logger(), "", stdlog.Lshortfile)

		s.server = &http.Server{
			ReadTimeout:  time.Minute,
			WriteTimeout: time.Minute,
			Handler:      s.handler(),
			Addr:         s.Config.ListenAddress,
			ErrorLog:     l,
		}
	})
}

func (s *Server) ListenAndServe() error {
	return s.ListenAndServeTLS(s.Config.TLSConfig)
}

// ListenAndServeTLS ...
// tlsConfig will override any prior configuration in s.Config
func (s *Server) ListenAndServeTLS(tlsConfig *tls.Config) error {
	s.Config.TLSConfig = tlsConfig
	ln, err := net.Listen("tcp", s.Config.ListenAddress)
	if err != nil {
		return err
	}

	return s.Serve(ln)
}

func (s *Server) Serve(ln net.Listener) error {
	s.ListenAddr = ln.Addr()
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
