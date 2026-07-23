package mtasts

import (
	"fmt"
	"net/http"

	"github.com/rs/zerolog"
)

func HTTPHandler(p *Policy) (http.HandlerFunc, error) {
	outputPolicy, err := FormatPolicy(p)
	if err != nil {
		return nil, fmt.Errorf("failed to format policy: %w", err)
	}

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("Cache-Control", "max-age=3600")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(outputPolicy)); err != nil {
			zerolog.Ctx(r.Context()).Error().Err(err).Msg("failed to write mta-sts policy response")
		}
	}, nil
}
