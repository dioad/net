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
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
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
}

// NewServer ...
func NewServer(config Config, accessLogWriter io.Writer) *Server {
	return &Server{
		Config:          config,
		ListenAddress:   config.ListenAddress,
		Router:          mux.NewRouter(),
		accessLogWriter: accessLogWriter,
		ResourceMap:     make(map[string]Resource, 0)}
}

func NewServerWithLogger(config Config, accessLogger zerolog.Logger) *Server {
	return &Server{
		Config:        config,
		ListenAddress: config.ListenAddress,
		Router:        mux.NewRouter(),
		accessLogger:  accessLogger,
		ResourceMap:   make(map[string]Resource, 0)}
}

// AddResource ...
func (s *Server) AddResource(pathPrefix string, r Resource) {
	subrouter := s.Router.PathPrefix(pathPrefix).Subrouter()
	r.RegisterRoutes(subrouter)
	s.ResourceMap[pathPrefix] = r
}

func (s *Server) handler() http.Handler {
	s.addDefaultHandlers()

	logHandler := ZerologStructuredLogHandler(s.accessLogger)
	if s.accessLogWriter != nil {
		logHandler = DefaultCombinedLogHandler(s.accessLogWriter)
	}

	return logHandler(s.Router)
}

func (s *Server) AddHandler(path string, handler http.Handler) {
	s.Router.Handle(path, handler)
}

func (s *Server) AddHandlerFunc(path string, handler http.HandlerFunc) {
	s.Router.HandleFunc(path, handler)
}

func (s *Server) addDefaultHandlers() {
	if s.Config.EnablePrometheusMetrics {
		s.Router.Handle("/metrics", promhttp.Handler())
	}

	if s.Config.EnableDebug {
		s.AddResource("/debug", pprof.NewResource(log.Logger))
	}

	if s.Config.EnableStatus {
		s.Router.Handle("/status", s.aggregateStatusHandler())
	}
}

func (s *Server) aggregateStatusHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		statusMap := make(map[string]interface{}, 0)
		httpStatus := http.StatusOK
		for path, resource := range s.ResourceMap {
			status, err := resource.Status()
			if err != nil {
				httpStatus = http.StatusInternalServerError
			}
			statusMap[path] = status
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
