package auth

import (
	"crypto/subtle"
	"net/http"
	"os"
	"strings"
)

// RequireBearerToken protects every MCP request, including resource reads and
// session establishment. Network transports fail closed without a configured key.
func RequireBearerToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := os.Getenv("SLACK_MCP_API_KEY")
		if key == "" {
			key = os.Getenv("SLACK_MCP_SSE_API_KEY")
		}
		if key == "" {
			http.Error(w, "MCP API key is not configured", http.StatusServiceUnavailable)
			return
		}
		scheme, token, ok := strings.Cut(r.Header.Get("Authorization"), " ")
		if !ok || !strings.EqualFold(scheme, "Bearer") || subtle.ConstantTimeCompare([]byte(key), []byte(token)) != 1 {
			w.Header().Set("WWW-Authenticate", `Bearer realm="slack-mcp"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
