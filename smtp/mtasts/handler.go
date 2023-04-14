package mtasts

import (
	"fmt"
	"net/http"
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
		w.Write([]byte(outputPolicy))
	}, nil
}
