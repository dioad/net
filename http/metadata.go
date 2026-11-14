package http

import (
	"net/http"
	"runtime/debug"

	"github.com/dioad/net/http/json"
)

// MetadataResource serves a server's build metadata: its version, commit and
// build date. Every Dioad server exposes the same document, so they share this
// resource rather than each carrying a copy.
type MetadataResource struct {
	// Version is the release version, such as "v1.2.3".
	Version string
	// Commit is the source revision the binary was built from.
	Commit string
	// Date is the build or commit time.
	Date string
}

// NewMetadataResource returns a MetadataResource for the given build values,
// normally set by the binary's ldflags. A value left empty is filled from the
// build information Go embeds in the binary (the main module's version, and
// the vcs.revision and vcs.time settings), so a binary built without ldflags
// still reports what it can.
func NewMetadataResource(version, commit, date string) *MetadataResource {
	r := &MetadataResource{Version: version, Commit: commit, Date: date}

	info, ok := debug.ReadBuildInfo()
	if !ok {
		return r
	}
	if r.Version == "" {
		r.Version = info.Main.Version
	}
	for _, setting := range info.Settings {
		switch {
		case setting.Key == "vcs.revision" && r.Commit == "":
			r.Commit = setting.Value
		case setting.Key == "vcs.time" && r.Date == "":
			r.Date = setting.Value
		}
	}

	return r
}

// Get returns a handler that responds with the version, commit and date.
func (r *MetadataResource) Get() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		json.NewResponseFromRequest(w, req).OK(json.Data(map[string]string{
			"version": r.Version,
			"commit":  r.Commit,
			"date":    r.Date,
		}))
	}
}

// Options returns a handler for OPTIONS requests, which says what the
// resource allows.
func (r *MetadataResource) Options() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Allow", "GET, OPTIONS")
		w.WriteHeader(http.StatusNoContent)
	}
}

// Handler returns the HTTP handler for the resource: GET and OPTIONS at its
// root, with or without a trailing slash.
func (r *MetadataResource) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", r.Get())
	mux.HandleFunc("GET /{$}", r.Get())
	mux.HandleFunc("OPTIONS /", r.Options())

	return mux
}

var _ Resource = (*MetadataResource)(nil)
