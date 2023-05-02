package dns

import (
	"net/url"
	"strings"
	"testing"

	"github.com/miekg/dns"
)

func TestDOHClient(t *testing.T) {
	dohTestHelper(t, dns.TypeA, "dns.google.", "8.8.8.8", "8.8.4.4")
}

func dohTestHelper(t *testing.T, queryType uint16, queryHost string, values ...string) {
	t.Helper()

	u, err := url.Parse("https://dns.google")
	if err != nil {
		t.Fatalf("error parsing url: %s", err)
	}

	c := NewDOHClient(u)

	msgQuestion := dns.Msg{}
	msgQuestion.SetQuestion(queryHost, queryType)

	msgAnswer, err := c.Exchange(&msgQuestion)
	if err != nil {
		t.Fatalf("error exchanging message: %s", err)
	}

	if !containsRecords(t, msgAnswer.Answer, queryType, values...) {
		t.Errorf("expected answer to contain addr record for %s", strings.Join(values, ","))
	}
}

func containsRecords(t *testing.T, records []dns.RR, responseType uint16, values ...string) bool {
	t.Helper()

	for _, record := range records {
		if record.Header().Rrtype == responseType {
			if rrContainsRecords(t, record, responseType, values...) {
				return true
			}
		}
	}

	return false
}

func rrContainsRecords(t *testing.T, record dns.RR, responseType uint16, values ...string) bool {
	t.Helper()

	for _, value := range values {
		if rrContainsRecord(t, record, responseType, value) {
			return true
		}
	}

	return false
}

func rrContainsRecord(t *testing.T, record dns.RR, responseType uint16, value string) bool {
	t.Helper()

	switch responseType {
	case dns.TypeA:
		a, ok := record.(*dns.A)
		if !ok {
			return false
		}

		return a.A.String() == value
	case dns.TypeCNAME:
		cname, ok := record.(*dns.CNAME)
		if !ok {
			return false
		}

		return cname.Target == value
	}

	return false
}
