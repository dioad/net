package dns

import (
	"sync"
	"time"
)

// autoRefresher runs fetchFn on a ticker until the process exits. start is
// safe to call more than once on the same instance; only the first call
// takes effect, so repeated Render calls (e.g. across a rebuild of the
// owning record) each get their own independent autoRefresher rather than
// racing to set a shared flag.
type autoRefresher struct {
	once sync.Once
}

// start begins calling fetchFn every period, in a new goroutine, until the
// process exits. There is currently no lifecycle signal (context or
// otherwise) reaching Render, so the goroutine is not stopped early; each
// new autoRefresher (one per Render-triggering record rebuild) still only
// ever starts one ticker, closing the unsynchronized-flag race and the
// per-call duplicate-goroutine risk an earlier version of this mechanism had.
func (a *autoRefresher) start(period time.Duration, fetchFn func()) {
	a.once.Do(func() {
		go func() {
			ticker := time.NewTicker(period)
			defer ticker.Stop()
			for range ticker.C {
				fetchFn()
			}
		}()
	})
}
