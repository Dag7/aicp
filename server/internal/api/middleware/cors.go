package middleware

import (
	"net/http"

	"github.com/rs/cors"
)

// CORS returns CORS middleware with AICP-appropriate defaults.
// When origins are explicit, credentials are allowed. When origins are
// wildcard ("*"), credentials are disabled to comply with the CORS spec
// and prevent CSRF attacks.
func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	if len(allowedOrigins) == 0 {
		allowedOrigins = []string{"*"}
	}

	// AllowCredentials: true is incompatible with wildcard origins per CORS spec.
	// Only enable credentials when explicit origins are configured.
	allowCredentials := true
	for _, origin := range allowedOrigins {
		if origin == "*" {
			allowCredentials = false
			break
		}
	}

	c := cors.New(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type", "X-Request-ID"},
		ExposedHeaders:   []string{"X-Request-ID"},
		AllowCredentials: allowCredentials,
		MaxAge:           86400,
	})

	return c.Handler
}
