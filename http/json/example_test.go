package json_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"

	"github.com/rs/zerolog"

	"github.com/dioad/net/http/json"
)

// A handler answers with a helper named after the status. The options say
// what goes to the client.
func ExampleResponse_Created() {
	w := httptest.NewRecorder()

	json.NewResponse(w).Created(
		json.Location("/things/42"),
		json.Data(map[string]any{"id": 42}),
	)

	fmt.Println(w.Code, w.Header().Get("Location"))
	fmt.Print(w.Body.String())
	// Output:
	// 201 /things/42
	// {"id":42,"message":"created"}
}

// LogErr keeps the cause of a failure in the server's log, and PublicMessage
// says what the client is told instead.
func ExampleResponse_InternalServerError() {
	logger := zerolog.New(os.Stdout)
	r := httptest.NewRequestWithContext(logger.WithContext(context.Background()), http.MethodGet, "/things/42", nil)
	w := httptest.NewRecorder()

	json.NewResponseFromRequest(w, r).InternalServerError(
		json.LogErr(errors.New("connection refused")),
		json.LogMessage("failed to load thing"),
		json.PublicMessage("try again later"),
	)

	fmt.Println(w.Code)
	fmt.Print(w.Body.String())
	// Output:
	// {"level":"error","error":"connection refused","message":"failed to load thing"}
	// 500
	// {"error":"try again later"}
}

// LogField adds a field to the request's logger, so that the access log line
// the server writes after the handler returns carries it.
func ExampleLogField() {
	logger := zerolog.New(os.Stdout)
	r := httptest.NewRequestWithContext(logger.WithContext(context.Background()), http.MethodPost, "/events", nil)
	w := httptest.NewRecorder()

	json.NewResponseFromRequest(w, r).Accepted(json.LogField("event_type", "agent.registered"))

	// The server's request log does this once the handler has returned.
	zerolog.Ctx(r.Context()).Info().Msg("accessLog")
	fmt.Print(w.Body.String())
	// Output:
	// {"level":"info","event_type":"agent.registered","message":"accessLog"}
	// {"message":"accepted"}
}
