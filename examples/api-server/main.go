// Package main is what a Dioad API server looks like using only dioad/net:
// an API listener, a management listener for the platform's health checks
// and metrics scrapes, a readiness gate, and a drain on SIGTERM.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"

	diohttp "github.com/dioad/net/http"
	"github.com/dioad/net/http/json"
	"github.com/dioad/net/metrics"
)

const shutdownTimeout = 10 * time.Second

// config is what the process is started with.
type config struct {
	listenAddress     string
	managementAddress string
	drainDelay        time.Duration
}

func main() {
	var cfg config
	flag.StringVar(&cfg.listenAddress, "listen", ":8080", "address of the API listener")
	flag.StringVar(&cfg.managementAddress, "management-listen", "127.0.0.1:8081", "address of the management listener")
	flag.DurationVar(&cfg.drainDelay, "drain-delay", 5*time.Second, "how long to report not ready before closing the listeners")
	flag.Parse()

	logger := zerolog.New(os.Stderr).With().Timestamp().Str("service", "api-server").Logger()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := run(logger.WithContext(ctx), cfg)
	stop()
	if err != nil {
		logger.Fatal().Err(err).Msg("server failed")
	}
}

// run builds the application and serves until ctx is done or a server fails.
func run(ctx context.Context, cfg config) error {
	app, err := build(ctx, cfg)
	if err != nil {
		return err
	}

	return app.run(ctx)
}

// application is the wired process: its two servers and its readiness gate.
type application struct {
	api        *diohttp.Server
	management *diohttp.Server
	readiness  *readinessGate
	drainDelay time.Duration
}

// build wires the application. It binds no listener and starts no goroutine;
// run does.
func build(ctx context.Context, cfg config) (*application, error) {
	logger := zerolog.Ctx(ctx)

	greetingsServed := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "api_server_greetings_total",
		Help: "Greetings served.",
	})
	// Registered before any server starts, so that the first scrape sees it.
	if err := metrics.RegisterAll(prometheus.DefaultRegisterer, greetingsServed); err != nil {
		return nil, fmt.Errorf("failed to register metrics: %w", err)
	}

	api := diohttp.NewServer(
		diohttp.Config{ListenAddress: cfg.listenAddress},
		diohttp.WithLogger(*logger),
		diohttp.WithComponent("api"),
	)
	api.AddResource("/greetings", greetings{served: greetingsServed})

	readiness := &readinessGate{}
	if err := api.AddHealthSource("readiness", readiness); err != nil {
		return nil, fmt.Errorf("failed to add readiness gate: %w", err)
	}

	// The management server reports the API server's health sources and
	// request metrics without any authentication the API server may add, so
	// it listens where only the platform can reach it.
	management := diohttp.NewOperationalServer(
		api,
		diohttp.OperationalConfig{
			ListenAddress:           cfg.managementAddress,
			EnablePrometheusMetrics: true,
			EnableStatus:            true,
		},
		diohttp.WithLogger(*logger),
		diohttp.WithComponent("management"),
	)

	return &application{api: api, management: management, readiness: readiness, drainDelay: cfg.drainDelay}, nil
}

// run starts both servers, reports ready, and blocks until ctx is done or a
// server fails. It then drains and shuts both servers down.
func (a *application) run(ctx context.Context) error {
	logger := zerolog.Ctx(ctx)

	// The management server starts first, so that the platform can already
	// see "not ready" while the API server comes up.
	if err := a.management.Start(ctx); err != nil {
		return err
	}
	if err := a.api.Start(ctx); err != nil {
		return errors.Join(err, a.shutdown(ctx))
	}
	a.readiness.set(stateReady)
	logger.Info().Msg("ready")

	select {
	case <-ctx.Done():
		a.drain(logger)

		return a.shutdown(ctx)
	case err := <-a.api.Failed():
		// Nothing is left to drain once a server has failed.
		return errors.Join(fmt.Errorf("api server failed: %w", err), a.shutdown(ctx))
	case err := <-a.management.Failed():
		return errors.Join(fmt.Errorf("management server failed: %w", err), a.shutdown(ctx))
	}
}

// drain reports not ready and waits, so that the platform stops sending
// requests before the listeners close. Requests that still arrive are served.
func (a *application) drain(logger *zerolog.Logger) {
	a.readiness.set(stateDraining)
	logger.Info().Dur("drain_delay", a.drainDelay).Msg("draining")
	time.Sleep(a.drainDelay)
}

// shutdown stops the API server, then the management server, so that health
// stays observable until the API server's requests have finished.
func (a *application) shutdown(ctx context.Context) error {
	// ctx is already done when the process was interrupted, and the
	// shutdown must outlive it.
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
	defer cancel()

	return errors.Join(a.api.Shutdown(shutdownCtx), a.management.Shutdown(shutdownCtx))
}

const (
	stateStarting int32 = iota
	stateReady
	stateDraining
)

// readinessGate is a health source that is ready only between startup and
// drain. /health/ready on both servers reports it.
type readinessGate struct{ state atomic.Int32 }

func (g *readinessGate) set(state int32) { g.state.Store(state) }

// Ready implements the readiness half of a health source.
func (g *readinessGate) Ready(context.Context) (any, error) {
	switch g.state.Load() {
	case stateReady:
		return "ready", nil
	case stateDraining:
		return nil, errors.New("draining")
	default:
		return nil, errors.New("starting")
	}
}

// greetings is the API's one resource.
type greetings struct{ served prometheus.Counter }

// Handler implements the http.Resource interface. Its routes are relative to
// the prefix the resource is mounted at.
func (g greetings) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{name}", g.greet)

	return mux
}

func (g greetings) greet(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	g.served.Inc()

	// LogField puts the name on the request's access log line.
	json.NewResponseFromRequest(w, r).OK(
		json.Data(map[string]any{"greeting": "hello, " + name}),
		json.LogField("greeting_name", name),
	)
}
