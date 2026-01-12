package observability

import (
	"net"
	"net/http"
	"strings"
)

func DeviceIDFromRequest(r *http.Request) string {
	return r.Header.Get("X-Device-Id")
}

func RequestIDFromRequest(r *http.Request) string {
	return r.Header.Get("X-Request-Id")
}

func IPFromRequest(r *http.Request) string {
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		parts := strings.Split(forwarded, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}
