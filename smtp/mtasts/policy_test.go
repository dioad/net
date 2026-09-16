package mtasts

import (
	"testing"
)

func TestSimplePolicy(t *testing.T) {
	p := Policy{
		Version: "STSv1",
		Mode:    ModeTesting,
		MX:      []string{"mx.example.com"},
		MaxAge:  3600,
	}

	expected := "version: STSv1\nmode: testing\nmx: mx.example.com\nmax_age: 3600\n"

	result, err := FormatPolicy(&p)
	if err != nil {
		t.Fatalf("FormatPolicy() error = %v, want nil", err)
	}

	if expected != result {
		t.Errorf("got: %s, expected: %s", result, expected)
	}
}

func TestFormatPolicy_RequiresMXForTestingAndEnforceModes(t *testing.T) {
	for _, mode := range []Mode{ModeTesting, ModeEnforce} {
		t.Run(string(mode), func(t *testing.T) {
			p := Policy{Version: "STSv1", Mode: mode, MaxAge: 3600}

			_, err := FormatPolicy(&p)

			if err == nil {
				t.Errorf("FormatPolicy() with empty MX and mode %q should error per RFC 8461 3.2 (mx MUST appear for testing/enforce), got nil", mode)
			}
		})
	}
}

func TestFormatPolicy_AllowsEmptyMXForModeNone(t *testing.T) {
	p := Policy{Version: "STSv1", Mode: ModeNone, MaxAge: 3600}

	_, err := FormatPolicy(&p)

	if err != nil {
		t.Errorf("FormatPolicy() with mode none and empty MX should not error, got %v", err)
	}
}
