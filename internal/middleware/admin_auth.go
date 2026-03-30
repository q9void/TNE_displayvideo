package middleware

import (
	"net/http"
	"os"
	"strings"

	"github.com/thenexusengine/tne_springwire/pkg/logger"
)

// AdminAuth middleware protects admin endpoints with API key or HTTP Basic Auth.
// Accepts either:
//   - Bearer token matching ADMIN_API_KEY
//   - HTTP Basic Auth with ADMIN_USER / ADMIN_PASSWORD credentials
func AdminAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip auth for non-admin paths
		isAdmin := strings.HasPrefix(r.URL.Path, "/admin/") ||
			strings.HasPrefix(r.URL.Path, "/catalyst/admin")
		if !isAdmin {
			next.ServeHTTP(w, r)
			return
		}

		adminAPIKey := os.Getenv("ADMIN_API_KEY")
		adminUser := os.Getenv("ADMIN_USER")
		adminPassword := os.Getenv("ADMIN_PASSWORD")
		authRequired := os.Getenv("ADMIN_AUTH_REQUIRED") != "false" // Default: true

		anyCredConfigured := adminAPIKey != "" || (adminUser != "" && adminPassword != "")

		if !anyCredConfigured {
			if authRequired {
				logger.Log.Error().
					Str("path", r.URL.Path).
					Str("remote_addr", r.RemoteAddr).
					Msg("Admin endpoint access denied - no admin credentials configured")
				http.Error(w, "Admin endpoints disabled - no admin credentials configured", http.StatusForbidden)
				return
			}
			// Dev mode - log warning and allow
			logger.Log.Warn().
				Str("path", r.URL.Path).
				Str("remote_addr", r.RemoteAddr).
				Msg("No admin credentials set - endpoints unprotected (dev mode)")
			next.ServeHTTP(w, r)
			return
		}

		authHeader := r.Header.Get("Authorization")

		// Try HTTP Basic Auth (browser-friendly, for HTML admin pages)
		if adminUser != "" && adminPassword != "" && strings.HasPrefix(authHeader, "Basic ") {
			user, pass, ok := r.BasicAuth()
			if ok && user == adminUser && pass == adminPassword {
				logger.Log.Info().
					Str("path", r.URL.Path).
					Str("remote_addr", r.RemoteAddr).
					Msg("Admin endpoint access granted via basic auth")
				next.ServeHTTP(w, r)
				return
			}
			// Invalid basic auth credentials — challenge again
			w.Header().Set("WWW-Authenticate", `Basic realm="TNE Admin"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Try Bearer token / API key
		if adminAPIKey != "" {
			if authHeader == "" {
				// If basic auth is configured, send WWW-Authenticate to trigger browser prompt
				if adminUser != "" && adminPassword != "" {
					w.Header().Set("WWW-Authenticate", `Basic realm="TNE Admin"`)
				}
				logger.Log.Warn().
					Str("path", r.URL.Path).
					Str("remote_addr", r.RemoteAddr).
					Msg("Admin endpoint access denied - no Authorization header")
				http.Error(w, "Unauthorized - Admin credentials required", http.StatusUnauthorized)
				return
			}
			token := strings.TrimPrefix(authHeader, "Bearer ")
			token = strings.TrimSpace(token)
			if token == adminAPIKey {
				logger.Log.Info().
					Str("path", r.URL.Path).
					Str("remote_addr", r.RemoteAddr).
					Msg("Admin endpoint access granted via API key")
				next.ServeHTTP(w, r)
				return
			}
			logger.Log.Warn().
				Str("path", r.URL.Path).
				Str("remote_addr", r.RemoteAddr).
				Msg("Admin endpoint access denied - invalid credentials")
			http.Error(w, "Unauthorized - Invalid admin credentials", http.StatusUnauthorized)
			return
		}

		// Only basic auth configured but no Basic auth header present — trigger browser prompt
		w.Header().Set("WWW-Authenticate", `Basic realm="TNE Admin"`)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	})
}
