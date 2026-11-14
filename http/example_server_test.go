package http_test

import (
	"context"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"sync/atomic"

	"github.com/rs/zerolog"

	dhttp "github.com/dioad/net/http"
)

// greetings is a Resource: a set of routes that a server mounts under one
// path prefix.
type greetings struct{}

func (greetings) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{name}", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "hello, "+html.EscapeString(r.PathValue("name")))
	})

	return mux
}

// servedBy is a middleware that labels every response.
func servedBy(name string) dhttp.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Served-By", name)
			next.ServeHTTP(w, r)
		})
	}
}

// startGate is a health source that reports not ready until it is opened.
type startGate struct{ open atomic.Bool }

func (g *startGate) Ready(context.Context) (any, error) {
	if !g.open.Load() {
		return nil, errors.New("starting")
	}

	return "started", nil
}

// get returns the status, the X-Served-By header and the body of a GET.
func get(ctx context.Context, url string) (int, string, string) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, "", err.Error()
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, "", err.Error()
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)

	return resp.StatusCode, resp.Header.Get("X-Served-By"), string(body)
}

// A server with a resource mounted under a path prefix and a middleware that
// runs for every request. Start returns once the listener is bound, and
// Shutdown waits for the serve loop to end.
func ExampleNewServer() {
	ctx := context.Background()

	server := dhttp.NewServer(
		dhttp.Config{ListenAddress: "127.0.0.1:0"},
		dhttp.WithLogger(zerolog.Nop()),
		dhttp.WithHealth(),
	)
	server.Use(servedBy("example"))
	server.AddResource("/greetings", greetings{})

	if err := server.Start(ctx); err != nil {
		fmt.Println("start:", err)

		return
	}
	base := "http://" + server.ListenAddr.String()

	fmt.Println(get(ctx, base+"/greetings/world"))
	status, _, _ := get(ctx, base+"/health/ready")
	fmt.Println("ready:", status)
	fmt.Println("shutdown:", server.Shutdown(ctx))
	// Output:
	// 200 example hello, world
	// ready: 200
	// shutdown: <nil>
}

// An operational server puts the primary server's health endpoints on a
// listener of their own, for a platform health check that cannot present the
// credentials the primary asks for. A health source added to the primary is
// reported there.
func ExampleNewOperationalServer() {
	ctx := context.Background()

	primary := dhttp.NewServer(dhttp.Config{ListenAddress: "127.0.0.1:0"}, dhttp.WithLogger(zerolog.Nop()))
	gate := &startGate{}
	if err := primary.AddHealthSource("startup", gate); err != nil {
		fmt.Println("health source:", err)

		return
	}

	management := dhttp.NewOperationalServer(
		primary,
		dhttp.OperationalConfig{ListenAddress: "127.0.0.1:0"},
		dhttp.WithLogger(zerolog.Nop()),
	)
	if err := management.Start(ctx); err != nil {
		fmt.Println("start:", err)

		return
	}
	ready := "http://" + management.ListenAddr.String() + "/health/ready"

	status, _, _ := get(ctx, ready)
	fmt.Println("before the gate opens:", status)

	gate.open.Store(true)
	status, _, _ = get(ctx, ready)
	fmt.Println("after the gate opens:", status)

	fmt.Println("shutdown:", management.Shutdown(ctx))
	// Output:
	// before the gate opens: 503
	// after the gate opens: 200
	// shutdown: <nil>
}
