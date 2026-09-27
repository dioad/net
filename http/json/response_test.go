package json

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rs/zerolog"
)

func TestNewResponse(t *testing.T) {
	w := httptest.NewRecorder()
	resp := NewResponse(w)

	if resp == nil {
		t.Fatal("Expected Response to be created, got nil")
	}
	if resp.Writer != w {
		t.Error("Expected Writer to be set correctly")
	}
	if resp.logger != nil {
		t.Error("Expected logger to be nil")
	}
}

func TestNewResponseWithLogger(t *testing.T) {
	t.Parallel()
	var logOutput bytes.Buffer
	logger := zerolog.New(&logOutput)
	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)

	resp := NewResponseWithLogger(w, req, logger)

	if resp == nil {
		t.Fatal("Expected Response to be created, got nil")
	}
	if resp.Writer != w {
		t.Error("Expected Writer to be set correctly")
	}
	if resp.logger == nil {
		t.Error("Expected logger to be set")
	}

	// Trigger a log entry and verify snake_case field names.
	resp.InternalServerError(LogErr(errors.New("oops")), LogMessage("test error"))
	var entry map[string]any
	err := json.Unmarshal(logOutput.Bytes(), &entry)
	if err != nil {
		t.Fatalf("Failed to parse log output: %v", err)
	}
	if _, ok := entry["remote_addr"]; !ok {
		t.Error("Expected 'remote_addr' field in log entry, got none")
	}
	if _, ok := entry["user_agent"]; !ok {
		t.Error("Expected 'user_agent' field in log entry, got none")
	}
	if _, ok := entry["remoteAddr"]; ok {
		t.Error("Unexpected legacy 'remoteAddr' camelCase field in log entry")
	}
	if _, ok := entry["userAgent"]; ok {
		t.Error("Unexpected legacy 'userAgent' camelCase field in log entry")
	}
}

func TestNewResponseFromRequest(t *testing.T) {
	t.Parallel()
	var logOutput bytes.Buffer
	// Simulate the context logger as AddResource enriches it: request_id,
	// method, url (full pre-strip path), remote_addr, user_agent are all
	// present before the resource handler runs.
	ctxLogger := zerolog.New(&logOutput).With().
		Str("request_id", "test-req-123").
		Str("method", "GET").
		Str("url", "/app/test").
		Str("remote_addr", "127.0.0.1:12345").
		Str("user_agent", "test-agent/1.0").
		Logger()

	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)
	req = req.WithContext(ctxLogger.WithContext(req.Context()))

	resp := NewResponseFromRequest(w, req)

	require.NotNil(t, resp)
	require.NotNil(t, resp.logger)

	resp.InternalServerError(LogErr(errors.New("oops")), LogMessage("ctx test"))
	var entry map[string]any
	require.NoError(t, json.Unmarshal(logOutput.Bytes(), &entry))
	assert.Equal(t, "test-req-123", entry["request_id"])
	assert.Equal(t, "/app/test", entry["url"])
	assert.Equal(t, "127.0.0.1:12345", entry["remote_addr"])
}

func TestBadRequestWithMessage(t *testing.T) {
	w := httptest.NewRecorder()
	resp := NewResponse(w)

	resp.BadRequestWithMessage("invalid request")

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status code %d, got %d", http.StatusBadRequest, w.Code)
	}

	var result map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &result)
	if err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if result["error"] != "invalid request" {
		t.Errorf("Expected error message %q, got %q", "invalid request", result["error"])
	}
}

func TestInvalidInputWithMessage(t *testing.T) {
	w := httptest.NewRecorder()
	resp := NewResponse(w)
	err := errors.New("validation error")

	resp.InvalidInputWithMessage(err, "invalid input")

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status code %d, got %d", http.StatusBadRequest, w.Code)
	}

	var result map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if result["error"] != "invalid input" {
		t.Errorf("Expected error message %q, got %q", "invalid input", result["error"])
	}
}

func TestInternalServerErrorWithMessage(t *testing.T) {
	w := httptest.NewRecorder()
	resp := NewResponse(w)
	err := errors.New("database error")

	resp.InternalServerErrorWithMessage(err, "internal error")

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status code %d, got %d", http.StatusInternalServerError, w.Code)
	}

	var result map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if result["error"] != "internal error" {
		t.Errorf("Expected error message %q, got %q", "internal error", result["error"])
	}
}

func TestForbiddenWithMessage(t *testing.T) {
	w := httptest.NewRecorder()
	resp := NewResponse(w)

	resp.ForbiddenWithMessage("access denied")

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status code %d, got %d", http.StatusForbidden, w.Code)
	}

	var result map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &result)
	if err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if result["error"] != "access denied" {
		t.Errorf("Expected error message %q, got %q", "access denied", result["error"])
	}
}

func TestUnauthorizedWithMessage(t *testing.T) {
	w := httptest.NewRecorder()
	resp := NewResponse(w)

	resp.UnauthorizedWithMessage("authentication required")

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status code %d, got %d", http.StatusUnauthorized, w.Code)
	}

	var result map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &result)
	if err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if result["error"] != "authentication required" {
		t.Errorf("Expected error message %q, got %q", "authentication required", result["error"])
	}
}

func TestConflictWithMessage(t *testing.T) {
	w := httptest.NewRecorder()
	resp := NewResponse(w)

	resp.ConflictWithMessage("resource already exists")

	if w.Code != http.StatusConflict {
		t.Errorf("Expected status code %d, got %d", http.StatusConflict, w.Code)
	}

	var result map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &result)
	if err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if result["error"] != "resource already exists" {
		t.Errorf("Expected error message %q, got %q", "resource already exists", result["error"])
	}
}

func TestNotFoundWithMessage(t *testing.T) {
	w := httptest.NewRecorder()
	resp := NewResponse(w)

	resp.NotFoundWithMessage("resource not found")

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status code %d, got %d", http.StatusNotFound, w.Code)
	}

	var result map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &result)
	if err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if result["error"] != "resource not found" {
		t.Errorf("Expected error message %q, got %q", "resource not found", result["error"])
	}
}

func TestNotAcceptableWithMessage(t *testing.T) {
	w := httptest.NewRecorder()
	resp := NewResponse(w)

	resp.NotAcceptableWithMessage("format not acceptable")

	if w.Code != http.StatusNotAcceptable {
		t.Errorf("Expected status code %d, got %d", http.StatusNotAcceptable, w.Code)
	}

	var result map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &result)
	if err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if result["error"] != "format not acceptable" {
		t.Errorf("Expected error message %q, got %q", "format not acceptable", result["error"])
	}
}

func TestUnprocessableEntity(t *testing.T) {
	w := httptest.NewRecorder()
	resp := NewResponse(w)

	resp.UnprocessableEntity(PublicMessage("state must be \"running\" or \"stopped\""))

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)

	var result map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Equal(t, "state must be \"running\" or \"stopped\"", result["error"])
}

func TestUnprocessableEntity_DefaultMessage(t *testing.T) {
	w := httptest.NewRecorder()
	resp := NewResponse(w)

	resp.UnprocessableEntity()

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)

	var result map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Equal(t, "unprocessable entity", result["error"])
}

func TestServiceUnavailable(t *testing.T) {
	w := httptest.NewRecorder()
	resp := NewResponse(w)

	resp.ServiceUnavailable(PublicMessage("event store unavailable"))

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var result map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Equal(t, "event store unavailable", result["error"])
}

func TestServiceUnavailable_DefaultMessage(t *testing.T) {
	w := httptest.NewRecorder()
	resp := NewResponse(w)

	resp.ServiceUnavailable()

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var result map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Equal(t, "service unavailable", result["error"])
}

func TestProblem_DefaultsTypeAndStatus(t *testing.T) {
	w := httptest.NewRecorder()
	resp := NewResponse(w)

	resp.Problem(http.StatusForbidden, Problem{Title: "account quota exceeded"})

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Equal(t, "application/problem+json; charset=utf-8", w.Header().Get("Content-Type"))

	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "about:blank", body["type"])
	assert.Equal(t, "account quota exceeded", body["title"])
	assert.Equal(t, float64(http.StatusForbidden), body["status"])
	assert.NotContains(t, body, "detail")
	assert.NotContains(t, body, "instance")
}

func TestProblem_ExplicitFields(t *testing.T) {
	w := httptest.NewRecorder()
	resp := NewResponse(w)

	resp.Problem(http.StatusForbidden, Problem{
		Type:     "https://example.com/problems/quota-exceeded",
		Title:    "account quota exceeded",
		Status:   http.StatusForbidden,
		Detail:   "the account has exhausted its monthly connection quota",
		Instance: "/accounts/acc-123/quota",
	})

	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "https://example.com/problems/quota-exceeded", body["type"])
	assert.Equal(t, "account quota exceeded", body["title"])
	assert.Equal(t, float64(http.StatusForbidden), body["status"])
	assert.Equal(t, "the account has exhausted its monthly connection quota", body["detail"])
	assert.Equal(t, "/accounts/acc-123/quota", body["instance"])
}

func TestProblem_ExtensionsAreTopLevelMembers(t *testing.T) {
	w := httptest.NewRecorder()
	resp := NewResponse(w)

	resp.Problem(http.StatusForbidden, Problem{
		Title:      "account quota exceeded",
		Extensions: map[string]any{"reason": "quota_exceeded"},
	})

	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "quota_exceeded", body["reason"])
	assert.Equal(t, "account quota exceeded", body["title"])
}

func TestProblem_ExtensionsCannotOverrideStandardMembers(t *testing.T) {
	w := httptest.NewRecorder()
	resp := NewResponse(w)

	resp.Problem(http.StatusForbidden, Problem{
		Title:      "account quota exceeded",
		Extensions: map[string]any{"status": "not-a-number", "type": "hijacked"},
	})

	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, float64(http.StatusForbidden), body["status"])
	assert.Equal(t, "about:blank", body["type"])
}

func TestProblem_LogErr(t *testing.T) {
	var logOutput bytes.Buffer
	logger := zerolog.New(&logOutput)
	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)

	resp := NewResponseWithLogger(w, req, logger)
	resp.Problem(http.StatusForbidden, Problem{Title: "account quota exceeded"}, LogErr(errors.New("quota check failed")))

	assert.NotZero(t, logOutput.Len())
}

func TestProblem_DataAndPublicMessageAreDroppedWithWarning(t *testing.T) {
	var logOutput bytes.Buffer
	logger := zerolog.New(&logOutput)
	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)

	resp := NewResponseWithLogger(w, req, logger)
	resp.Problem(http.StatusForbidden, Problem{Title: "account quota exceeded"},
		Data(map[string]any{"ignored": true}), PublicMessage("ignored too"))

	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.NotContains(t, body, "ignored")
	assert.NotContains(t, body, "message")
	assert.NotZero(t, logOutput.Len(), "dropping Data()/PublicMessage() for a Problem() response should be logged")
}

func TestCreatedWithMessage(t *testing.T) {
	w := httptest.NewRecorder()
	resp := NewResponse(w)

	resp.CreatedWithMessage("resource created")

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status code %d, got %d", http.StatusCreated, w.Code)
	}

	var result map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &result)
	if err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if result["message"] != "resource created" {
		t.Errorf("Expected message %q, got %q", "resource created", result["message"])
	}
}

func TestCreatedWithURI(t *testing.T) {
	w := httptest.NewRecorder()
	resp := NewResponse(w)

	resp.CreatedWithURI("/api/resource/123")

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status code %d, got %d", http.StatusCreated, w.Code)
	}

	location := w.Header().Get("Location")
	if location != "/api/resource/123" {
		t.Errorf("Expected Location header %q, got %q", "/api/resource/123", location)
	}

	var result map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &result)
	if err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if result["uri"] != "/api/resource/123" {
		t.Errorf("Expected uri %q, got %q", "/api/resource/123", result["uri"])
	}
}

func TestCreatedWithURIAndMessage(t *testing.T) {
	w := httptest.NewRecorder()
	resp := NewResponse(w)

	resp.CreatedWithURIAndMessage("/api/resource/456", "created successfully")

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status code %d, got %d", http.StatusCreated, w.Code)
	}

	location := w.Header().Get("Location")
	if location != "/api/resource/456" {
		t.Errorf("Expected Location header %q, got %q", "/api/resource/456", location)
	}

	var result map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &result)
	if err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if result["uri"] != "/api/resource/456" {
		t.Errorf("Expected uri %q, got %q", "/api/resource/456", result["uri"])
	}
	if result["message"] != "created successfully" {
		t.Errorf("Expected message %q, got %q", "created successfully", result["message"])
	}
}

func TestNoContent(t *testing.T) {
	w := httptest.NewRecorder()
	resp := NewResponse(w)

	resp.NoContent()

	if w.Code != http.StatusNoContent {
		t.Errorf("Expected status code %d, got %d", http.StatusNoContent, w.Code)
	}

	if w.Body.Len() != 0 {
		t.Errorf("Expected empty body, got %d bytes", w.Body.Len())
	}
}

func TestNoContent_IgnoresDataAndPublicMessage(t *testing.T) {
	var logOutput bytes.Buffer
	logger := zerolog.New(&logOutput)
	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)

	resp := NewResponseWithLogger(w, req, logger)
	resp.NoContent(Data(map[string]string{"id": "1"}), PublicMessage("done"))

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Zero(t, w.Body.Len(), "a 204 response must have no body, even when Data()/PublicMessage() are passed")
	assert.NotZero(t, logOutput.Len(), "ignoring a caller-supplied Data()/PublicMessage() on a 204 should be logged, not silently dropped")
}

func TestAcceptedWithMessage(t *testing.T) {
	w := httptest.NewRecorder()
	resp := NewResponse(w)

	resp.AcceptedWithMessage("request accepted")

	if w.Code != http.StatusAccepted {
		t.Errorf("Expected status code %d, got %d", http.StatusAccepted, w.Code)
	}

	var result map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &result)
	if err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if result["message"] != "request accepted" {
		t.Errorf("Expected message %q, got %q", "request accepted", result["message"])
	}
}

func TestOK(t *testing.T) {
	w := httptest.NewRecorder()
	resp := NewResponse(w)

	data := map[string]string{
		"status": "ok",
		"id":     "123",
	}

	resp.OK(Data(data))

	if w.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
	}

	var result map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &result)
	if err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if result["status"] != "ok" {
		t.Errorf("Expected status %q, got %q", "ok", result["status"])
	}
	if result["id"] != "123" {
		t.Errorf("Expected id %q, got %q", "123", result["id"])
	}
}

func TestData(t *testing.T) {
	w := httptest.NewRecorder()
	resp := NewResponse(w)

	data := map[string]int{
		"count": 42,
	}

	resp.Data(http.StatusPartialContent, data)

	if w.Code != http.StatusPartialContent {
		t.Errorf("Expected status code %d, got %d", http.StatusPartialContent, w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		t.Errorf("Expected Content-Type to contain %q, got %q", "application/json", contentType)
	}

	var result map[string]int
	err := json.Unmarshal(w.Body.Bytes(), &result)
	if err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if result["count"] != 42 {
		t.Errorf("Expected count %d, got %d", 42, result["count"])
	}
}

func TestData_Nil(t *testing.T) {
	w := httptest.NewRecorder()
	resp := NewResponse(w)

	resp.Data(http.StatusNoContent, nil)

	if w.Code != http.StatusNoContent {
		t.Errorf("Expected status code %d, got %d", http.StatusNoContent, w.Code)
	}
}

func TestOK_StructDataWithMessage_LogsWarningWhenMessageIsDropped(t *testing.T) {
	type user struct {
		ID string `json:"id"`
	}

	var logOutput bytes.Buffer
	logger := zerolog.New(&logOutput)
	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)

	resp := NewResponseWithLogger(w, req, logger)
	resp.OK(Data(user{ID: "1"}), PublicMessage("done"))

	// A non-map payload (the overwhelmingly common case) still can't carry
	// PublicMessage today -- no wire-format change was made for a
	// combination that has zero precedent among this package's callers --
	// but the drop must now be visible in logs instead of silent.
	var result user
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Equal(t, "1", result.ID)
	assert.NotContains(t, w.Body.String(), "done")
	assert.NotZero(t, logOutput.Len(), "dropping PublicMessage for a non-map Data() payload should be logged")
}

func TestReadBody_ValidJSON(t *testing.T) {
	type TestStruct struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	jsonData := `{"name":"test","value":123}`
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/test", bytes.NewBufferString(jsonData))

	result, err := ReadBody[TestStruct](req)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result.Name != "test" {
		t.Errorf("Expected name %q, got %q", "test", result.Name)
	}
	if result.Value != 123 {
		t.Errorf("Expected value %d, got %d", 123, result.Value)
	}
}

func TestReadBody_InvalidJSON(t *testing.T) {
	type TestStruct struct {
		Name string `json:"name"`
	}

	invalidJSON := `{"name": "test"`
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/test", bytes.NewBufferString(invalidJSON))

	_, err := ReadBody[TestStruct](req)

	if err == nil {
		t.Error("Expected error for invalid JSON, got nil")
	}
}

func TestReadBody_EmptyBody(t *testing.T) {
	type TestStruct struct {
		Name string `json:"name"`
	}

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/test", bytes.NewBufferString(""))

	_, err := ReadBody[TestStruct](req)

	if err == nil {
		t.Error("Expected error for empty body, got nil")
	}
}

func TestResponseWithLogger_ErrorLogging(t *testing.T) {
	var logOutput bytes.Buffer
	logger := zerolog.New(&logOutput)
	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)

	resp := NewResponseWithLogger(w, req, logger)
	err := errors.New("test error")

	resp.InternalServerErrorWithMessage(err, "internal error occurred")

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status code %d, got %d", http.StatusInternalServerError, w.Code)
	}

	// Check that something was logged
	if logOutput.Len() == 0 {
		t.Error("Expected log output, got none")
	}
}

func TestBadRequestWithMessages(t *testing.T) {
	var logOutput bytes.Buffer
	logger := zerolog.New(&logOutput)
	w := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", nil)

	resp := NewResponseWithLogger(w, req, logger)
	resp.BadRequestWithMessages("client error", "server log message")

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status code %d, got %d", http.StatusBadRequest, w.Code)
	}

	var result map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &result)
	if err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if result["error"] != "client error" {
		t.Errorf("Expected error message %q, got %q", "client error", result["error"])
	}
}

func TestInvalidInputWithMessages(t *testing.T) {
	w := httptest.NewRecorder()
	resp := NewResponse(w)
	err := errors.New("validation error")

	resp.InvalidInputWithMessages(err, "client message", "server message")

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status code %d, got %d", http.StatusBadRequest, w.Code)
	}

	var result map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if result["error"] != "client message" {
		t.Errorf("Expected error message %q, got %q", "client message", result["error"])
	}
}

func TestInternalServerErrorWithMessages(t *testing.T) {
	w := httptest.NewRecorder()
	resp := NewResponse(w)
	err := errors.New("db error")

	resp.InternalServerErrorWithMessages(err, "client message", "server message")

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status code %d, got %d", http.StatusInternalServerError, w.Code)
	}

	var result map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if result["error"] != "client message" {
		t.Errorf("Expected error message %q, got %q", "client message", result["error"])
	}
}

func TestForbiddenWithMessages(t *testing.T) {
	w := httptest.NewRecorder()
	resp := NewResponse(w)

	resp.ForbiddenWithMessages("client forbidden", "server forbidden")

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status code %d, got %d", http.StatusForbidden, w.Code)
	}

	var result map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &result)
	if err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if result["error"] != "client forbidden" {
		t.Errorf("Expected error message %q, got %q", "client forbidden", result["error"])
	}
}

func TestUnauthorizedWithMessages(t *testing.T) {
	w := httptest.NewRecorder()
	resp := NewResponse(w)

	resp.UnauthorizedWithMessages("client auth error", "server auth error")

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status code %d, got %d", http.StatusUnauthorized, w.Code)
	}

	var result map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &result)
	if err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if result["error"] != "client auth error" {
		t.Errorf("Expected error message %q, got %q", "client auth error", result["error"])
	}
}

func TestConflictWithMessages(t *testing.T) {
	w := httptest.NewRecorder()
	resp := NewResponse(w)

	resp.ConflictWithMessages("client conflict", "server conflict")

	if w.Code != http.StatusConflict {
		t.Errorf("Expected status code %d, got %d", http.StatusConflict, w.Code)
	}

	var result map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &result)
	if err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if result["error"] != "client conflict" {
		t.Errorf("Expected error message %q, got %q", "client conflict", result["error"])
	}
}

func TestNotFoundWithMessages(t *testing.T) {
	w := httptest.NewRecorder()
	resp := NewResponse(w)

	resp.NotFoundWithMessages("client not found", "server not found")

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status code %d, got %d", http.StatusNotFound, w.Code)
	}

	var result map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &result)
	if err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if result["error"] != "client not found" {
		t.Errorf("Expected error message %q, got %q", "client not found", result["error"])
	}
}
