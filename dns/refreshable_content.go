package dns

import (
	"sync"
	"time"
)

// refreshableContent holds a string value that is fetched lazily on first
// Render and, optionally, kept fresh afterward by a background ticker. It is
// the shared implementation behind TLSARecord and TemplatedFileTXTRecord.
//
// refreshableContent holds its mutable state behind a lazily-allocated
// pointer so a struct embedding it by value stays a plain, copyable value
// type. This matters because callers (e.g. mapstructure decode, range loops,
// by-value function parameters) may copy these records by value several
// times before Render is ever called on the final address; embedding a
// sync.Mutex directly, without the pointer indirection, turns every one of
// those copies into a go vet copylocks violation.
type refreshableContent struct {
	state *refreshableState
}

// refreshableState is the mutable state behind refreshableContent.state.
// It must only ever be referenced through a pointer, never copied by value,
// since it embeds a sync.Mutex.
type refreshableState struct {
	mu          sync.Mutex
	contents    string
	autoRefresh autoRefresher
}

// Render ensures contents is populated by calling fetch, unless it was
// already populated by an earlier Render call. If autoRefresh is true, it
// additionally starts (at most once per instance) a background ticker that
// recalls fetch every period and updates contents -- see autoRefresher.
func (r *refreshableContent) Render(fetch func() (string, error), autoRefresh bool, period time.Duration) error {
	if r.state == nil {
		r.state = &refreshableState{}
	}

	r.state.mu.Lock()
	empty := r.state.contents == ""
	r.state.mu.Unlock()

	if empty {
		if err := r.refetch(fetch); err != nil {
			return err
		}
	}

	if autoRefresh {
		r.state.autoRefresh.start(period, func() { _ = r.refetch(fetch) })
	}

	return nil
}

// refetch calls fetch and, on success, stores its result.
func (r *refreshableContent) refetch(fetch func() (string, error)) error {
	contents, err := fetch()
	if err != nil {
		return err
	}
	r.state.mu.Lock()
	r.state.contents = contents
	r.state.mu.Unlock()
	return nil
}

// Value returns the current contents, or "" if Render has not yet been
// called, or has not yet succeeded.
func (r *refreshableContent) Value() string {
	if r.state == nil {
		return ""
	}
	r.state.mu.Lock()
	defer r.state.mu.Unlock()
	return r.state.contents
}
