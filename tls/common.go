package tls

import "crypto/tls"

func convertTLSVersion(version string) uint16 {
	switch version {
	case "TLS13":
		return tls.VersionTLS13
	case "TLS12":
		return tls.VersionTLS12
	case "TLS11":
		return tls.VersionTLS11
	case "TLS10":
		return tls.VersionTLS10
	default:
		return tls.VersionTLS12
	}
}

func convertClientAuthType(authType string) tls.ClientAuthType {
	switch authType {
	case "RequestClientCert":
		return tls.RequestClientCert
	case "RequireAnyClientCert":
		return tls.RequireAnyClientCert
	case "VerifyClientCertIfGiven":
		return tls.VerifyClientCertIfGiven
	case "RequireAndVerifyClientCert":
		return tls.RequireAndVerifyClientCert
	default:
		return tls.NoClientCert
	}
}
