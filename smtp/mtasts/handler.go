package mtasts

import (
	"fmt"
	"net/http"

	"github.com/rs/zerolog"
)

// HTTPHandler returns a handler serving p's policy document. p is a
// pointer, not a snapshot: the policy is rendered fresh on every request,
// so mutations made to *p after HTTPHandler returns (e.g. rotating MX
// hosts) are reflected on the next request.
func HTTPHandler(p *Policy) (http.HandlerFunc, error) {
	// Validate once up front so an already-broken policy fails fast at
	// construction, in addition to the same check on every request below.
	if _, err := FormatPolicy(p); err != nil {
		return nil, fmt.Errorf("failed to format policy: %w", err)
	}

	return func(w http.ResponseWriter, r *http.Request) {
		outputPolicy, err := FormatPolicy(p)
		if err != nil {
			zerolog.Ctx(r.Context()).Error().Err(err).Msg("failed to format mta-sts policy")
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("Cache-Control", "max-age=3600")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(outputPolicy)); err != nil {
			zerolog.Ctx(r.Context()).Error().Err(err).Msg("failed to write mta-sts policy response")
		}
	}, nil
}
