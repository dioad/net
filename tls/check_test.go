package tls_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dioad/config/configcheck"
	dnt "github.com/dioad/net/tls"
)

func findingsOf(check func(*configcheck.Collector)) []configcheck.Finding {
	c := configcheck.NewCollector()
	check(c.Field("x"))

	return c.Findings()
}

func TestServerConfig_Check(t *testing.T) {
	t.Parallel()

	const noEffect = "has no effect: client-auth-type is not set, so client certificates are never requested or checked; " +
		"set client-auth-type, for example RequireAndVerifyClientCert, or remove client-ca-file"
	tests := []struct {
		name string
		cfg  *dnt.ServerConfig
		want []configcheck.Finding
	}{
		{"unset", nil, nil},
		{"acme only", &dnt.ServerConfig{ACME: dnt.ACMEConfig{CacheDirectory: ".autocert"}}, nil},
		{"local cert and key", &dnt.ServerConfig{LocalConfig: dnt.LocalConfig{Certificate: "c.pem", Key: "k.pem"}}, nil},
		{"local single pem file", &dnt.ServerConfig{LocalConfig: dnt.LocalConfig{SinglePEMFile: "all.pem"}}, nil},
		{"acme and local", &dnt.ServerConfig{
			ACME:        dnt.ACMEConfig{CacheDirectory: ".autocert"},
			LocalConfig: dnt.LocalConfig{Certificate: "c.pem", Key: "k.pem"},
		}, []configcheck.Finding{{Path: "x", Message: "sets [acme local], but only one of acme, self-signed and local is used"}}},
		{"local cert without key", &dnt.ServerConfig{LocalConfig: dnt.LocalConfig{Certificate: "c.pem"}},
			[]configcheck.Finding{{Path: "x.local", Message: "needs both cert and key, or a single-pem-file"}}},
		{"client ca without a type enforces nothing", &dnt.ServerConfig{ClientCAFile: "ca.pem"},
			[]configcheck.Finding{{Path: "x.client-ca-file", Message: noEffect}}},
		{"client ca with NoClientCert enforces nothing", &dnt.ServerConfig{ClientCAFile: "ca.pem", ClientAuthType: "NoClientCert"},
			[]configcheck.Finding{{Path: "x.client-ca-file", Message: noEffect}}},
		{"client ca with a type", &dnt.ServerConfig{ClientCAFile: "ca.pem", ClientAuthType: "RequireAndVerifyClientCert"}, nil},
		{"client type without a ca", &dnt.ServerConfig{ClientAuthType: "RequestClientCert"}, nil},
		{"unknown client type", &dnt.ServerConfig{ClientAuthType: "RequireClientCert"},
			[]configcheck.Finding{{Path: "x.client-auth-type", Message: `"RequireClientCert" must be one of NoClientCert, RequestClientCert, RequireAnyClientCert, VerifyClientCertIfGiven, RequireAndVerifyClientCert`}}},
		{"local single pem file and key", &dnt.ServerConfig{LocalConfig: dnt.LocalConfig{SinglePEMFile: "all.pem", Key: "k.pem"}},
			[]configcheck.Finding{{Path: "x.local", Message: "sets single-pem-file and cert or key; set either single-pem-file, or cert and key"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, findingsOf(tt.cfg.Check))
		})
	}
}

func TestClientConfig_Check(t *testing.T) {
	t.Parallel()

	var unset *dnt.ClientConfig
	assert.Empty(t, findingsOf(unset.Check))
	assert.Empty(t, findingsOf((&dnt.ClientConfig{InsecureSkipVerify: true}).Check))
	assert.Equal(t, []configcheck.Finding{{Path: "x", Message: "needs both cert and key for a client certificate"}},
		findingsOf((&dnt.ClientConfig{Key: "k.pem"}).Check))
}

func ExampleServerConfig_Check() {
	cfg := dnt.ServerConfig{
		LocalConfig:  dnt.LocalConfig{Certificate: "server.pem"},
		ClientCAFile: "clients.pem",
	}

	findings := configcheck.NewCollector()
	cfg.Check(findings.Field("tls-server"))

	for _, f := range findings.Findings() {
		fmt.Println(f.Path)
	}
	// Output:
	// tls-server.local
	// tls-server.client-ca-file
}

func ExampleClientConfig_Check() {
	cfg := dnt.ClientConfig{Certificate: "client.pem"}

	findings := configcheck.NewCollector()
	cfg.Check(findings.Field("tls-client"))

	fmt.Println(findings.Findings()[0])
	// Output: tls-client: needs both cert and key for a client certificate
}
