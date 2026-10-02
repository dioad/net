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

// ResponseOption represents a configuration option for responses.
type ResponseOption interface {
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
func LogErr(err error) ResponseOption {
	return withError{err}
}

// LogMessage provides a custom message for server logs (separate from public message).
func LogMessage(msg string) ResponseOption {
	return withLogMessage{msg}
}

// PublicMessage sets the message sent to the client (overrides default).
func PublicMessage(msg string) ResponseOption {
	return withPublicMessage{msg}
}

// Data includes structured data in the response.
func Data(data any) ResponseOption {
	return withData{data}
}

// Location sets the Location header (typically for 201 Created responses).
func Location(uri string) ResponseOption {
	return withLocation{uri}
}

// Header sets a custom response header.
func Header(key, value string) ResponseOption {
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
func (r *Response) respondWithStatus(code int, defaultMessage string, opts ...ResponseOption) {
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

	r.dropBodyForNoContent(cfg, code)
	body := r.buildResponseBody(cfg, code)

	// Apply headers
	for k, v := range cfg.headers {
		r.Writer.Header().Set(k, v)
	}

	// Send response
	r.Data(code, body)
}

// dropBodyForNoContent clears cfg's body fields when code is 204 No
// Content, which HTTP requires to have no body. Whatever the caller
// supplied is dropped rather than silently written.
func (r *Response) dropBodyForNoContent(cfg *responseConfig, code int) {
	if code != http.StatusNoContent || (cfg.data == nil && cfg.publicMessage == "") {
		return
	}

	r.logWarn("Data()/PublicMessage() is ignored on a 204 No Content response, which must not have a body")
	cfg.data = nil
	cfg.publicMessage = ""
}

// buildResponseBody builds the response body from cfg, preferring
// structured data over a bare public message.
func (r *Response) buildResponseBody(cfg *responseConfig, code int) any {
	if cfg.data != nil {
		return r.mergeResponseData(cfg.data, cfg.publicMessage, code)
	}

	if cfg.publicMessage == "" {
		return nil
	}

	if isErrorStatus(code) {
		return map[string]string{"error": cfg.publicMessage}
	}

	return map[string]string{"message": cfg.publicMessage}
}

// mergeResponseData combines structured data with message if needed.
func (r *Response) mergeResponseData(data any, message string, code int) any {
	m, ok := data.(map[string]any)
	if !ok {
		if message != "" {
			r.logWarn("PublicMessage is dropped: Data() payload is not a map[string]any, so it cannot be merged with a message")
		}

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
func (r *Response) BadRequest(opts ...ResponseOption) {
	r.respondWithStatus(http.StatusBadRequest, "bad request", opts...)
}

// Unauthorized sends a 401 Unauthorized response.
func (r *Response) Unauthorized(opts ...ResponseOption) {
	r.respondWithStatus(http.StatusUnauthorized, "unauthorized", opts...)
}

// Forbidden sends a 403 Forbidden response.
func (r *Response) Forbidden(opts ...ResponseOption) {
	r.respondWithStatus(http.StatusForbidden, "forbidden", opts...)
}

// NotFound sends a 404 Not Found response.
func (r *Response) NotFound(opts ...ResponseOption) {
	r.respondWithStatus(http.StatusNotFound, "not found", opts...)
}

// Conflict sends a 409 Conflict response.
func (r *Response) Conflict(opts ...ResponseOption) {
	r.respondWithStatus(http.StatusConflict, "conflict", opts...)
}

// InternalServerError sends a 500 Internal Server Error response.
func (r *Response) InternalServerError(opts ...ResponseOption) {
	r.respondWithStatus(http.StatusInternalServerError, "internal server error", opts...)
}

// NotAcceptable sends a 406 Not Acceptable response.
func (r *Response) NotAcceptable(opts ...ResponseOption) {
	r.respondWithStatus(http.StatusNotAcceptable, "not acceptable", opts...)
}

// InvalidInput sends a 400 Bad Request response for invalid input.
func (r *Response) InvalidInput(opts ...ResponseOption) {
	r.respondWithStatus(http.StatusBadRequest, "invalid input", opts...)
}

// UnprocessableEntity sends a 422 Unprocessable Entity response.
func (r *Response) UnprocessableEntity(opts ...ResponseOption) {
	r.respondWithStatus(http.StatusUnprocessableEntity, "unprocessable entity", opts...)
}

// ServiceUnavailable sends a 503 Service Unavailable response.
func (r *Response) ServiceUnavailable(opts ...ResponseOption) {
	r.respondWithStatus(http.StatusServiceUnavailable, "service unavailable", opts...)
}

// NotImplemented sends a 501 Not Implemented response.
func (r *Response) NotImplemented(opts ...ResponseOption) {
	r.respondWithStatus(http.StatusNotImplemented, "not implemented", opts...)
}

// RFC 9457 Problem Details (https://www.rfc-editor.org/rfc/rfc9457)
//
// Problem is additive: it exists alongside BadRequest/Forbidden/NotFound/etc.
// rather than changing what they emit, so adopting it is opt-in per call
// site and per repo.

// Problem is an RFC 9457 Problem Details object. Every field is optional per
// the RFC; Type defaults to "about:blank" and Status is filled in from the
// status code passed to (*Response).Problem when left zero, so the common
// case only needs Title (and Detail, for an occurrence-specific message).
type Problem struct {
	// Type is a URI reference identifying the problem type. Defaults to
	// "about:blank" (RFC 9457 section 3.1) when empty. Consumers MUST use
	// Type, not Status, as the problem's primary identifier.
	Type string
	// Title is a short, human-readable summary. It should stay stable
	// across occurrences of this problem type, except for localization.
	Title string
	// Status is the HTTP status code, advisory only -- the response's
	// actual status line is authoritative. Left zero, it is set to the
	// status code passed to Problem.
	Status int
	// Detail is a human-readable explanation specific to this occurrence.
	// It should help the client correct the problem, not aid debugging.
	Detail string
	// Instance is a URI reference identifying this specific occurrence.
	Instance string
	// Extensions carries problem-type-specific members (e.g. a
	// machine-readable "reason" code) as additional top-level JSON members
	// alongside type/title/status/detail/instance (RFC 9457 section 3.2). A
	// key colliding with one of those five names is dropped in favour of
	// the standard member.
	Extensions map[string]any
}

// Problem sends an RFC 9457 "application/problem+json" response. status
// sets the HTTP status line; p.Status is set to status when p.Status is
// zero, since RFC 9457 requires the two to match.
//
// opts accepts LogErr, LogMessage and Header/Location for side effects,
// exactly as the other Response methods do. Data and PublicMessage do not
// apply to a Problem body -- there is no message-merging step to plug them
// into -- and are dropped with a logged warning if passed.
func (r *Response) Problem(status int, p Problem, opts ...ResponseOption) {
	cfg := &responseConfig{headers: make(map[string]string)}
	for _, opt := range opts {
		opt.apply(cfg)
	}

	r.logProblemConfig(cfg, p.Title)

	for k, v := range cfg.headers {
		r.Writer.Header().Set(k, v)
	}

	body := buildProblemBody(status, p)

	r.Writer.Header().Set("Content-Type", "application/problem+json; charset=utf-8")
	r.Writer.WriteHeader(status)
	if err := json.NewEncoder(r.Writer).Encode(body); err != nil {
		r.logError(err, "error encoding response")
	}
}

// logProblemConfig logs the side effects of options that don't apply to a
// Problem response: Data/PublicMessage are warned about and dropped, and
// LogErr is logged against defaultLogMessage when LogMessage is unset.
func (r *Response) logProblemConfig(cfg *responseConfig, defaultLogMessage string) {
	if cfg.data != nil || cfg.publicMessage != "" {
		r.logWarn("Data()/PublicMessage() do not apply to Problem() and were dropped")
	}

	if cfg.logErr == nil {
		return
	}

	msg := cfg.logMessage
	if msg == "" {
		msg = defaultLogMessage
	}
	r.logError(cfg.logErr, msg)
}

// buildProblemBody fills in p's RFC 9457 defaults (Type, Status) and renders
// it as a map ready for JSON encoding, with Extensions merged underneath the
// standard members.
func buildProblemBody(status int, p Problem) map[string]any {
	if p.Type == "" {
		p.Type = "about:blank"
	}
	if p.Status == 0 {
		p.Status = status
	}

	body := make(map[string]any, len(p.Extensions)+5)
	maps.Copy(body, p.Extensions)
	body["type"] = p.Type
	body["status"] = p.Status
	if p.Title != "" {
		body["title"] = p.Title
	}
	if p.Detail != "" {
		body["detail"] = p.Detail
	}
	if p.Instance != "" {
		body["instance"] = p.Instance
	}

	return body
}

// Semantic success response functions

// OK sends a 200 OK response.
func (r *Response) OK(opts ...ResponseOption) {
	r.respondWithStatus(http.StatusOK, "", opts...)
}

// Created sends a 201 Created response.
func (r *Response) Created(opts ...ResponseOption) {
	r.respondWithStatus(http.StatusCreated, "created", opts...)
}

// Accepted sends a 202 Accepted response.
func (r *Response) Accepted(opts ...ResponseOption) {
	r.respondWithStatus(http.StatusAccepted, "accepted", opts...)
}

// WithStatus sends a response with a custom status code.
func (r *Response) WithStatus(code int, opts ...ResponseOption) {
	r.respondWithStatus(code, "", opts...)
}

// NoContent sends a 204 No Content response.
func (r *Response) NoContent(opts ...ResponseOption) {
	r.respondWithStatus(http.StatusNoContent, "", opts...)
}

// BadRequestWithMessage sends a 400 Bad Request response with message as
// the public message.
//
// Deprecated: Use BadRequest() with options instead.
func (r *Response) BadRequestWithMessage(message string) {
	r.BadRequest(PublicMessage(message))
}

// BadRequestWithMessages sends a 400 Bad Request response with
// responseMessage as the public message and logMessage logged separately.
//
// Deprecated: Use BadRequest() with options instead.
func (r *Response) BadRequestWithMessages(responseMessage, logMessage string) {
	r.BadRequest(LogMessage(logMessage), PublicMessage(responseMessage))
}

// InvalidInputWithMessage sends a 400 Bad Request response with message as
// the public message and err logged.
//
// Deprecated: Use InvalidInput() with options instead.
func (r *Response) InvalidInputWithMessage(err error, message string) {
	r.InvalidInput(PublicMessage(message), LogErr(err))
}

// InvalidInputWithMessages sends a 400 Bad Request response with
// responseMessage as the public message, and err and logMessage logged.
//
// Deprecated: Use InvalidInput() with options instead.
func (r *Response) InvalidInputWithMessages(err error, responseMessage, logMessage string) {
	r.InvalidInput(PublicMessage(responseMessage), LogErr(err), LogMessage(logMessage))
}

// InternalServerErrorWithMessage sends a 500 Internal Server Error response
// with message as the public message and err logged.
//
// Deprecated: Use InternalServerError() with options instead.
func (r *Response) InternalServerErrorWithMessage(err error, message string) {
	r.InternalServerError(LogErr(err), PublicMessage(message))
}

// InternalServerErrorWithMessages sends a 500 Internal Server Error
// response with responseMessage as the public message, and err and
// logMessage logged.
//
// Deprecated: Use InternalServerError() with options instead.
func (r *Response) InternalServerErrorWithMessages(err error, responseMessage string, logMessage string) {
	r.InternalServerError(PublicMessage(responseMessage), LogErr(err), LogMessage(logMessage))
}

// ForbiddenWithMessages sends a 403 Forbidden response with
// responseMessage as the public message and logMessage logged separately.
//
// Deprecated: Use Forbidden() with options instead.
func (r *Response) ForbiddenWithMessages(responseMessage, logMessage string) {
	r.Forbidden(PublicMessage(responseMessage), LogMessage(logMessage))
}

// ForbiddenWithMessage sends a 403 Forbidden response with message as both
// the public message and the logged message.
//
// Deprecated: Use Forbidden() with options instead.
func (r *Response) ForbiddenWithMessage(message string) {
	r.Forbidden(PublicMessage(message), LogMessage(message))
}

// UnauthorizedWithMessages sends a 401 Unauthorized response with
// responseMessage as the public message and logMessage logged separately.
//
// Deprecated: Use Unauthorized() with options instead.
func (r *Response) UnauthorizedWithMessages(responseMessage, logMessage string) {
	r.Unauthorized(PublicMessage(responseMessage), LogMessage(logMessage))
}

// UnauthorizedWithMessage sends a 401 Unauthorized response with message
// as both the public message and the logged message.
//
// Deprecated: Use Unauthorized() with options instead.
func (r *Response) UnauthorizedWithMessage(message string) {
	r.Unauthorized(PublicMessage(message), LogMessage(message))
}

// ConflictWithMessage sends a 409 Conflict response with message as the
// public message.
//
// Deprecated: Use Conflict() with options instead.
func (r *Response) ConflictWithMessage(message string) {
	r.Conflict(PublicMessage(message))
}

// ConflictWithMessages sends a 409 Conflict response with responseMessage
// as the public message and logMessage logged separately.
//
// Deprecated: Use Conflict() with options instead.
func (r *Response) ConflictWithMessages(responseMessage, logMessage string) {
	r.Conflict(PublicMessage(responseMessage), LogMessage(logMessage))
}

func (r *Response) logError(err error, message string) {
	if r.logger != nil {
		r.logger.Error().Err(err).Msg(message)
	}
}

func (r *Response) logWarn(message string) {
	if r.logger != nil {
		r.logger.Warn().Msg(message)
	}
}

// NotFoundWithMessage sends a 404 Not Found response with message as the
// public message.
//
// Deprecated: Use NotFound() with options instead.
func (r *Response) NotFoundWithMessage(message string) {
	r.NotFound(PublicMessage(message))
}

// NotFoundWithMessages sends a 404 Not Found response with responseMessage
// as the public message and logMessage logged separately.
//
// Deprecated: Use NotFound() with options instead.
func (r *Response) NotFoundWithMessages(responseMessage, logMessage string) {
	r.NotFound(PublicMessage(responseMessage), LogMessage(logMessage))
}

// NotAcceptableWithMessage sends a 406 Not Acceptable response with
// message as the public message.
//
// Deprecated: Use NotAcceptable() with options instead.
func (r *Response) NotAcceptableWithMessage(message string) {
	r.NotAcceptable(PublicMessage(message))
}

// NotAcceptableWithMessages sends a 406 Not Acceptable response with
// responseMessage as the public message and logMessage logged separately.
//
// Deprecated: Use NotAcceptable() with options instead.
func (r *Response) NotAcceptableWithMessages(responseMessage, logMessage string) {
	r.NotAcceptable(PublicMessage(responseMessage), LogMessage(logMessage))
}

// CreatedWithMessage sends a 201 Created response with message as the
// public message.
//
// Deprecated: Use Created() with options instead.
func (r *Response) CreatedWithMessage(message string) {
	r.Created(PublicMessage(message))
}

// CreatedWithURI sends a 201 Created response with a Location header and
// public message set to uri, and a body containing uri.
//
// Deprecated: Use Created() with Location() option instead.
func (r *Response) CreatedWithURI(uri string) {
	r.Created(Location(uri), PublicMessage(uri), Data(map[string]any{
		"uri": uri,
	}))
}

// CreatedWithURIAndMessage sends a 201 Created response with a Location
// header set to uri, a body containing uri, and message as the public
// message.
//
// Deprecated: Use Created() with Location() and PublicMessage() options instead.
func (r *Response) CreatedWithURIAndMessage(uri string, message string) {
	r.Created(Location(uri), Data(map[string]any{"uri": uri}), PublicMessage(message))
}

// AcceptedWithMessage sends a 202 Accepted response with message as the
// public message.
//
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
