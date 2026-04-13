// Package http provides an HTTP server and client with built-in support for metrics, authentication, and structured logging.
package http

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	stdlog "log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/pires/go-proxyproto"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/cors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/weaveworks/common/middleware"

	"github.com/dioad/filter"

	auth "github.com/dioad/auth/http"
	"github.com/dioad/auth/http/middleware/jwt"
	authjwt "github.com/dioad/auth/jwt"
	"github.com/dioad/auth/oidc"
	"github.com/dioad/net/http/pprof"
)

// Config represents the configuration for an HTTP server
type Config struct {
	// ListenAddress is the address to listen on, e.g. ":8080"
	ListenAddress string
	// EnablePrometheusMetrics enables the /metrics endpoint for Prometheus metrics
	EnablePrometheusMetrics bool
	// EnableDebug enables the /debug endpoint for pprof debugging
	EnableDebug bool
	// EnableStatus enables the /status endpoint for server status
	EnableStatus bool
	// EnableProxyProtocol enables the PROXY protocol for client IP forwarding
	EnableProxyProtocol bool
	// TLSConfig is the TLS configuration for the server
	TLSConfig *tls.Config
	// AuthConfig is the authentication configuration for the server
	AuthConfig auth.ServerConfig
	// EnableHealth enables the /health/live and /health/ready endpoints for health checks
	EnableHealth bool
	// ReadHeaderTimeout is the maximum duration for reading request headers.
	// If zero, defaults to defaultReadHeaderTimeout.
	// Setting this prevents ghost TCP connections (accepted but no HTTP request sent)
	// from blocking graceful shutdown for up to 5 seconds.
	ReadHeaderTimeout time.Duration
	// IdleTimeout is the maximum duration an idle (keep-alive) connection will
	// remain open before being closed. If zero, Go's http.Server defaults to
	// ReadTimeout.
	IdleTimeout time.Duration
}

// defaultReadHeaderTimeout is applied when Config.ReadHeaderTimeout is zero.
// It prevents ghost TCP connections (accepted but headers never sent) from
// blocking graceful Shutdown for up to 5 seconds due to Go's net/http
// StateNew→StateIdle promotion logic.
const defaultReadHeaderTimeout = 10 * time.Second

// Server represents an HTTP server with various features like metrics, authentication, and resources
type Server struct {
	// Config is the server configuration
	Config Config
	// Mux is the main router for the server
	Mux *http.ServeMux
	// Logger is the logger for the server
	Logger zerolog.Logger
	// ResourceMap maps path prefixes to resources
	ResourceMap map[string]Resource
	// ListenAddr is the address the server is listening on
	ListenAddr net.Addr
	// LogHandler is the handler wrapper for logging requests
	LogHandler HandlerWrapper
	// HealthRegistry aggregates internal server health endpoints and metadata
	HealthRegistry *HealthRegistry

	// Private fields
	server         *http.Server
	serverInitOnce sync.Once
	metricSet      *MetricSet
	instrument     *middleware.Instrument
	rootResource   RootResource
	middlewares    []Middleware
}

func newDefaultServer(config Config) *Server {
	r := prometheus.NewRegistry()
	m := NewMetricSet(r)
	m.Register(r)
	mux := http.NewServeMux()

	server := &Server{
		Config:         config,
		Mux:            mux,
		ResourceMap:    make(map[string]Resource),
		metricSet:      m,
		HealthRegistry: NewHealthRegistry(log.Logger),
		middlewares:    make([]Middleware, 0),
	}

	return server
}

// ServerOption is a function that configures a Server
type ServerOption func(*Server)

// WithLogWriter returns a ServerOption that configures the server to log requests to the given writer
// using the combined log format
func WithLogWriter(w io.Writer) ServerOption {
	return func(s *Server) {
		if w != nil {
			s.LogHandler = DefaultCombinedLogHandler(w)
		}
	}
}

// WithLogger returns a ServerOption that configures the server to use the given logger
// for both server logs and request logs
func WithLogger(l zerolog.Logger) ServerOption {
	return func(s *Server) {
		s.Logger = l
		if s.LogHandler == nil {
			s.LogHandler = ZerologStructuredLogHandler(l)
		}
		s.HealthRegistry.logger = l
	}
}

// OAuth2ValidatorHandler returns a middleware that validates OAuth2 tokens using the provided configurations.
func OAuth2ValidatorHandler(v []oidc.ValidatorConfig) (Middleware, error) {
	var validators []authjwt.TokenValidator
	for _, cfg := range v {
		validator, err := oidc.NewValidatorFromConfig(&cfg)
		if err != nil {
			return nil, err
		}
		validators = append(validators, validator)
	}

	multiValidator := &authjwt.MultiValidator{Validators: validators}
	authHandler := jwt.NewHandler(multiValidator, "auth_token", log.Logger)

	// Convert from common generic wrapper standard back to pure http.Handler middleware
	return func(next http.Handler) http.Handler {
		return authHandler.Wrap(next)
	}, nil
}

// CORSHandler returns a middleware that handles Cross-Origin Resource Sharing (CORS).
func CORSHandler(options cors.Options) (Middleware, error) {
	corsMiddleware := cors.New(options)
	return corsMiddleware.Handler, nil
}

// WithOAuth2Validator returns a ServerOption that configures the server to use OAuth2 validation
// for authentication using the given validator configurations
func WithOAuth2Validator(v []oidc.ValidatorConfig) ServerOption {
	return func(s *Server) {
		handler, err := OAuth2ValidatorHandler(v)
		if err == nil {
			s.Use(handler)
		} else {
			s.Logger.Fatal().Err(err).Msg("failed to create OAuth2 validator")
		}
	}
}

// CORSAllowLocalhostOrigin returns true if the given origin is a localhost origin.
func CORSAllowLocalhostOrigin(origin string) bool {
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	host, _, _ := strings.Cut(u.Host, ":")

	return host == "localhost"
}

// WithCORS returns a ServerOption that configures the server with the given CORS options.
func WithCORS(options cors.Options) ServerOption {
	return func(s *Server) {
		if options.Logger == nil {
			corsLogger := s.Logger.With().
				Str("component", "cors").Logger()

			options.Logger = &corsLogger
		}
		handler, _ := CORSHandler(options)
		s.Use(handler)
	}
}

// WithServerAuth returns a ServerOption that configures the server to use the given
// authentication configuration
func WithServerAuth(cfg auth.ServerConfig) ServerOption {
	return func(s *Server) {
		h, err := auth.NewHandler(&cfg)
		if err != nil {
			s.Logger.Fatal().Err(err).Msg("error creating auth handler.")
			return
		}

		s.Use(func(next http.Handler) http.Handler {
			return h.Wrap(next)
		})
	}
}

// NewServer creates a new HTTP server with the given configuration and options
// Options can be used to customize the server, such as adding a logger, authentication, or metrics
func NewServer(config Config, opts ...ServerOption) *Server {
	server := newDefaultServer(config)

	for _, opt := range opts {
		opt(server)
	}

	return server
}

// WithTelemetryInstrument returns a ServerOption that configures the server to use the given
// telemetry instrument for metrics collection
func WithTelemetryInstrument(i middleware.Instrument) ServerOption {
	return func(s *Server) {
		s.ConfigureTelemetryInstrument(i)
	}
}

// ConfigureTelemetryInstrument configures the server with the given telemetry instrument
func (s *Server) ConfigureTelemetryInstrument(i middleware.Instrument) {
	s.instrument = &i
	s.Use(func(next http.Handler) http.Handler {
		return s.instrument.Wrap(next)
	})
}

// WithPrometheusRegistry returns a ServerOption that configures the server to register
// its metrics with the given Prometheus registry
func WithPrometheusRegistry(r prometheus.Registerer) ServerOption {
	return func(s *Server) {
		s.metricSet.Register(r)
	}
}

// filterNilMiddlewares removes nil middlewares from the slice
func filterNilMiddlewares(middlewares []Middleware) []Middleware {
	return filter.FilterSlice(middlewares, func(m Middleware) bool {
		return m != nil
	})
}

// AddResource adds a resource to the server at the specified path prefix
// Optional middlewares can be provided to be applied exclusively to the resource's routes.
func (s *Server) AddResource(pathPrefix string, r Resource, middlewares ...Middleware) {
	s.ResourceMap[pathPrefix] = r
	s.HealthRegistry.Register(pathPrefix, r)

	validMiddlewares := filterNilMiddlewares(middlewares)
	resourceHandler := Chain(r.Handler(), validMiddlewares...)

	// We strip the prefix to make the resource's routes relative to its mount point.
	// We handle both trailing and non-trailing slash versions to avoid unexpected
	// 404s or redirects for clients that don't support them (e.g. some POST clients).
	prefixToStrip := strings.TrimSuffix(pathPrefix, "/")

	// We use a custom handler to ensure that a leading slash is always present
	// for the inner resource handler, even if the stripped path is empty.
	h := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		p := strings.TrimPrefix(req.URL.Path, prefixToStrip)
		if p == "" || p[0] != '/' {
			req.URL.Path = "/" + p
		} else {
			req.URL.Path = p
		}
		resourceHandler.ServeHTTP(w, req)
	})

	s.Mux.Handle(prefixToStrip+"/", h)
	if prefixToStrip != "" {
		s.Mux.Handle(prefixToStrip, h)
	}
}

// AddRootResource sets the root resource for the server
// The root resource's Index method will be called for any path that doesn't match other routes
func (s *Server) AddRootResource(r RootResource) {
	s.rootResource = r
}

// handler returns the HTTP handler for the server
// It adds default handlers and the root resource handler if configured
func (s *Server) handler() http.Handler {
	var handler http.Handler = s.Mux
	handler = Chain(handler, s.middlewares...)

	if s.Config.EnablePrometheusMetrics && s.metricSet != nil {
		handler = s.metricSet.Middleware(s.Mux, handler)
	}

	if s.LogHandler != nil {
		handler = s.LogHandler(handler)
	}

	return handler
}

// AddHandler adds a handler for the specified path
func (s *Server) AddHandler(path string, handler http.Handler) {
	s.Mux.Handle(path, handler)
}

// AddHandlerFunc adds a handler function for the specified path
func (s *Server) AddHandlerFunc(path string, handler http.HandlerFunc) {
	s.Mux.HandleFunc(path, handler)
}

// addDefaultHandlers adds default handlers to the server based on configuration
func (s *Server) addDefaultHandlers() {
	if s.Config.EnablePrometheusMetrics {
		// Combine the server's private registry with the global prometheus.DefaultGatherer
		// This ensures that both internal metrics and any globally registered metrics are served.
		gatherers := prometheus.Gatherers{
			s.metricSet.registry,
			prometheus.DefaultGatherer,
		}
		s.Mux.Handle("/metrics", promhttp.HandlerFor(gatherers, promhttp.HandlerOpts{}))
	}

	if s.Config.EnableDebug {
		s.AddResource("/debug", pprof.NewResource(log.Logger))
	}

	// Mount the health registry handlers directly
	if s.Config.EnableStatus {
		s.AddHandlerFunc("GET /status", s.HealthRegistry.aggregateStatusHandler())
	}
	if s.Config.EnableHealth {
		s.AddHandlerFunc("GET /health/live", s.HealthRegistry.aggregateLivenessHandler())
		s.AddHandlerFunc("GET /health/ready", s.HealthRegistry.aggregateReadinessHandler())
	}
}

// Use adds middleware to the server's global middleware chain.
// Any nil middlewares will be filtered out. Middlewares are executed in the order added.
func (s *Server) Use(middlewares ...Middleware) {
	s.middlewares = append(s.middlewares, filterNilMiddlewares(middlewares)...)
}

// AddStatusStaticMetadataItem adds a static metadata item to the status endpoint
// These items will be included in the "Metadata" section of the status response
func (s *Server) AddStatusStaticMetadataItem(key string, value any) {
	s.HealthRegistry.AddStaticMetadata(key, value)
}

// initialiseServer initializes the HTTP server if it hasn't been initialized yet
func (s *Server) initialiseServer() {
	s.serverInitOnce.Do(func() {
		s.addDefaultHandlers()

		if s.rootResource != nil {
			s.AddHandler("/", s.rootResource.Index())
		}

		// Create a standard logger that writes to our zerolog logger
		errorLogger := stdlog.New(s.Logger.With().Str("level", "error").Logger(), "", stdlog.Lshortfile)

		readHeaderTimeout := s.Config.ReadHeaderTimeout
		if readHeaderTimeout == 0 {
			readHeaderTimeout = defaultReadHeaderTimeout
		}

		server := &http.Server{
			ReadTimeout:       time.Minute,
			ReadHeaderTimeout: readHeaderTimeout,
			WriteTimeout:      time.Minute,
			IdleTimeout:       s.Config.IdleTimeout,
			Handler:           s.handler(),
			Addr:              s.Config.ListenAddress,
			ErrorLog:          errorLogger,
		}

		s.server = server
	})
}

// ListenAndServe starts the server with the TLS configuration from the server's config
// It creates a listener on the configured address and calls Serve
func (s *Server) ListenAndServe() error {
	return s.ListenAndServeTLS(s.Config.TLSConfig)
}

// ListenAndServeTLS starts the server with the provided TLS configuration
// The tlsConfig will override any prior configuration in s.Config
// It creates a listener on the configured address and calls Serve
func (s *Server) ListenAndServeTLS(tlsConfig *tls.Config) error {
	s.Config.TLSConfig = tlsConfig
	ln, err := net.Listen("tcp", s.Config.ListenAddress)
	if err != nil {
		s.Logger.Error().Err(err).Str("address", s.Config.ListenAddress).Msg("failed to listen on address")
		return err
	}

	return s.Serve(ln)
}

// Serve starts the server with the provided listener
// It initializes the server if needed, configures TLS and proxy protocol if enabled,
// and starts serving HTTP or HTTPS requests
func (s *Server) Serve(ln net.Listener) error {
	s.ListenAddr = ln.Addr()
	s.initialiseServer()
	s.server.TLSConfig = s.Config.TLSConfig

	addr := ln.Addr()
	addrString := "missing"
	if addr != nil {
		addrString = addr.String()
	}

	s.Logger.Info().
		Str("address", addrString).
		Bool("tls_enabled", s.Config.TLSConfig != nil).
		Bool("proxy_protocol_enabled", s.Config.EnableProxyProtocol).
		Msg("starting server")

	if s.Config.EnableProxyProtocol {
		ln = &proxyproto.Listener{
			Listener:          ln,
			ReadHeaderTimeout: 10 * time.Second,
		}
		s.Logger.Debug().Msg("proxy protocol enabled")
	}

	var err error
	if s.Config.TLSConfig != nil {
		err = s.server.ServeTLS(ln, "", "")
	} else {
		err = s.server.Serve(ln)
	}

	if err != nil && !errors.Is(err, http.ErrServerClosed) && !errors.Is(err, net.ErrClosed) {
		s.Logger.Error().Err(err).Msg("server error")
	} else {
		s.Logger.Info().Msg("server stopped")
	}

	return err
}

// ServeTLS is a convenience method that calls Serve
// It's provided for compatibility with the http.Server interface
func (s *Server) ServeTLS(ln net.Listener) error {
	return s.Serve(ln)
}

// RegisterOnShutdown registers a function to be called when the server is shutting down
// This function will be called in a new goroutine when Shutdown is called
func (s *Server) RegisterOnShutdown(f func()) {
	s.initialiseServer()
	s.server.RegisterOnShutdown(f)
}

// Shutdown gracefully shuts down the server without interrupting any active connections
// It waits for all connections to finish or for the context to be canceled
func (s *Server) Shutdown(ctx context.Context) error {
	s.initialiseServer()
	return s.server.Shutdown(ctx)
}
