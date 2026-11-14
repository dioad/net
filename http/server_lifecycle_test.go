package http

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dioad/lifecycle"
)

// freeAddress returns a loopback address nothing is listening on.
func freeAddress(t *testing.T) string {
	t.Helper()
	ln, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := ln.Addr().String()
	require.NoError(t, ln.Close())

	return addr
}

func newLifecycleServer(t *testing.T, config Config) *Server {
	t.Helper()
	server := NewServer(config, WithLogger(zerolog.Nop()))
	server.AddHandlerFunc("GET /ping", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	return server
}

func TestServer_Start_ServesOnTheConfiguredAddress(t *testing.T) {
	t.Parallel()
	addr := freeAddress(t)
	server := newLifecycleServer(t, Config{ListenAddress: addr})

	require.NoError(t, server.Start(t.Context()))
	t.Cleanup(func() { _ = server.Shutdown(context.Background()) })

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://"+addr+"/ping", nil)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err, "the listener is bound by the time Start returns")
	_ = resp.Body.Close()
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	require.NoError(t, server.Live(t.Context()))
}

func TestServer_Start_ReturnsTheBindError(t *testing.T) {
	t.Parallel()
	blocker, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = blocker.Close() })
	server := newLifecycleServer(t, Config{ListenAddress: blocker.Addr().String()})

	err = server.Start(t.Context())

	require.ErrorContains(t, err, "failed to listen on")
	require.NoError(t, server.Live(t.Context()), "a server that never served is not dead")
	require.NoError(t, server.Shutdown(t.Context()), "a failed Start leaves nothing to wait for")
}

func TestServer_Start_CanOnlyBeCalledOnce(t *testing.T) {
	t.Parallel()
	server := newLifecycleServer(t, Config{ListenAddress: freeAddress(t)})
	require.NoError(t, server.Start(t.Context()))
	t.Cleanup(func() { _ = server.Shutdown(context.Background()) })

	require.ErrorContains(t, server.Start(t.Context()), "already started")
}

func TestServer_Shutdown_WaitsForTheServeLoopAfterStart(t *testing.T) {
	t.Parallel()
	server := newLifecycleServer(t, Config{ListenAddress: freeAddress(t)})
	require.NoError(t, server.Start(t.Context()))

	require.NoError(t, server.Shutdown(t.Context()))

	select {
	case err := <-server.Failed():
		t.Fatalf("an orderly shutdown is not a failure: %v", err)
	default:
	}
	require.NoError(t, server.Live(t.Context()), "a stopped server is not dead")
}

func TestServer_Shutdown_WithoutStartDoesNotWait(t *testing.T) {
	t.Parallel()
	server := newLifecycleServer(t, Config{})

	require.NoError(t, server.Shutdown(t.Context()))
}

func TestServer_Failed_ReportsAServeLoopThatEndedUnexpectedly(t *testing.T) {
	t.Parallel()
	// An unknown policy makes Serve return before it accepts anything.
	server := newLifecycleServer(t, Config{
		ListenAddress:       freeAddress(t),
		EnableProxyProtocol: true,
		ProxyProtocolPolicy: ProxyProtocolPolicy("nonsense"),
	})
	require.NoError(t, server.Start(t.Context()))

	var failure error
	select {
	case failure = <-server.Failed():
	case <-time.After(5 * time.Second):
		t.Fatal("the failure was not reported")
	}
	require.Error(t, failure)

	require.ErrorIs(t, server.Live(t.Context()), failure, "Live reports what Failed did")
	require.ErrorIs(t, server.Shutdown(t.Context()), failure, "Shutdown surfaces the serve error")
}

func ExampleServer_Start() {
	server := NewServer(Config{ListenAddress: "127.0.0.1:0"}, WithLogger(zerolog.Nop()))
	if err := server.Start(context.Background()); err != nil {
		fmt.Println("start:", err)

		return
	}

	fmt.Println("live:", server.Live(context.Background()))
	fmt.Println("shutdown:", server.Shutdown(context.Background()))
	// Output:
	// live: <nil>
	// shutdown: <nil>
}

func TestServer_Start_RecordsTheBoundAddress(t *testing.T) {
	t.Parallel()
	server := newLifecycleServer(t, Config{ListenAddress: "127.0.0.1:0"})

	require.NoError(t, server.Start(t.Context()))
	t.Cleanup(func() { _ = server.Shutdown(context.Background()) })

	status, _ := getPath(t, server.ListenAddr.String(), "/ping")
	assert.Equal(t, http.StatusNoContent, status)
}

// A *Server is a lifecycle.Service, with Failed and Live, and needs no adapter.
var (
	_ lifecycle.Service  = (*Server)(nil)
	_ lifecycle.Failer   = (*Server)(nil)
	_ lifecycle.Liveness = (*Server)(nil)
)

func TestServer_RunsUnderALifecycleApp(t *testing.T) {
	t.Parallel()
	api := newLifecycleServer(t, Config{ListenAddress: "127.0.0.1:0"})
	management := newLifecycleServer(t, Config{ListenAddress: "127.0.0.1:0"})
	app := lifecycle.New()
	require.NoError(t, app.Add("management", management))
	require.NoError(t, app.Add("api", api))
	ctx, cancel := context.WithCancel(t.Context())
	result := make(chan error, 1)
	go func() { result <- app.Run(ctx) }()

	require.Eventually(t, func() bool {
		_, err := app.Readiness().Ready(t.Context())

		return err == nil
	}, 5*time.Second, time.Millisecond)
	status, _ := getPath(t, api.ListenAddr.String(), "/ping")
	assert.Equal(t, http.StatusNoContent, status)
	require.NoError(t, app.Live(t.Context()))

	cancel()
	select {
	case err := <-result:
		require.NoError(t, err, "a clean stop of both servers")
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return")
	}
}

func TestServer_AServeFailureEndsALifecycleApp(t *testing.T) {
	t.Parallel()
	management := newLifecycleServer(t, Config{ListenAddress: "127.0.0.1:0"})
	// An unknown policy makes Serve return before it accepts anything.
	api := newLifecycleServer(t, Config{
		ListenAddress:       "127.0.0.1:0",
		EnableProxyProtocol: true,
		ProxyProtocolPolicy: ProxyProtocolPolicy("nonsense"),
	})
	app := lifecycle.New()
	require.NoError(t, app.Add("management", management))
	require.NoError(t, app.Add("api", api))

	var err error
	within(t, func() { err = app.Run(t.Context()) })

	failure, ok := errors.AsType[*lifecycle.ServiceError](err)
	require.True(t, ok, "Run returns the failure of the serve loop")
	assert.Equal(t, "api", failure.Service)
	require.ErrorContains(t, err, `unknown proxy protocol policy "nonsense"`)
}

// within fails the test if fn does not return in time.
func within(t *testing.T, fn func()) {
	t.Helper()

	done := make(chan struct{})
	go func() {
		defer close(done)
		fn()
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("did not return in time")
	}
}
