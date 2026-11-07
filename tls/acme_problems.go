package tls

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime"
	"net/http"

	"github.com/rs/zerolog"
)

// acmeProblemBadNonce is the problem type a CA returns for a stale replay
// nonce. The ACME client retries these itself, so they are not failures.
const acmeProblemBadNonce = "urn:ietf:params:acme:error:badNonce"

// acmeMaxInspectedBody bounds how much of a response is buffered for
// inspection. ACME JSON documents are a few kilobytes; anything larger is
// passed through untouched.
const acmeMaxInspectedBody = 1 << 20

// acmeProblemLogger is the ACME client's http.RoundTripper. It logs why a
// CA rejected a request or a validation, which autocert never reports: it
// discards the CA's reason and, once every challenge type has failed,
// returns only its own "no viable challenge type found". The reason is in
// the problem, authorization, challenge and order documents (RFC 8555) that
// pass through here.
type acmeProblemLogger struct {
	logger zerolog.Logger
	next   http.RoundTripper
}

// newACMEProblemLogger wraps next, logging with ctx's logger. autocert calls
// the CA from contexts of its own, so the logger is taken from ctx once
// here to keep the caller's fields (a tunnel's name, say) on every event.
func newACMEProblemLogger(ctx context.Context, next http.RoundTripper) *acmeProblemLogger {
	return &acmeProblemLogger{logger: *zerolog.Ctx(ctx), next: next}
}

func (l *acmeProblemLogger) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := l.next.RoundTrip(req)
	if err != nil {
		return resp, err
	}

	mediaType, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if mediaType != "application/json" && mediaType != "application/problem+json" {
		return resp, nil
	}

	body, complete, err := peekBody(resp)
	if err != nil || !complete {
		// The caller still reads the whole body; only inspection is skipped.
		return resp, nil //nolint:nilerr // a body we cannot inspect is still the CA's response
	}

	if resp.StatusCode >= http.StatusBadRequest {
		l.logProblem(req, resp.StatusCode, body)
		return resp, nil
	}
	l.logInvalidObject(req, body)

	return resp, nil
}

// peekBody reads up to acmeMaxInspectedBody bytes of resp's body and puts
// them back in front of the rest, so the caller reads the response as sent.
// The bool reports whether that was the whole body.
func peekBody(resp *http.Response) ([]byte, bool, error) {
	body, err := io.ReadAll(io.LimitReader(resp.Body, acmeMaxInspectedBody+1))
	resp.Body = readCloser{Reader: io.MultiReader(bytes.NewReader(body), resp.Body), Closer: resp.Body}
	if err != nil {
		return nil, false, err
	}

	return body, len(body) <= acmeMaxInspectedBody, nil
}

type readCloser struct {
	io.Reader
	io.Closer
}

type acmeIdentifier struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type acmeProblem struct {
	Type        string           `json:"type"`
	Detail      string           `json:"detail"`
	Subproblems []acmeSubproblem `json:"subproblems"`
}

type acmeSubproblem struct {
	Type       string         `json:"type"`
	Detail     string         `json:"detail"`
	Identifier acmeIdentifier `json:"identifier"`
}

type acmeValidationRecord struct {
	Hostname          string   `json:"hostname"`
	AddressUsed       string   `json:"addressUsed"`       //nolint:tagliatelle // RFC 8555 field name
	AddressesResolved []string `json:"addressesResolved"` //nolint:tagliatelle // RFC 8555 field name
}

type acmeChallenge struct {
	Type             string                 `json:"type"`
	URL              string                 `json:"url"`
	Status           string                 `json:"status"`
	Error            *acmeProblem           `json:"error"`
	ValidationRecord []acmeValidationRecord `json:"validationRecord"` //nolint:tagliatelle // RFC 8555 field name
}

// acmeObject holds the fields of an authorization, a challenge and an order
// that say whether, and why, it is invalid.
type acmeObject struct {
	Status string `json:"status"`
	// Error is set on an invalid challenge or order.
	Error *acmeProblem `json:"error"`

	// Identifier and Challenges are set on an authorization.
	Identifier *acmeIdentifier `json:"identifier"`
	Challenges []acmeChallenge `json:"challenges"`

	// Identifiers is set on an order.
	Identifiers []acmeIdentifier `json:"identifiers"`

	// Type, URL and ValidationRecord are set on a challenge.
	Type             string                 `json:"type"`
	URL              string                 `json:"url"`
	ValidationRecord []acmeValidationRecord `json:"validationRecord"` //nolint:tagliatelle // RFC 8555 field name
}

func (l *acmeProblemLogger) logProblem(req *http.Request, status int, body []byte) {
	var problem acmeProblem
	if json.Unmarshal(body, &problem) != nil || problem.Type == "" {
		return
	}

	level := zerolog.WarnLevel
	if problem.Type == acmeProblemBadNonce {
		level = zerolog.DebugLevel
	}
	logger := withProblem(l.logger.With(), &problem).
		Str("acme_url", req.URL.String()).
		Int("http_status", status).
		Logger()
	logger.WithLevel(level).Msg("acme request rejected")
}

func (l *acmeProblemLogger) logInvalidObject(req *http.Request, body []byte) {
	var object acmeObject
	if json.Unmarshal(body, &object) != nil || object.Status != "invalid" {
		return
	}

	switch {
	case object.Identifier != nil:
		l.logInvalidAuthorization(req, &object)
	case object.Identifiers != nil:
		logger := withProblem(l.logger.With(), object.Error).
			Str("acme_url", req.URL.String()).
			Strs("acme_identifiers", identifierValues(object.Identifiers)).
			Logger()
		logger.Warn().Msg("acme order invalid")
	case object.Type != "":
		l.logFailedChallenge("", &acmeChallenge{
			Type:             object.Type,
			URL:              object.URL,
			Error:            object.Error,
			ValidationRecord: object.ValidationRecord,
		})
	}
}

func (l *acmeProblemLogger) logInvalidAuthorization(req *http.Request, authz *acmeObject) {
	failed := false
	for i := range authz.Challenges {
		if authz.Challenges[i].Error == nil {
			continue
		}
		failed = true
		l.logFailedChallenge(authz.Identifier.Value, &authz.Challenges[i])
	}
	if failed {
		return
	}

	// An authorization can be invalid with no challenge error, when it was
	// deactivated or expired before any challenge was answered.
	l.logger.Warn().
		Str("acme_url", req.URL.String()).
		Str("acme_identifier", authz.Identifier.Value).
		Msg("acme authorization invalid")
}

func (l *acmeProblemLogger) logFailedChallenge(identifier string, challenge *acmeChallenge) {
	hostnames := make([]string, 0, len(challenge.ValidationRecord))
	used := make([]string, 0, len(challenge.ValidationRecord))
	var resolved []string
	for _, record := range challenge.ValidationRecord {
		hostnames = append(hostnames, record.Hostname)
		used = append(used, record.AddressUsed)
		resolved = append(resolved, record.AddressesResolved...)
	}

	fields := withProblem(l.logger.With(), challenge.Error).
		Str("acme_challenge_type", challenge.Type).
		Str("acme_challenge_url", challenge.URL).
		Strs("acme_validation_hostnames", hostnames).
		Strs("acme_validation_addresses_used", used).
		Strs("acme_validation_addresses_resolved", resolved)
	if identifier != "" {
		fields = fields.Str("acme_identifier", identifier)
	}
	logger := fields.Logger()
	logger.Warn().Msg("acme challenge failed")
}

// withProblem adds a CA problem document's type, detail and subproblems to
// fields. A nil problem adds nothing.
func withProblem(fields zerolog.Context, problem *acmeProblem) zerolog.Context {
	if problem == nil {
		return fields
	}
	fields = fields.
		Str("acme_problem_type", problem.Type).
		Str("acme_problem_detail", problem.Detail)
	if len(problem.Subproblems) > 0 {
		subproblems := make([]string, 0, len(problem.Subproblems))
		for _, sub := range problem.Subproblems {
			subproblems = append(subproblems, sub.Identifier.Value+": "+sub.Type+": "+sub.Detail)
		}
		fields = fields.Strs("acme_subproblems", subproblems)
	}

	return fields
}

func identifierValues(identifiers []acmeIdentifier) []string {
	values := make([]string, 0, len(identifiers))
	for _, id := range identifiers {
		values = append(values, id.Value)
	}

	return values
}
