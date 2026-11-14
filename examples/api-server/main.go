// Package main is what a Dioad API server looks like using only dioad/net and
// dioad/lifecycle: an API listener, a management listener for the platform's
// health checks and metrics scrapes, readiness, and a drain on SIGTERM.
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"

	"github.com/dioad/lifecycle"

	diohttp "github.com/dioad/net/http"
	"github.com/dioad/net/http/json"
	"github.com/dioad/net/metrics"
)

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

// run builds the application and runs it until ctx is done or a server fails.
func run(ctx context.Context, cfg config) error {
	app, err := build(ctx, cfg)
	if err != nil {
		return err
	}

	return app.Run(ctx)
}

// build wires the application. It binds no listener and starts no goroutine;
// Run does.
func build(ctx context.Context, cfg config) (*lifecycle.App, error) {
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

	// Services start in the order they are added and stop in reverse, so the
	// management server is up before the API server comes up, and outlasts it.
	app := lifecycle.New(lifecycle.WithDrainDelay(cfg.drainDelay))
	if err := app.Add("management", management); err != nil {
		return nil, err
	}
	if err := app.Add("api", api); err != nil {
		return nil, err
	}

	// The app's readiness is not ready until both servers have started and
	// not ready once the drain begins. It is a health source of the API
	// server, so the management server reports it too.
	if err := api.AddHealthSource("readiness", app.Readiness()); err != nil {
		return nil, fmt.Errorf("failed to add readiness: %w", err)
	}
	// A dependency of the API, such as a database, is a check on it:
	// app.Readiness().AddCheck("database", ping, lifecycle.WithCacheTTL(5*time.Second)).

	return app, nil
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
