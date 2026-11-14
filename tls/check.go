package tls

import (
	"slices"
	"strings"

	"github.com/dioad/config/configcheck"
	"github.com/dioad/generics"
)

// clientAuthTypes are the client-auth-type values this package understands.
// Anything else is silently treated as NoClientCert.
var clientAuthTypes = []string{
	"NoClientCert",
	"RequestClientCert",
	"RequireAnyClientCert",
	"VerifyClientCertIfGiven",
	"RequireAndVerifyClientCert",
}

// Check reports the problems in a tls-server block to findings, which should be
// positioned at the block. It mirrors what this package does with the block, so
// a config Check passes is one NewServerTLSConfig accepts and means what it says:
//
//   - only the first of acme, self-signed and local that is set is used, in that
//     order, so setting more than one leaves the others silently unused;
//   - a local block needs a single-pem-file, or both a cert and a key, and not
//     both forms;
//   - client-ca-file is loaded whatever client-auth-type says, but client
//     certificates are checked only when the type asks for it, so a CA file with
//     no type reads as mutual TLS and enforces nothing;
//   - an unrecognised client-auth-type is treated as NoClientCert.
//
// A nil block is not set at this level and is not checked.
func (c *ServerConfig) Check(findings *configcheck.Collector) {
	if c == nil {
		return
	}

	var set []string
	if !generics.IsZeroValue(c.ACME) {
		set = append(set, "acme")
	}
	if !generics.IsZeroValue(c.SelfSigned) {
		set = append(set, "self-signed")
	}
	if !generics.IsZeroValue(c.LocalConfig) {
		set = append(set, "local")
	}
	if len(set) > 1 {
		findings.Errorf("sets %v, but only one of acme, self-signed and local is used", set)
	}

	c.LocalConfig.check(findings.Field("local"))
	c.checkClientAuth(findings)
}

func (c *ServerConfig) checkClientAuth(findings *configcheck.Collector) {
	authType := c.ClientAuthType
	if authType != "" && !slices.Contains(clientAuthTypes, authType) {
		findings.Field("client-auth-type").Errorf("%q must be one of %s", authType, strings.Join(clientAuthTypes, ", "))

		return
	}
	if c.ClientCAFile != "" && (authType == "" || authType == "NoClientCert") {
		findings.Field("client-ca-file").Errorf("has no effect: client-auth-type is not set, so client certificates are never requested or checked; " +
			"set client-auth-type, for example RequireAndVerifyClientCert, or remove client-ca-file")
	}
}

func (c LocalConfig) check(findings *configcheck.Collector) {
	if generics.IsZeroValue(c) {
		return
	}
	if c.SinglePEMFile != "" {
		if c.Certificate != "" || c.Key != "" {
			findings.Errorf("sets single-pem-file and cert or key; set either single-pem-file, or cert and key")
		}

		return
	}
	if c.Certificate == "" || c.Key == "" {
		findings.Errorf("needs both cert and key, or a single-pem-file")
	}
}

// Check reports the problems in a tls-client block to findings, which should be
// positioned at the block: a client certificate needs both its cert and its
// key. A nil block is not set at this level and is not checked.
func (c *ClientConfig) Check(findings *configcheck.Collector) {
	if c == nil {
		return
	}
	if (c.Certificate == "") != (c.Key == "") {
		findings.Errorf("needs both cert and key for a client certificate")
	}
}
