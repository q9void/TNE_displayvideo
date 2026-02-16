package middleware

import (
	"net/http"
	"os"
	"strings"

	"github.com/thenexusengine/tne_springwire/pkg/logger"
)

// AdminAuth middleware protects admin endpoints with API key authentication
func AdminAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip auth for non-admin paths
		if !strings.HasPrefix(r.URL.Path, "/admin/") {
			next.ServeHTTP(w, r)
			return
		}

		// Get admin API key from environment
		adminAPIKey := os.Getenv("ADMIN_API_KEY")
		authRequired := os.Getenv("ADMIN_AUTH_REQUIRED") != "false" // Default: true

		if adminAPIKey == "" {
			if authRequired {
				// FAIL CLOSED - reject when key not configured
				logger.Log.Error().
					Str("path", r.URL.Path).
					Str("remote_addr", r.RemoteAddr).
					Msg("Admin endpoint access denied - ADMIN_API_KEY not set")
				http.Error(w, "Admin endpoints disabled - ADMIN_API_KEY not set", http.StatusForbidden)
				return
			}
			// Dev mode - log warning and allow
			logger.Log.Warn().
				Str("path", r.URL.Path).
				Str("remote_addr", r.RemoteAddr).
				Msg("ADMIN_API_KEY not set - endpoints unprotected (dev mode)")
			next.ServeHTTP(w, r)
			return
		}

		// Check Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			logger.Log.Warn().
				Str("path", r.URL.Path).
				Str("remote_addr", r.RemoteAddr).
				Msg("Admin endpoint access denied - no Authorization header")
			http.Error(w, "Unauthorized - Admin API key required", http.StatusUnauthorized)
			return
		}

		// Support both "Bearer TOKEN" and "TOKEN" formats
		token := strings.TrimPrefix(authHeader, "Bearer ")
		token = strings.TrimSpace(token)

		if token != adminAPIKey {
			logger.Log.Warn().
				Str("path", r.URL.Path).
				Str("remote_addr", r.RemoteAddr).
				Msg("Admin endpoint access denied - invalid API key")
			http.Error(w, "Unauthorized - Invalid admin API key", http.StatusUnauthorized)
			return
		}

		// Authentication successful
		logger.Log.Info().
			Str("path", r.URL.Path).
			Str("remote_addr", r.RemoteAddr).
			Msg("Admin endpoint access granted")

		next.ServeHTTP(w, r)
	})
}
