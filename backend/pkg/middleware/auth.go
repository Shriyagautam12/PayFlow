package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/Shriyagautam12/PayFlow/internal/auth"
)

const MerchantIDKey = "merchant_id"
const MerchantEmailKey = "merchant_email"

// RequireAuth is a Gin middleware that validates the JWT on every protected route.
// It extracts the Bearer token from the Authorization header, validates it,
// and injects merchant_id into the request context for downstream handlers.
func RequireAuth(tokens *auth.TokenService) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authorization header required"})
			return
		}

		// Expect: "Bearer <token>"
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization format"})
			return
		}

		claims, err := tokens.ValidateAccessToken(parts[1])
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}

		// Inject into context — downstream handlers call GetMerchantID(c)
		c.Set(MerchantIDKey, claims.MerchantID)
		c.Set(MerchantEmailKey, claims.Email)
		c.Next()
	}
}

// GetMerchantID extracts the authenticated merchant ID from the Gin context.
// Call this in any handler protected by RequireAuth.
func GetMerchantID(c *gin.Context) string {
	id, _ := c.Get(MerchantIDKey)
	return id.(string)
}
