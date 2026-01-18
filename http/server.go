// Package http provides an HTTP server and client with built-in support for metrics, authentication, and structured logging.
package http

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	stdlog "log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/pires/go-proxyproto"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/cors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/weaveworks/common/middleware"

	"github.com/dioad/filter"

	"github.com/dioad/net/http/auth"
	"github.com/dioad/net/http/authz/jwt"
	"github.com/dioad/net/http/pprof"
	"github.com/dioad/net/oidc"
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
}

// Server represents an HTTP server with various features like metrics, authentication, and resources
type Server struct {
	// Config is the server configuration
	Config Config
	// Router is the main router for the server
	Router *mux.Router
	// Logger is the logger for the server
	Logger zerolog.Logger
	// ResourceMap maps path prefixes to resources
	ResourceMap map[string]Resource
	// ListenAddr is the address the server is listening on
	ListenAddr net.Addr
	// LogHandler is the handler wrapper for logging requests
	LogHandler HandlerWrapper

	// Private fields
	server            *http.Server
	serverInitOnce    sync.Once
	metricSet         *MetricSet
	instrument        *middleware.Instrument
	rootResource      RootResource
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
		s.LogHandler = ZerologStructuredLogHandler(l)
	}
}

// OAuth2ValidatorHandler returns a middleware that validates OAuth2 tokens using the provided configurations.
func OAuth2ValidatorHandler(v []oidc.ValidatorConfig) (mux.MiddlewareFunc, error) {
	validator, err := oidc.NewMultiValidatorFromConfig(v)
	if err != nil {
		return nil, err
	}

	authHandler := jwt.NewHandler(validator, "auth_token")
	return authHandler.Wrap, nil
}

// CORSHandler returns a middleware that handles Cross-Origin Resource Sharing (CORS).
func CORSHandler(options cors.Options) (mux.MiddlewareFunc, error) {
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

// // wrapWithOptionsMethodBypass creates middleware that allows OPTIONS requests to pass through
// // without authentication, while still applying the authentication middleware to other methods.
// func wrapWithOptionsMethodBypass(authMiddleware auth.Middleware) mux.MiddlewareFunc {
// 	return func(next http.Handler) http.Handler {
// 		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 			if r.Method == http.MethodOptions || authMiddleware == nil {
// 				// If the request is an OPTIONS request, we bypass authentication
// 				// and just call the next handler directly.
// 				next.ServeHTTP(w, r)
// 				return
// 			}
//
// 			authMiddleware.Wrap(next).ServeHTTP(w, r)
// 		})
// 	}
// }

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

		s.Use(h.Wrap)
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
		s.instrument = &i
		s.Use(s.instrument.Wrap)
	}
}

// WithPrometheusRegistry returns a ServerOption that configures the server to register
// its metrics with the given Prometheus registry
func WithPrometheusRegistry(r prometheus.Registerer) ServerOption {
	return func(s *Server) {
		s.metricSet.Register(r)
	}
}

// filterNilMiddlewares removes nil middlewares from the slice
func filterNilMiddlewares(middlewares []mux.MiddlewareFunc) []mux.MiddlewareFunc {
	return filter.FilterSlice(middlewares, func(m mux.MiddlewareFunc) bool {
		return m != nil
	})
}

// AddResource adds a resource to the server at the specified path prefix
// The resource can implement either PathResource or DefaultResource to register its routes
// Optional middlewares can be provided to be applied to the resource's routes
func (s *Server) AddResource(pathPrefix string, r Resource, middlewares ...mux.MiddlewareFunc) {
	subrouter := s.Router.PathPrefix(pathPrefix).Subrouter()
	subrouter.Use(filterNilMiddlewares(middlewares)...)

	if rp, ok := r.(PathResource); ok {
		rp.RegisterRoutesWithPrefix(subrouter, pathPrefix)
	} else if rp, ok := r.(DefaultResource); ok {
		rp.RegisterRoutes(subrouter)
	}
	s.ResourceMap[pathPrefix] = r
}

// AddRootResource sets the root resource for the server
// The root resource's Index method will be called for any path that doesn't match other routes
func (s *Server) AddRootResource(r RootResource) {
	s.rootResource = r
	// s.AddResource("/", r)
}

// handler returns the HTTP handler for the server
// It adds default handlers and the root resource handler if configured
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

// AddHandler adds a handler for the specified path
func (s *Server) AddHandler(path string, handler http.Handler) {
	s.AddHandlerFunc(path, handler.ServeHTTP)
}

// AddHandlerFunc adds a handler function for the specified path
func (s *Server) AddHandlerFunc(path string, handler http.HandlerFunc) {
	s.Router.HandleFunc(path, handler)
}

// addDefaultHandlers adds default handlers to the server based on configuration
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

// Use adds middleware to the server's router
// Any nil middlewares will be filtered out
func (s *Server) Use(middlewares ...mux.MiddlewareFunc) {
	s.Router.Use(filterNilMiddlewares(middlewares)...)
}

// AddStatusStaticMetadataItem adds a static metadata item to the status endpoint
// These items will be included in the "Metadata" section of the status response
func (s *Server) AddStatusStaticMetadataItem(key string, value any) {
	s.metadataStatusMap[key] = value
}

// aggregateStatusHandler returns a handler that aggregates status information from all resources
func (s *Server) aggregateStatusHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		statusMap := make(map[string]any)
		httpStatus := http.StatusOK
		var statusErrors []error

		// Collect status from all resources that implement StatusResource
		routeStatusMap := make(map[string]any)
		for path, resource := range s.ResourceMap {
			if sr, ok := resource.(StatusResource); ok {
				status, err := sr.Status()
				if err != nil {
					httpStatus = http.StatusInternalServerError
					statusErrors = append(statusErrors, err)
					s.Logger.Error().Err(err).Str("path", path).Msg("error getting resource status")
				}
				routeStatusMap[path] = status
			}
		}
		statusMap["Routes"] = routeStatusMap
		statusMap["Metadata"] = s.metadataStatusMap

		// Set appropriate content type for JSON
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(httpStatus)

		// Encode the status map as JSON
		encoder := json.NewEncoder(w)
		err := encoder.Encode(statusMap)
		if err != nil {
			s.Logger.Error().Err(err).Msg("error encoding status response")
			// We've already written the status code, so we can't change it now
			// Just log the error and return
		}

		// Log a summary of the status request
		logEvent := s.Logger.Info()
		if len(statusErrors) > 0 {
			logEvent = s.Logger.Error().Int("error_count", len(statusErrors))
		}
		logEvent.Int("status_code", httpStatus).
			Int("resource_count", len(routeStatusMap)).
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Msg("status request processed")
	}
}

// initialiseServer initializes the HTTP server if it hasn't been initialized yet
func (s *Server) initialiseServer() {
	s.serverInitOnce.Do(func() {
		// Create a standard logger that writes to our zerolog logger
		errorLogger := stdlog.New(s.Logger.With().Str("level", "error").Logger(), "", stdlog.Lshortfile)

		server := &http.Server{
			ReadTimeout:  time.Minute,
			WriteTimeout: time.Minute,
			Handler:      s.handler(),
			Addr:         s.Config.ListenAddress,
			ErrorLog:     errorLogger,
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
