package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func serveMetadata(t *testing.T, r *MetadataResource, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	r.Handler().ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), method, path, nil))

	return rec
}

func TestMetadataResource_ServesTheBuildValues(t *testing.T) {
	t.Parallel()
	r := NewMetadataResource("v1.2.3", "abc123", "2026-11-14")

	rec := serveMetadata(t, r, http.MethodGet, "/")

	require.Equal(t, http.StatusOK, rec.Code)
	var body map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), "body: %s", rec.Body.String())
	assert.Equal(t, map[string]string{"version": "v1.2.3", "commit": "abc123", "date": "2026-11-14"}, body)
}

func TestMetadataResource_AnswersOptions(t *testing.T) {
	t.Parallel()
	rec := serveMetadata(t, NewMetadataResource("v1", "c", "d"), http.MethodOptions, "/")

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, "GET, OPTIONS", rec.Header().Get("Allow"))
}

func TestMetadataResource_RejectsOtherMethods(t *testing.T) {
	t.Parallel()
	rec := serveMetadata(t, NewMetadataResource("v1", "c", "d"), http.MethodPost, "/")

	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

func TestNewMetadataResource_FillsEmptyValuesFromTheBuild(t *testing.T) {
	t.Parallel()
	r := NewMetadataResource("", "", "")

	// A test binary carries a main module version, so Version is never left
	// empty; the vcs settings depend on how it was built, so they are not
	// asserted.
	assert.NotEmpty(t, r.Version)
}

func TestNewMetadataResource_KeepsValuesItIsGiven(t *testing.T) {
	t.Parallel()
	r := NewMetadataResource("v9", "deadbeef", "yesterday")

	assert.Equal(t, MetadataResource{Version: "v9", Commit: "deadbeef", Date: "yesterday"}, *r)
}

func ExampleMetadataResource() {
	resource := NewMetadataResource("v1.2.3", "abc123", "2026-11-14")

	rec := httptest.NewRecorder()
	resource.Handler().ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil))

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		fmt.Println("decode:", err)

		return
	}
	fmt.Println(rec.Code, body["version"], body["commit"], body["date"])
	// Output: 200 v1.2.3 abc123 2026-11-14
}
