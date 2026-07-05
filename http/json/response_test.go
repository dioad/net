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
	req := httptest.NewRequest("GET", "/test", nil)

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
	if err := json.Unmarshal(logOutput.Bytes(), &entry); err != nil {
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
	req := httptest.NewRequest("GET", "/test", nil)
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
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
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
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
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
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
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
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
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
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
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
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if result["error"] != "format not acceptable" {
		t.Errorf("Expected error message %q, got %q", "format not acceptable", result["error"])
	}
}

func TestCreatedWithMessage(t *testing.T) {
	w := httptest.NewRecorder()
	resp := NewResponse(w)

	resp.CreatedWithMessage("resource created")

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status code %d, got %d", http.StatusCreated, w.Code)
	}

	var result map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
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
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
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
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
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

func TestAcceptedWithMessage(t *testing.T) {
	w := httptest.NewRecorder()
	resp := NewResponse(w)

	resp.AcceptedWithMessage("request accepted")

	if w.Code != http.StatusAccepted {
		t.Errorf("Expected status code %d, got %d", http.StatusAccepted, w.Code)
	}

	var result map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
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
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
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
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
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

func TestReadBody_ValidJSON(t *testing.T) {
	type TestStruct struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	jsonData := `{"name":"test","value":123}`
	req := httptest.NewRequest("POST", "/test", bytes.NewBufferString(jsonData))

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
	req := httptest.NewRequest("POST", "/test", bytes.NewBufferString(invalidJSON))

	_, err := ReadBody[TestStruct](req)

	if err == nil {
		t.Error("Expected error for invalid JSON, got nil")
	}
}

func TestReadBody_EmptyBody(t *testing.T) {
	type TestStruct struct {
		Name string `json:"name"`
	}

	req := httptest.NewRequest("POST", "/test", bytes.NewBufferString(""))

	_, err := ReadBody[TestStruct](req)

	if err == nil {
		t.Error("Expected error for empty body, got nil")
	}
}

func TestResponseWithLogger_ErrorLogging(t *testing.T) {
	var logOutput bytes.Buffer
	logger := zerolog.New(&logOutput)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)

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
	req := httptest.NewRequest("GET", "/test", nil)

	resp := NewResponseWithLogger(w, req, logger)
	resp.BadRequestWithMessages("client error", "server log message")

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status code %d, got %d", http.StatusBadRequest, w.Code)
	}

	var result map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
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
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
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
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
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
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
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
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if result["error"] != "client not found" {
		t.Errorf("Expected error message %q, got %q", "client not found", result["error"])
	}
}
