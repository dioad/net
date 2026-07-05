// Package json provides utilities for handling JSON requests and responses.
package json

import (
	"encoding/json"
	"maps"
	"net/http"

	"github.com/rs/zerolog"
)

// Response simplifies sending structured JSON responses and logging errors.
type Response struct {
	Writer http.ResponseWriter
	// Request http.Request
	logger *zerolog.Logger
}

// responseOption represents a configuration option for responses.
type responseOption interface {
	apply(*responseConfig)
}

// responseConfig holds the merged configuration from all applied options.
type responseConfig struct {
	data          any
	logErr        error
	logMessage    string
	publicMessage string
	headers       map[string]string
}

// Private option implementations

type withError struct{ err error }

func (w withError) apply(cfg *responseConfig) { cfg.logErr = w.err }

type withData struct{ data any }

func (w withData) apply(cfg *responseConfig) { cfg.data = w.data }

type withLogMessage struct{ msg string }

func (w withLogMessage) apply(cfg *responseConfig) { cfg.logMessage = w.msg }

type withPublicMessage struct{ msg string }

func (w withPublicMessage) apply(cfg *responseConfig) { cfg.publicMessage = w.msg }

type withLocation struct{ uri string }

func (w withLocation) apply(cfg *responseConfig) {
	cfg.headers["Location"] = w.uri
}

type withHeader struct {
	key   string
	value string
}

func (w withHeader) apply(cfg *responseConfig) {
	cfg.headers[w.key] = w.value
}

// Public option factory functions

// LogErr includes an underlying error for server-side logging.
func LogErr(err error) responseOption {
	return withError{err}
}

// LogMessage provides a custom message for server logs (separate from public message).
func LogMessage(msg string) responseOption {
	return withLogMessage{msg}
}

// PublicMessage sets the message sent to the client (overrides default).
func PublicMessage(msg string) responseOption {
	return withPublicMessage{msg}
}

// Data includes structured data in the response.
func Data(data any) responseOption {
	return withData{data}
}

// Location sets the Location header (typically for 201 Created responses).
func Location(uri string) responseOption {
	return withLocation{uri}
}

// Header sets a custom response header.
func Header(key, value string) responseOption {
	return withHeader{key, value}
}

// NewResponse creates a new Response helper with the provided ResponseWriter.
func NewResponse(w http.ResponseWriter) *Response {
	return &Response{
		Writer: w,
	}
}

// NewResponseWithLogger creates a new Response helper with a logger that includes request metadata.
//
// Deprecated: Use NewResponseFromRequest in HTTP handlers to automatically inherit the
// request-scoped context logger (carrying request_id, principal, etc.).
// NewResponseWithLogger remains useful when an explicit logger is needed (e.g. tests).
func NewResponseWithLogger(w http.ResponseWriter, r *http.Request, l zerolog.Logger) *Response {
	logger := l.With().
		Str("method", r.Method).
		Str("url", r.URL.Redacted()).
		Str("remote_addr", r.RemoteAddr).
		Str("user_agent", r.UserAgent()).
		Logger()
	return &Response{
		Writer: w,
		logger: &logger,
	}
}

// NewResponseFromRequest creates a Response that logs using the zerolog logger stored in
// r's context. All context fields — request_id, principal, auth_source, method, url
// (the full pre-strip path), remote_addr, and user_agent — are present because
// AddResource injects them into the context logger before calling the resource handler.
//
// Prefer this over NewResponseWithLogger in HTTP handlers.
func NewResponseFromRequest(w http.ResponseWriter, r *http.Request) *Response {
	return &Response{
		Writer: w,
		logger: zerolog.Ctx(r.Context()),
	}
}

// respondWithStatus sends a response with the given status code and applied options.
func (r *Response) respondWithStatus(code int, defaultMessage string, opts ...responseOption) {
	cfg := &responseConfig{
		publicMessage: defaultMessage,
		headers:       make(map[string]string),
	}

	// Apply all options
	for _, opt := range opts {
		opt.apply(cfg)
	}

	// Log error if provided
	if cfg.logErr != nil {
		msg := cfg.logMessage
		if msg == "" {
			msg = defaultMessage
		}
		r.logError(cfg.logErr, msg)
	}

	// Build response body
	var body any
	if cfg.data != nil {
		body = r.mergeResponseData(cfg.data, cfg.publicMessage, code)
	} else if cfg.publicMessage != "" {
		if isErrorStatus(code) {
			body = map[string]string{"error": cfg.publicMessage}
		} else {
			body = map[string]string{"message": cfg.publicMessage}
		}
	}

	// Apply headers
	for k, v := range cfg.headers {
		r.Writer.Header().Set(k, v)
	}

	// Send response
	r.Data(code, body)
}

// mergeResponseData combines structured data with message if needed.
func (r *Response) mergeResponseData(data any, message string, code int) any {
	m, ok := data.(map[string]any)
	if !ok {
		return data
	}

	result := make(map[string]any)
	maps.Copy(result, m)

	if message != "" {
		key := "message"
		if isErrorStatus(code) {
			key = "error"
		}
		result[key] = message
	}

	return result
}

// isErrorStatus checks if a status code is an error (4xx or 5xx).
func isErrorStatus(code int) bool {
	return code >= 400
}

// Semantic error response functions

// BadRequest sends a 400 Bad Request response.
func (r *Response) BadRequest(opts ...responseOption) {
	r.respondWithStatus(http.StatusBadRequest, "bad request", opts...)
}

// Unauthorized sends a 401 Unauthorized response.
func (r *Response) Unauthorized(opts ...responseOption) {
	r.respondWithStatus(http.StatusUnauthorized, "unauthorized", opts...)
}

// Forbidden sends a 403 Forbidden response.
func (r *Response) Forbidden(opts ...responseOption) {
	r.respondWithStatus(http.StatusForbidden, "forbidden", opts...)
}

// NotFound sends a 404 Not Found response.
func (r *Response) NotFound(opts ...responseOption) {
	r.respondWithStatus(http.StatusNotFound, "not found", opts...)
}

// Conflict sends a 409 Conflict response.
func (r *Response) Conflict(opts ...responseOption) {
	r.respondWithStatus(http.StatusConflict, "conflict", opts...)
}

// InternalServerError sends a 500 Internal Server Error response.
func (r *Response) InternalServerError(opts ...responseOption) {
	r.respondWithStatus(http.StatusInternalServerError, "internal server error", opts...)
}

// NotAcceptable sends a 406 Not Acceptable response.
func (r *Response) NotAcceptable(opts ...responseOption) {
	r.respondWithStatus(http.StatusNotAcceptable, "not acceptable", opts...)
}

// InvalidInput sends a 400 Bad Request response for invalid input.
func (r *Response) InvalidInput(opts ...responseOption) {
	r.respondWithStatus(http.StatusBadRequest, "invalid input", opts...)
}

// Semantic success response functions

// OK sends a 200 OK response.
func (r *Response) OK(opts ...responseOption) {
	r.respondWithStatus(http.StatusOK, "", opts...)
}

// Created sends a 201 Created response.
func (r *Response) Created(opts ...responseOption) {
	r.respondWithStatus(http.StatusCreated, "created", opts...)
}

// Accepted sends a 202 Accepted response.
func (r *Response) Accepted(opts ...responseOption) {
	r.respondWithStatus(http.StatusAccepted, "accepted", opts...)
}

// WithStatus sends a response with a custom status code.
func (r *Response) WithStatus(code int, opts ...responseOption) {
	r.respondWithStatus(code, "", opts...)
}

// NoContent sends a 204 No Content response.
func (r *Response) NoContent(opts ...responseOption) {
	r.respondWithStatus(http.StatusNoContent, "", opts...)
}

// Deprecated: Use BadRequest() with options instead.
func (r *Response) BadRequestWithMessage(message string) {
	r.BadRequest(PublicMessage(message))
}

// Deprecated: Use BadRequest() with options instead.
func (r *Response) BadRequestWithMessages(responseMessage, logMessage string) {
	r.BadRequest(LogMessage(logMessage), PublicMessage(responseMessage))
}

// Deprecated: Use InvalidInput() with options instead.
func (r *Response) InvalidInputWithMessage(err error, message string) {
	r.InvalidInput(PublicMessage(message), LogErr(err))
}

// Deprecated: Use InvalidInput() with options instead.
func (r *Response) InvalidInputWithMessages(err error, responseMessage, logMessage string) {
	r.InvalidInput(PublicMessage(responseMessage), LogErr(err), LogMessage(logMessage))
}

// Deprecated: Use InternalServerError() with options instead.
func (r *Response) InternalServerErrorWithMessage(err error, message string) {
	r.InternalServerError(LogErr(err), PublicMessage(message))
}

// Deprecated: Use InternalServerError() with options instead.
func (r *Response) InternalServerErrorWithMessages(err error, responseMessage string, logMessage string) {
	r.InternalServerError(PublicMessage(responseMessage), LogErr(err), LogMessage(logMessage))
}

// Deprecated: Use Forbidden() with options instead.
func (r *Response) ForbiddenWithMessages(responseMessage, logMessage string) {
	r.Forbidden(PublicMessage(responseMessage), LogMessage(logMessage))
}

// Deprecated: Use Forbidden() with options instead.
func (r *Response) ForbiddenWithMessage(message string) {
	r.Forbidden(PublicMessage(message), LogMessage(message))
}

// Deprecated: Use Unauthorized() with options instead.
func (r *Response) UnauthorizedWithMessages(responseMessage, logMessage string) {
	r.Unauthorized(PublicMessage(responseMessage), LogMessage(logMessage))
}

// Deprecated: Use Unauthorized() with options instead.
func (r *Response) UnauthorizedWithMessage(message string) {
	r.Unauthorized(PublicMessage(message), LogMessage(message))
}

// Deprecated: Use Conflict() with options instead.
func (r *Response) ConflictWithMessage(message string) {
	r.Conflict(PublicMessage(message))
}

// Deprecated: Use Conflict() with options instead.
func (r *Response) ConflictWithMessages(responseMessage, logMessage string) {
	r.Conflict(PublicMessage(responseMessage), LogMessage(logMessage))
}

func (r *Response) logError(err error, message string) {
	if r.logger != nil {
		r.logger.Error().Err(err).Msg(message)
	}
}

// Deprecated: Use NotFound() with options instead.
func (r *Response) NotFoundWithMessage(message string) {
	r.NotFound(PublicMessage(message))
}

// Deprecated: Use NotFound() with options instead.
func (r *Response) NotFoundWithMessages(responseMessage, logMessage string) {
	r.NotFound(PublicMessage(responseMessage), LogMessage(logMessage))
}

// Deprecated: Use NotAcceptable() with options instead.
func (r *Response) NotAcceptableWithMessage(message string) {
	r.NotAcceptable(PublicMessage(message))
}

// Deprecated: Use NotAcceptable() with options instead.
func (r *Response) NotAcceptableWithMessages(responseMessage, logMessage string) {
	r.NotAcceptable(PublicMessage(responseMessage), LogMessage(logMessage))
}

// Deprecated: Use Created() with options instead.
func (r *Response) CreatedWithMessage(message string) {
	r.Created(PublicMessage(message))
}

// Deprecated: Use Created() with Location() option instead.
func (r *Response) CreatedWithURI(uri string) {
	r.Created(Location(uri), PublicMessage(uri), Data(map[string]any{
		"uri": uri,
	}))
}

// Deprecated: Use Created() with Location() and PublicMessage() options instead.
func (r *Response) CreatedWithURIAndMessage(uri string, message string) {
	r.Created(Location(uri), Data(map[string]any{"uri": uri}), PublicMessage(message))
}

// Deprecated: Use Accepted() with options instead.
func (r *Response) AcceptedWithMessage(message string) {
	r.Accepted(PublicMessage(message))
}

// Data sends a JSON response with the specified status code and data.
func (r *Response) Data(status int, data any) {
	r.Writer.Header().Set("Content-Type", "application/json; charset=utf-8") // normal header
	encoder := json.NewEncoder(r.Writer)
	r.Writer.WriteHeader(status)

	if data != nil {
		err := encoder.Encode(data)
		if err != nil {
			r.logError(err, "error encoding response")
		}
	}
}

// ReadBody reads and decodes the JSON request body into the specified type.
// It automatically closes the request body.
func ReadBody[T any](req *http.Request) (T, error) {
	var t T
	decoder := json.NewDecoder(req.Body)
	err := decoder.Decode(&t)
	if err != nil {
		_ = req.Body.Close()
		return t, err
	}
	return t, req.Body.Close()
}
