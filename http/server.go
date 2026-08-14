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

	"github.com/dioad/filter"

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
	mux    *http.ServeMux
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
	server           *http.Server
	serverInitOnce   sync.Once
	rootRouteOnce    sync.Once
	metricSet        *MetricSet
	metricsGatherers prometheus.Gatherers
	instrument       Instrument
	rootResource     RootResource
	middlewares      []Middleware
}

func newDefaultServer(config Config) *Server {
	r := prometheus.NewRegistry()
	m := NewMetricSet(r)
	mux := http.NewServeMux()

	server := &Server{
		Config:           config,
		mux:              mux,
		ResourceMap:      make(map[string]Resource),
		metricSet:        m,
		metricsGatherers: prometheus.Gatherers{m.registry, prometheus.DefaultGatherer},
		HealthRegistry:   NewHealthRegistry(log.Logger),
		middlewares:      make([]Middleware, 0),
	}

	server.Use(RequestIDMiddleware)

	return server
}

// ServerOption is a function that configures a Server
type ServerOption func(*Server)

// Instrument wraps an http.Handler with telemetry instrumentation.
type Instrument interface {
	Wrap(handler http.Handler) http.Handler
}

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

// CORSHandler returns a middleware that handles Cross-Origin Resource Sharing (CORS).
func CORSHandler(options cors.Options) Middleware {
	corsMiddleware := cors.New(options)
	return corsMiddleware.Handler
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
			options.Logger = new(s.Logger.With().
				Str("component", "cors").Logger())
		}
		s.Use(CORSHandler(options))
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
func WithTelemetryInstrument(i Instrument) ServerOption {
	return func(s *Server) {
		s.ConfigureTelemetryInstrument(i)
	}
}

// ConfigureTelemetryInstrument configures the server with the given telemetry instrument
func (s *Server) ConfigureTelemetryInstrument(i Instrument) {
	s.instrument = i
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

// WithMetricsGatherers replaces the default gatherers used for the /metrics endpoint.
// The default gathers from the server's own private registry and prometheus.DefaultGatherer.
func WithMetricsGatherers(gatherers ...prometheus.Gatherer) ServerOption {
	return func(s *Server) {
		s.metricsGatherers = gatherers
	}
}

// MetricSet returns the server's HTTP instrumentation metric set.
func (s *Server) MetricSet() *MetricSet {
	return s.metricSet
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
	rawPrefixToStrip := (&url.URL{Path: prefixToStrip}).EscapedPath()

	// We use a request clone rather than mutating req.URL.Path directly so
	// that any deferred logging middleware (e.g. hlog.AccessHandler) that
	// reads r.URL after ServeHTTP returns still sees the original, unstripped
	// path. This mirrors how the stdlib http.StripPrefix behaves.
	h := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		p := strings.TrimPrefix(req.URL.Path, prefixToStrip)
		if p == "" || p[0] != '/' {
			p = "/" + p
		}

		rp := ""
		// RawPath stores the encoded form of Path, so we strip the escaped
		// mount prefix, verify the remaining candidate still unescapes to the
		// stripped Path, and only preserve it when it carries encoding
		// information that differs from Path's default escaped form.
		if req.URL.RawPath != "" {
			rpCandidate := strings.TrimPrefix(req.URL.RawPath, rawPrefixToStrip)
			unescaped, err := url.PathUnescape(rpCandidate)
			escapedPath := (&url.URL{Path: p}).EscapedPath()
			if err == nil && unescaped == p && rpCandidate != escapedPath {
				rp = rpCandidate
			}
		}

		// Inject the original (pre-strip) request fields into the context logger
		// so that all handlers and service-layer code that calls
		// zerolog.Ctx(ctx) carry method, url, remote_addr, and user_agent
		// without each handler needing to add them individually.
		// This must happen before the clone so that r2 shares the same
		// context and therefore inherits the enrichment automatically.
		zerolog.Ctx(req.Context()).UpdateContext(func(c zerolog.Context) zerolog.Context {
			return c.Str("method", req.Method).
				Str("url", req.URL.Redacted()).
				Str("host", req.Host).
				Str("remote_addr", req.RemoteAddr).
				Str("user_agent", req.UserAgent())
		})

		r2 := new(http.Request)
		*r2 = *req
		r2.URL = new(url.URL)
		*r2.URL = *req.URL
		r2.URL.Path = p
		r2.URL.RawPath = rp

		resourceHandler.ServeHTTP(w, r2)
	})

	s.mux.Handle(prefixToStrip+"/", h)
	if prefixToStrip != "" {
		s.mux.Handle(prefixToStrip, h)
	}
}

// AddRootResource sets the root resource for the server and registers its
// "/" route. The root resource's Index method will be called for any path
// that doesn't match other routes.
//
// Safe to call at any point before the server starts actually accepting
// requests, regardless of whether some other method has already triggered
// initialiseServer (RegisterOnShutdown and Shutdown do, besides
// Serve/ListenAndServe): rootRouteOnce ensures "/" is registered exactly
// once, whichever of AddRootResource or initialiseServer runs first, rather
// than only ever inside initialiseServer's own one-time setup - a caller
// that finishes configuring the server (mounting resources, then calling
// AddRootResource) after something else already triggered initialisation
// would otherwise have "/" silently, permanently unregistered. "/" is never
// registered at all for servers that never call AddRootResource, so this
// doesn't affect callers that register their own catch-all pattern (e.g.
// "/{path...}") directly - registering "/" unconditionally would panic via
// net/http.ServeMux's duplicate-pattern check against such a caller.
func (s *Server) AddRootResource(r RootResource) {
	s.rootResource = r
	s.registerRootRoute()
}

func (s *Server) registerRootRoute() {
	s.rootRouteOnce.Do(func() {
		s.AddHandler("/", s.rootResourceHandler())
	})
}

// rootResourceHandler returns a handler for "/" that reads s.rootResource
// live, on every request, instead of being resolved once when the "/" route
// is registered - so a later AddRootResource call (replacing the resource)
// takes effect immediately, with no re-registration needed.
func (s *Server) rootResourceHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.rootResource == nil {
			http.NotFound(w, r)
			return
		}
		s.rootResource.Index()(w, r)
	}
}

// handler returns the HTTP handler for the server
// It adds default handlers and the root resource handler if configured
func (s *Server) handler() http.Handler {
	var handler http.Handler = s.mux
	handler = Chain(handler, s.middlewares...)

	if s.metricSet != nil {
		handler = s.metricSet.Middleware(s.mux, handler)
	}

	if s.LogHandler != nil {
		handler = s.LogHandler(handler)
	}

	return handler
}

// AddHandler adds a handler for the specified path
func (s *Server) AddHandler(path string, handler http.Handler) {
	s.mux.Handle(path, handler)
}

// AddHandlerFunc adds a handler function for the specified path
func (s *Server) AddHandlerFunc(path string, handler http.HandlerFunc) {
	s.mux.HandleFunc(path, handler)
}

// addDefaultHandlers adds default handlers to the server based on configuration
func (s *Server) addDefaultHandlers() {
	if s.Config.EnablePrometheusMetrics {
		s.mux.Handle("/metrics", promhttp.HandlerFor(s.metricsGatherers, promhttp.HandlerOpts{}))
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

// initialiseServer initializes the HTTP server if it hasn't been initialized yet.
// Guarded by serverInitOnce, so this can genuinely only run once - but several
// methods besides Serve/ListenAndServe also call it (RegisterOnShutdown,
// Shutdown), and any of those can fire before a caller has finished calling
// AddRootResource. Root-resource dispatch is registered as a handler that
// reads s.rootResource live, per request (see rootResourceHandler), rather
// than baking in a single Index() closure at Once-time, specifically so
// AddRootResource's call-time relative to those other methods doesn't matter.
func (s *Server) initialiseServer() {
	s.serverInitOnce.Do(func() {
		s.addDefaultHandlers()

		// Only registers "/" if a root resource has already been set by this
		// point - see AddRootResource's doc comment. A caller that never
		// calls AddRootResource at all must never have "/" registered here:
		// some servers register their own catch-all pattern directly (e.g.
		// "/{path...}"), which net/http.ServeMux would panic on as a
		// duplicate of an unconditionally-registered "/".
		if s.rootResource != nil {
			s.registerRootRoute()
		}

		errorLogger := stdlog.New(&tlsHandshakeErrorFilter{logger: s.Logger}, "", stdlog.Lshortfile)

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
