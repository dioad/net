package http

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsNilAny(t *testing.T) {
	var strPtr *string
	var slicePtr *[]string
	var example *Example

	assert.True(t, isNilAny(nil), "nil should be considered nil")
	assert.True(t, isNilAny(strPtr), "nil string pointer should be considered nil")
	assert.True(t, isNilAny(slicePtr), "nil slice pointer should be considered nil")
	assert.True(t, isNilAny(example), "nil struct pointer should be considered nil")

	strPtr = new("value")
	assert.False(t, isNilAny(strPtr), "non-nil string pointer should not be considered nil")

	nonNilSlice := []string{"value"}
	slicePtr = &nonNilSlice
	assert.False(t, isNilAny(slicePtr), "non-nil slice pointer should not be considered nil")
}

type intFieldExample struct {
	Count int
	Total uint
}

func TestUnmarshalQuery_IntFieldRejectsTrailingGarbage(t *testing.T) {
	var example intFieldExample

	err := UnmarshalQuery("Count=5%3B%20DROP%20TABLE", &example, DefaultHTTPMarshalOptions())

	assert.Error(t, err, "an int field with trailing non-digit characters must be rejected, not silently truncated to its leading digits")
}

func TestUnmarshalQuery_UintFieldRejectsTrailingGarbage(t *testing.T) {
	var example intFieldExample

	err := UnmarshalQuery("Total=10ms", &example, DefaultHTTPMarshalOptions())

	assert.Error(t, err, "a uint field with trailing non-digit characters must be rejected, not silently truncated to its leading digits")
}

func TestUnmarshalQuery_IntFieldAcceptsValidValue(t *testing.T) {
	var example intFieldExample

	err := UnmarshalQuery("Count=5&Total=10", &example, DefaultHTTPMarshalOptions())

	require.NoError(t, err)
	assert.Equal(t, 5, example.Count)
	assert.Equal(t, uint(10), example.Total)
}
