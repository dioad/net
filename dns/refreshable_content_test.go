package dns

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRefreshableContent_Render_FetchesOnce(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	var c refreshableContent

	fetch := func() (string, error) {
		calls.Add(1)
		return "first", nil
	}

	require.NoError(t, c.Render(fetch, false, 0))
	require.NoError(t, c.Render(fetch, false, 0))

	assert.Equal(t, "first", c.Value())
	assert.Equal(t, int32(1), calls.Load(), "fetch should only run once across two Render calls")
}

func TestRefreshableContent_Render_PropagatesFetchError(t *testing.T) {
	t.Parallel()

	var c refreshableContent
	wantErr := errors.New("boom")

	err := c.Render(func() (string, error) { return "", wantErr }, false, 0)
	require.ErrorIs(t, err, wantErr)
	assert.Empty(t, c.Value(), "a failed fetch must not populate Value")
}

func TestRefreshableContent_Render_RetriesAfterFailedFetch(t *testing.T) {
	t.Parallel()

	var c refreshableContent
	var calls atomic.Int32

	fetch := func() (string, error) {
		if calls.Add(1) == 1 {
			return "", errors.New("boom")
		}
		return "second", nil
	}

	require.Error(t, c.Render(fetch, false, 0))
	require.NoError(t, c.Render(fetch, false, 0))
	assert.Equal(t, "second", c.Value())
}

func TestRefreshableContent_Value_EmptyBeforeRender(t *testing.T) {
	t.Parallel()

	var c refreshableContent
	assert.Empty(t, c.Value())
}

// TestRefreshableContent_Render_AutoRefresh_NoRace exercises the same
// concurrent write/read pattern TLSARecord/TemplatedFileTXTRecord depend on,
// at the shared-type level.
func TestRefreshableContent_Render_AutoRefresh_NoRace(t *testing.T) {
	var c refreshableContent

	fetch := func() (string, error) {
		return "value", nil
	}

	require.NoError(t, c.Render(fetch, true, 5*time.Millisecond))

	deadline := time.After(200 * time.Millisecond)
	for {
		select {
		case <-deadline:
			assert.Equal(t, "value", c.Value())
			return
		default:
			_ = c.Value()
		}
	}
}
