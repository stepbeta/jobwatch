package api

import (
	"mime"
	"net"
	"net/http"
	"net/url"
	"strings"
)

// requestPolicy wraps the API mux with the local-request rules: the Host
// must name a loopback address or the address the server was bound to, and
// a state-changing request must not carry a foreign Origin. It runs before
// routing, so a rejected request never reaches a handler.
func (s *Server) requestPolicy(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.allowedRequestHost(r.Host) {
			writeError(w, http.StatusForbidden, "forbidden_host",
				"host is not the local JobWatch server")
			return
		}
		if isStateChanging(r.Method) {
			if origin := strings.TrimSpace(r.Header.Get("Origin")); origin != "" && !s.allowedOrigin(origin) {
				writeError(w, http.StatusForbidden, "forbidden_origin",
					"cross-origin writes to the local API are not allowed")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// isStateChanging reports whether the method can modify state. GET, HEAD
// and OPTIONS are safe; everything else (including POST) is checked for a
// foreign Origin.
func isStateChanging(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	}
	return true
}

// allowedRequestHost reports whether a Host header value names loopback or
// the explicitly bound address. Any port is accepted.
func (s *Server) allowedRequestHost(hostport string) bool {
	host := hostOnly(hostport)
	if isLoopbackHost(host) {
		return true
	}
	bind := strings.TrimSpace(s.BindAddr)
	if bind == "" {
		return false
	}
	return strings.EqualFold(host, hostOnly(bind))
}

// allowedOrigin reports whether an Origin header may drive a write. The
// origin is compared by host only, so the development proxy's port is
// accepted as long as it is loopback.
func (s *Server) allowedOrigin(origin string) bool {
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return false
	}
	return s.allowedRequestHost(u.Host)
}

// isLoopbackHost reports whether a bare host name or IP is loopback.
func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// hostOnly strips an optional port and IPv6 brackets from a Host value.
func hostOnly(hostport string) string {
	hostport = strings.TrimSpace(hostport)
	if host, _, err := net.SplitHostPort(hostport); err == nil {
		return strings.Trim(host, "[]")
	}
	return strings.Trim(hostport, "[]")
}

// requireJSONContentType checks the media type before decoding. It writes a
// 415 response and returns false when the body is not JSON.
func requireJSONContentType(w http.ResponseWriter, r *http.Request) bool {
	ct := r.Header.Get("Content-Type")
	mediaType, _, err := mime.ParseMediaType(ct)
	if err != nil || mediaType != "application/json" {
		writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type",
			"Content-Type must be application/json")
		return false
	}
	return true
}
