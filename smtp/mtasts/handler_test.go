package mtasts

import (
	"net/http"
	"net/http/httptest"
	"strings"
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

	request, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, "/mtsts", nil)
	response := httptest.NewRecorder()

	handler(response, request)

	if response.Code != http.StatusOK {
		t.Errorf("expected status code %d, got %d", http.StatusOK, response.Code)
	}
	if response.Body.String() != expected {
		t.Errorf("expected body %s, got %s", expected, response.Body.String())
	}
}

func TestHTTPHandler_ReflectsLaterPolicyMutations(t *testing.T) {
	p := PolicyFromConfig(Config{
		Mode:   ModeTesting,
		MX:     []string{"mx1.example.com"},
		MaxAge: 3600,
	})

	handler, err := HTTPHandler(p)
	if err != nil {
		t.Fatalf("failed to create handler: %s", err)
	}

	// A caller reasonably expects the running endpoint to reflect updates
	// to the *Policy it constructed the handler with (e.g. rotating MX
	// hosts) -- HTTPHandler takes a pointer, not a value.
	p.MX = []string{"mx2.example.com"}

	request, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, "/mtsts", nil)
	response := httptest.NewRecorder()
	handler(response, request)

	if !strings.Contains(response.Body.String(), "mx2.example.com") {
		t.Errorf("handler did not reflect the policy mutation, body = %q", response.Body.String())
	}
}
