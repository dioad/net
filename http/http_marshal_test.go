package http

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsNilAny(t *testing.T) {
	var strPtr *string
	var slicePtr *[]string
	var example *Example

	assert.True(t, isNilAny(nil), "nil should be considered nil")
	assert.True(t, isNilAny(strPtr), "nil string pointer should be considered nil")
	assert.True(t, isNilAny(slicePtr), "nil slice pointer should be considered nil")
	assert.True(t, isNilAny(example), "nil struct pointer should be considered nil")

	nonNilStr := "value"
	strPtr = &nonNilStr
	assert.False(t, isNilAny(strPtr), "non-nil string pointer should not be considered nil")

	nonNilSlice := []string{"value"}
	slicePtr = &nonNilSlice
	assert.False(t, isNilAny(slicePtr), "non-nil slice pointer should not be considered nil")
}
