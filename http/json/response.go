package json

import (
	"encoding/json"
	"net/http"

	"github.com/rs/zerolog"
)

type Response struct {
	Writer http.ResponseWriter
	// Request http.Request
	logger *zerolog.Logger
}

func NewResponse(w http.ResponseWriter) *Response {
	return &Response{
		Writer: w,
	}
}

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

func (r *Response) BadRequestWithMessage(message string) {
	r.BadRequestWithMessages(message, message)
}

func (r *Response) BadRequestWithMessages(responseMessage, logMessage string) {
	r.ErrorWithMessages(http.StatusBadRequest, responseMessage, logMessage, nil)
}

func (r *Response) InvalidInputWithMessage(err error, message string) {
	r.InvalidInputWithMessages(err, message, message)
}

func (r *Response) InvalidInputWithMessages(err error, responseMessage, logMessage string) {
	r.ErrorWithMessages(http.StatusBadRequest, responseMessage, logMessage, err)
}

func (r *Response) InternalServerErrorWithMessage(err error, message string) {
	r.InternalServerErrorWithMessages(err, message, message)
}

func (r *Response) InternalServerErrorWithMessages(err error, responseMessage string, logMessage string) {
	r.ErrorWithMessages(http.StatusInternalServerError, responseMessage, logMessage, err)
}

func (r *Response) ForbiddenWithMessages(responseMessage, logMessage string) {
	r.ErrorWithMessages(http.StatusForbidden, responseMessage, logMessage, nil)
}

func (r *Response) ForbiddenWithMessage(message string) {
	r.UnauthorizedWithMessages(message, message)
}

func (r *Response) UnauthorizedWithMessages(responseMessage, logMessage string) {
	r.ErrorWithMessages(http.StatusUnauthorized, responseMessage, logMessage, nil)
}

func (r *Response) UnauthorizedWithMessage(message string) {
	r.UnauthorizedWithMessages(message, message)
}

func (r *Response) ConflictWithMessage(message string) {
	r.ConflictWithMessages(message, message)
}

func (r *Response) ConflictWithMessages(responseMessage, logMessage string) {
	r.ErrorWithMessages(http.StatusConflict, responseMessage, logMessage, nil)
}

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
	return
}

func (r *Response) OK(data any) {
	r.Data(http.StatusOK, data)
}

func (r *Response) NotFoundWithMessage(message string) {
	r.NotFoundWithMessages(message, message)
}

func (r *Response) NotFoundWithMessages(responseMessage, logMessage string) {
	r.ErrorWithMessages(http.StatusNotFound, responseMessage, logMessage, nil)
}

func (r *Response) NotAcceptableWithMessage(message string) {
	r.NotAcceptableWithMessages(message, message)
}

func (r *Response) NotAcceptableWithMessages(responseMessage, logMessage string) {
	r.ErrorWithMessages(http.StatusNotAcceptable, responseMessage, logMessage, nil)
}

// TODO: Need to modify this to include the uri for created resource
func (r *Response) CreatedWithMessage(message string) {
	r.Data(http.StatusCreated, map[string]string{"message": message})
}

func (r *Response) NoContent() {
	r.Writer.WriteHeader(http.StatusNoContent)
}

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
