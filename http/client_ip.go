package http

import (
	"context"
	"net/http"
	"strings"
)

// httpContextKeyClientIP is an unexported type used as a key for storing the client IP in the context.
type httpContextKeyClientIP struct{}

// ContextWithClientIP extracts the client IP address from the request and stores it in the context.
// It checks X-Forwarded-For and X-Real-IP headers first (for proxied requests),
// then falls back to RemoteAddr.
func ContextWithClientIP(ctx context.Context, r *http.Request) context.Context {
	ip := GetClientIP(r)

	return context.WithValue(ctx, httpContextKeyClientIP{}, ip)
}

// ClientIPFromContext retrieves the client IP address from the context.
func ClientIPFromContext(ctx context.Context) (string, bool) {
	ip, ok := ctx.Value(httpContextKeyClientIP{}).(string)
	return ip, ok
}

// GetClientIP extracts the client IP address from a request.
// It checks X-Forwarded-For and X-Real-IP headers first (for proxied requests),
// then falls back to RemoteAddr.
func GetClientIP(r *http.Request) string {
	// Check X-Forwarded-For header (may contain multiple IPs)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP in the list (original client)
		if idx := strings.Index(xff, ","); idx != -1 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}

	// Fall back to RemoteAddr (remove port if present)
	addr := r.RemoteAddr
	if idx := strings.LastIndex(addr, ":"); idx != -1 {
		// Handle IPv6 addresses like [::1]:8080
		if strings.HasPrefix(addr, "[") {
			if bracketIdx := strings.Index(addr, "]"); bracketIdx != -1 {
				return addr[1:bracketIdx]
			}
		}
		return addr[:idx]
	}
	return addr
}
