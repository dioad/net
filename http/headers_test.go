package http

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMergedHTTPHeader_DoesNotMutateArgument(t *testing.T) {
	base := http.Header{}
	base.Set("X-Existing", "original")

	merged := MergedHTTPHeader(base, map[string]string{"X-New": "value"})

	assert.Equal(t, "original", base.Get("X-Existing"), "the base header argument must not be mutated by the caller's own clone-and-return contract")
	assert.Empty(t, base.Get("X-New"), "the base header argument must not gain the merged entries")

	assert.Equal(t, "original", merged.Get("X-Existing"))
	assert.Equal(t, "value", merged.Get("X-New"))
}

func TestCreateHTTPHeaderFromMap(t *testing.T) {
	got := CreateHTTPHeaderFromMap(map[string]string{"X-Foo": "bar"})

	assert.Equal(t, "bar", got.Get("X-Foo"))
}
