package mtasts

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestHandleMTASTS tests the HandleMTASTS function.
func TestHandleMTASTS(t *testing.T) {
	p := PolicyFromConfig(Config{
		Mode:   ModeTesting,
		MX:     []string{"mx.example.com"},
		MaxAge: 3600,
	})

	handler, err := HTTPHandler(p)
	if err != nil {
		t.Errorf("failed to create handler: %s", err)
	}

	expected := "version: STSv1\nmode: testing\nmx: mx.example.com\nmax_age: 3600\n"

	request, _ := http.NewRequest(http.MethodGet, "/mtsts", nil)
	response := httptest.NewRecorder()

	handler(response, request)

	if response.Code != http.StatusOK {
		t.Errorf("expected status code %d, got %d", http.StatusOK, response.Code)
	}
	if response.Body.String() != expected {
		t.Errorf("expected body %s, got %s", expected, response.Body.String())
	}
}
