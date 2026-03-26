package middleware

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

// GetMerchantID extracts the authenticated merchant ID from the Gin context.
// Only valid on routes protected by RequireAuth.
func GetMerchantID(c *gin.Context) string {
	id, _ := c.Get(MerchantIDKey)
	return id.(string)
}

// GetMerchantEmail extracts the authenticated merchant email from the Gin context.
// Only valid on routes protected by RequireAuth.
func GetMerchantEmail(c *gin.Context) string {
	email, _ := c.Get(MerchantEmailKey)
	return email.(string)
}

// QueryInt reads an integer query param, falling back to defaultVal on missing or invalid input.
func QueryInt(c *gin.Context, key string, defaultVal int) int {
	raw := c.Query(key)
	if raw == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < 1 {
		return defaultVal
	}
	return v
}
