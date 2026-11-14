// Package main is a daemon with no listener: a worker that does a unit of work
// on an interval, run by dioad/lifecycle. It starts, stops cleanly on SIGTERM,
// and exits non-zero if the worker fails.
package main

import (
	"context"
	"errors"
	"flag"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/rs/zerolog"

	"github.com/dioad/lifecycle"
)

func main() {
	interval := flag.Duration("interval", time.Second, "how often the worker does a unit of work")
	failAfter := flag.Int("fail-after", 0, "fail after this many units of work, to show a failure ending the process (0 never fails)")
	flag.Parse()

	logger := zerolog.New(os.Stderr).With().Timestamp().Str("service", "worker-daemon").Logger()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := run(logger.WithContext(ctx), newWorker(logger, *interval, *failAfter))
	stop()
	if err != nil {
		logger.Fatal().Err(err).Msg("daemon failed")
	}
}

// run runs the worker until ctx is done or the worker fails.
func run(ctx context.Context, w *worker) error {
	app := lifecycle.New()
	if err := app.Add("worker", w); err != nil {
		return err
	}

	return app.Run(ctx)
}

// worker does a unit of work on an interval. It is a lifecycle.Service, and
// reports its own failure, so the app can end the process, by implementing
// Failed.
type worker struct {
	logger    zerolog.Logger
	interval  time.Duration
	failAfter int

	units  atomic.Int64
	cancel context.CancelFunc
	done   chan struct{}
	failed chan error
}

func newWorker(logger zerolog.Logger, interval time.Duration, failAfter int) *worker {
	return &worker{
		logger:    logger,
		interval:  interval,
		failAfter: failAfter,
		done:      make(chan struct{}),
		failed:    make(chan error, 1),
	}
}

// Start launches the loop and returns. The loop's context is not derived from
// ctx, which is the process's and is done as soon as SIGTERM arrives, before
// the app has drained: the worker stops when Shutdown says so.
func (w *worker) Start(ctx context.Context) error {
	loopCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	w.cancel = cancel

	go w.loop(loopCtx)
	w.logger.Info().Dur("interval", w.interval).Msg("worker started")

	return nil
}

// Shutdown stops the loop and waits for the unit of work in progress, until
// ctx is done.
func (w *worker) Shutdown(ctx context.Context) error {
	w.cancel()
	select {
	case <-w.done:
		w.logger.Info().Int64("units", w.units.Load()).Msg("worker stopped")

		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Failed reports, at most once, the error that ended the loop.
func (w *worker) Failed() <-chan error {
	return w.failed
}

func (w *worker) loop(ctx context.Context) {
	defer close(w.done)

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			units := w.units.Add(1)
			w.logger.Info().Int64("unit", units).Msg("did a unit of work")
			if w.failAfter > 0 && units >= int64(w.failAfter) {
				w.failed <- errors.New("work failed")

				return
			}
		}
	}
}
