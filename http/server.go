package http

import (
	"crypto/tls"
	"io"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Config ...
type Config struct {
	ListenAddress string
}

// Server ...
type Server struct {
	ListenAddress   string
	Router          *mux.Router
	AccessLogWriter io.Writer
	EnablePrometheusMetrics bool
	EnableDebug bool
	Resources []Resource
}

// NewServer ...
func NewServer(config *Config, accessLogWriter io.Writer) *Server {
	return &Server{
		ListenAddress:   config.ListenAddress,
		Router:          mux.NewRouter(),
		AccessLogWriter: accessLogWriter,
		Resources: 	make([]Resource, 0), }
}

// AddResource ...
func (s *Server) AddResource(pathPrefix string, r Resource) {
	subrouter := s.Router.PathPrefix(pathPrefix).Subrouter()
	r.RegisterRoutes(subrouter)
	s.Resources = append(s.Resources, r)
}

func (s *Server) handler() http.Handler {
	s.addDefaultHandlers()
	return handlers.CombinedLoggingHandler(s.AccessLogWriter, s.Router)
}

func (s *Server) AddHandleFunc(path string, handler http.HandlerFunc) {
	s.Router.HandleFunc(path, handler)
}

func (s *Server) addDefaultHandlers() {
	if s.EnablePrometheusMetrics {
		s.Router.Handle("/metrics", promhttp.Handler())
	}

	//generate /status

//	if s.EnableDebug {
//		s.Router.HandleFunc("/debug/goroutines", handleDebugGoroutines)
//	}
}

func (s *Server) ListenAndServe() error {
	return s.ListenAndServeTLS(nil)
}

// ListenAndServe ...
func (s *Server) ListenAndServeTLS(tlsConfig *tls.Config) error {
	server := &http.Server{
		TLSConfig:    tlsConfig,
		ReadTimeout:  time.Minute,
		WriteTimeout: time.Minute,
		Handler:      s.handler(),
		Addr:         s.ListenAddress,
	}

	if tlsConfig != nil {
		log.Printf("TLS serving on: %v", server.Addr)
		return server.ListenAndServeTLS("","")
	} else {
		log.Printf("Serving on: %v", server.Addr)
		return server.ListenAndServe()
	}
}

func (s *Server) Serve(ln net.Listener) error {
	server := &http.Server{
		Handler:      s.handler(),
	}

	return server.Serve(ln)
}
