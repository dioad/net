package http

import "net/http"

// EnforceTLSHandler wraps an HTTP handler to enforce TLS connections.
type EnforceTLSHandler struct {
	EnforceTLS bool
}

func (h *EnforceTLSHandler) Wrap(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h.EnforceTLS && r.TLS == nil {
			http.Error(w, "TLS required", http.StatusForbidden)
			return
		}

		handler.ServeHTTP(w, r)
	})
}
