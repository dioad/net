package metrics

import (
	"errors"
	"fmt"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func counterOpts(name string) prometheus.CounterOpts {
	return prometheus.CounterOpts{Name: name, Help: name}
}

func TestRegisterAll_RegistersEveryCollector(t *testing.T) {
	t.Parallel()
	reg := prometheus.NewRegistry()

	require.NoError(t, RegisterAll(reg, prometheus.NewCounter(counterOpts("a_total")), prometheus.NewCounter(counterOpts("b_total"))))

	families, err := reg.Gather()
	require.NoError(t, err)
	assert.Len(t, families, 2)
}

func TestRegisterAll_ToleratesACollectorThatIsAlreadyRegistered(t *testing.T) {
	t.Parallel()
	reg := prometheus.NewRegistry()
	first := prometheus.NewCounter(counterOpts("shared_total"))
	require.NoError(t, RegisterAll(reg, first))

	require.NoError(t, RegisterAll(reg, prometheus.NewCounter(counterOpts("shared_total")), prometheus.NewCounter(counterOpts("other_total"))))

	families, err := reg.Gather()
	require.NoError(t, err)
	assert.Len(t, families, 2, "the new collector after the duplicate is still registered")
}

// failingRegisterer fails every registration with err.
type failingRegisterer struct {
	prometheus.Registerer

	err error
}

func (f failingRegisterer) Register(prometheus.Collector) error { return f.err }

func TestRegisterAll_ReturnsAnyOtherError(t *testing.T) {
	t.Parallel()
	errBoom := errors.New("boom")

	err := RegisterAll(failingRegisterer{err: errBoom}, prometheus.NewCounter(counterOpts("a_total")))

	require.ErrorIs(t, err, errBoom)
	assert.ErrorContains(t, err, "collector 0")
}

func ExampleRegisterAll() {
	reg := prometheus.NewRegistry()
	requests := prometheus.NewCounter(prometheus.CounterOpts{Name: "requests_total", Help: "Requests served."})

	fmt.Println(RegisterAll(reg, requests))
	// Registering the same collector again is tolerated.
	fmt.Println(RegisterAll(reg, requests))
	// Output:
	// <nil>
	// <nil>
}
