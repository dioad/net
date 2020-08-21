package http

import (
	"crypto/tls"
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"runtime"
	"time"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Config ...
type Config struct {
	ListenAddress           string
	EnablePrometheusMetrics bool
	EnableDebug             bool
}

// Server ...
type Server struct {
	Config          Config
	ListenAddress   string
	Router          *mux.Router
	AccessLogWriter io.Writer
	Resources       []Resource
}

// NewServer ...
func NewServer(config Config, accessLogWriter io.Writer) *Server {
	return &Server{
		Config:          config,
		ListenAddress:   config.ListenAddress,
		Router:          mux.NewRouter(),
		AccessLogWriter: accessLogWriter,
		Resources:       make([]Resource, 0)}
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
	if s.Config.EnablePrometheusMetrics {
		s.Router.Handle("/metrics", promhttp.Handler())
	}

	if s.Config.EnableDebug {
		s.Router.HandleFunc("/debug", s.debugHandler())
	}
	s.Router.Handle("/status", s.aggregateStatusHandler())
}

func (s *Server) debugHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		buf := make([]byte, 1<<20)
		buf = buf[:runtime.Stack(buf, true)]
		w.Write(buf)
	}
}

func (s *Server) aggregateStatusHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		statuses := make([]interface{}, 0)
		httpStatus := http.StatusOK
		for _, resource := range s.Resources {
			status, err := resource.Status()
			if err != nil {
				httpStatus = http.StatusInternalServerError
			}
			statuses = append(statuses, status)
		}

		w.Header().Set("Content-Type", "text/json; charset=utf-8") // normal header

		w.WriteHeader(httpStatus)

		encoder := json.NewEncoder(w)
		encoder.Encode(statuses)

	}
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
		return server.ListenAndServeTLS("", "")
	} else {
		log.Printf("Serving on: %v", server.Addr)
		return server.ListenAndServe()
	}
}

func (s *Server) Serve(ln net.Listener) error {
	server := &http.Server{
		Handler: s.handler(),
	}

	return server.Serve(ln)
}
