package metrics

import (
	"errors"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
)

// RegisterAll registers each collector with reg. A collector that is already
// registered is skipped, so a component can register its metrics from a
// constructor that runs more than once. Any other error stops registration and
// is returned, naming the collector that failed; collectors before it stay
// registered.
func RegisterAll(reg prometheus.Registerer, collectors ...prometheus.Collector) error {
	for i, c := range collectors {
		err := reg.Register(c)
		if err == nil {
			continue
		}
		if _, ok := errors.AsType[prometheus.AlreadyRegisteredError](err); ok {
			continue
		}

		return fmt.Errorf("failed to register collector %d: %w", i, err)
	}

	return nil
}
