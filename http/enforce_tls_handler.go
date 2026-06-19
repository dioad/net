package http

import "net/http"

// EnforceTLSHandler wraps an HTTP handler to enforce TLS connections.
type EnforceTLSHandler struct {
	enforceTLS bool
}

// NewEnforceTLSHandler creates an EnforceTLSHandler that rejects non-TLS requests when enforce is true.
func NewEnforceTLSHandler(enforce bool) *EnforceTLSHandler {
	return &EnforceTLSHandler{enforceTLS: enforce}
}

func (h *EnforceTLSHandler) Wrap(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h.enforceTLS && r.TLS == nil {
			http.Error(w, "TLS required", http.StatusForbidden)
			return
		}

		handler.ServeHTTP(w, r)
	})
}
