// Package json provides utilities for handling JSON requests and responses.
package json

import (
	"encoding/json"
	"net/http"

	"github.com/rs/zerolog"
)

// Response simplifies sending structured JSON responses and logging errors.
type Response struct {
	Writer http.ResponseWriter
	// Request http.Request
	logger *zerolog.Logger
}

// NewResponse creates a new Response helper with the provided ResponseWriter.
func NewResponse(w http.ResponseWriter) *Response {
	return &Response{
		Writer: w,
	}
}

// NewResponseWithLogger creates a new Response helper with a logger that includes request metadata.
func NewResponseWithLogger(w http.ResponseWriter, r *http.Request, l zerolog.Logger) *Response {
	logger := l.With().
		Str("method", r.Method).
		Str("url", r.URL.Redacted()).
		Str("remoteAddr", r.RemoteAddr).
		Str("userAgent", r.UserAgent()).
		Logger()
	return &Response{
		Writer: w,
		// Request: r,
		logger: &logger,
	}
}

// BadRequestWithMessage sends a 400 Bad Request response with a JSON error message.
func (r *Response) BadRequestWithMessage(message string) {
	r.BadRequestWithMessages(message, message)
}

// BadRequestWithMessages sends a 400 Bad Request response and logs a separate message.
func (r *Response) BadRequestWithMessages(responseMessage, logMessage string) {
	r.ErrorWithMessages(http.StatusBadRequest, responseMessage, logMessage, nil)
}

// InvalidInputWithMessage sends a 400 Bad Request response for invalid input.
func (r *Response) InvalidInputWithMessage(err error, message string) {
	r.InvalidInputWithMessages(err, message, message)
}

// InvalidInputWithMessages sends a 400 Bad Request response for invalid input and logs the error.
func (r *Response) InvalidInputWithMessages(err error, responseMessage, logMessage string) {
	r.ErrorWithMessages(http.StatusBadRequest, responseMessage, logMessage, err)
}

// InternalServerErrorWithMessage sends a 500 Internal Server Error response.
func (r *Response) InternalServerErrorWithMessage(err error, message string) {
	r.InternalServerErrorWithMessages(err, message, message)
}

// InternalServerErrorWithMessages sends a 500 Internal Server Error response and logs the error.
func (r *Response) InternalServerErrorWithMessages(err error, responseMessage string, logMessage string) {
	r.ErrorWithMessages(http.StatusInternalServerError, responseMessage, logMessage, err)
}

// ForbiddenWithMessages sends a 403 Forbidden response.
func (r *Response) ForbiddenWithMessages(responseMessage, logMessage string) {
	r.ErrorWithMessages(http.StatusForbidden, responseMessage, logMessage, nil)
}

// ForbiddenWithMessage sends a 403 Forbidden response with the same message for response and log.
func (r *Response) ForbiddenWithMessage(message string) {
	r.ForbiddenWithMessages(message, message)
}

// UnauthorizedWithMessages sends a 401 Unauthorized response.
func (r *Response) UnauthorizedWithMessages(responseMessage, logMessage string) {
	r.ErrorWithMessages(http.StatusUnauthorized, responseMessage, logMessage, nil)
}

// UnauthorizedWithMessage sends a 401 Unauthorized response with the same message for response and log.
func (r *Response) UnauthorizedWithMessage(message string) {
	r.UnauthorizedWithMessages(message, message)
}

// ConflictWithMessage sends a 409 Conflict response.
func (r *Response) ConflictWithMessage(message string) {
	r.ConflictWithMessages(message, message)
}

// ConflictWithMessages sends a 409 Conflict response and logs a separate message.
func (r *Response) ConflictWithMessages(responseMessage, logMessage string) {
	r.ErrorWithMessages(http.StatusConflict, responseMessage, logMessage, nil)
}

// ErrorWithMessages sends an error response with the specified status code and messages.
func (r *Response) ErrorWithMessages(code int, responseMessage string, logMessage string, err error) {
	data := map[string]string{"error": responseMessage}
	r.logError(err, logMessage)
	r.Data(code, data)
}

func (r *Response) logError(err error, message string) {
	if r.logger != nil {
		r.logger.Error().Err(err).Msg(message)
	}
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

// OK sends a 200 OK response with the provided data.
func (r *Response) OK(data any) {
	r.Data(http.StatusOK, data)
}

// NotFoundWithMessage sends a 404 Not Found response.
func (r *Response) NotFoundWithMessage(message string) {
	r.NotFoundWithMessages(message, message)
}

// NotFoundWithMessages sends a 404 Not Found response and logs a separate message.
func (r *Response) NotFoundWithMessages(responseMessage, logMessage string) {
	r.ErrorWithMessages(http.StatusNotFound, responseMessage, logMessage, nil)
}

// NotAcceptableWithMessage sends a 406 Not Acceptable response.
func (r *Response) NotAcceptableWithMessage(message string) {
	r.NotAcceptableWithMessages(message, message)
}

// NotAcceptableWithMessages sends a 406 Not Acceptable response and logs a separate message.
func (r *Response) NotAcceptableWithMessages(responseMessage, logMessage string) {
	r.ErrorWithMessages(http.StatusNotAcceptable, responseMessage, logMessage, nil)
}

// CreatedWithMessage sends a 201 Created response.
func (r *Response) CreatedWithMessage(message string) {
	r.Data(http.StatusCreated, map[string]string{"message": message})
}

// CreatedWithURI sends a 201 Created response with a Location header pointing to the newly created resource.
// The uri parameter should be the full path/URL to the newly created resource.
// This follows REST best practices by including the Location header and resource URI in the response.
func (r *Response) CreatedWithURI(uri string) {
	r.Writer.Header().Set("Location", uri)
	r.Data(http.StatusCreated, map[string]string{"uri": uri})
}

// CreatedWithURIAndMessage sends a 201 Created response with a Location header and custom message.
func (r *Response) CreatedWithURIAndMessage(uri string, message string) {
	r.Writer.Header().Set("Location", uri)
	r.Data(http.StatusCreated, map[string]interface{}{"uri": uri, "message": message})
}

// NoContent sends a 204 No Content response.
func (r *Response) NoContent() {
	r.Writer.WriteHeader(http.StatusNoContent)
}

// AcceptedWithMessage sends a 202 Accepted response.
func (r *Response) AcceptedWithMessage(s string) {
	r.Data(http.StatusAccepted, map[string]string{"message": s})
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
