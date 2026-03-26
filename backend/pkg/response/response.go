package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Error is the standard error envelope for all API responses.
type Error struct {
	Error string `json:"error"`
}

// OK is the standard success envelope when there is no domain-specific body.
type OK struct {
	Message string `json:"message"`
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// Err writes a JSON error response with the given status code.
func Err(c *gin.Context, status int, msg string) {
	c.JSON(status, Error{Error: msg})
}

// BadRequest writes a 400 JSON error.
func BadRequest(c *gin.Context, msg string) {
	Err(c, http.StatusBadRequest, msg)
}

// Unauthorized writes a 401 JSON error.
func Unauthorized(c *gin.Context, msg string) {
	Err(c, http.StatusUnauthorized, msg)
}

// Forbidden writes a 403 JSON error.
func Forbidden(c *gin.Context, msg string) {
	Err(c, http.StatusForbidden, msg)
}

// NotFound writes a 404 JSON error.
func NotFound(c *gin.Context, msg string) {
	Err(c, http.StatusNotFound, msg)
}

// Conflict writes a 409 JSON error.
func Conflict(c *gin.Context, msg string) {
	Err(c, http.StatusConflict, msg)
}

// PaymentRequired writes a 402 JSON error.
func PaymentRequired(c *gin.Context, msg string) {
	Err(c, http.StatusPaymentRequired, msg)
}

// Internal writes a 500 JSON error.
func Internal(c *gin.Context, msg string) {
	Err(c, http.StatusInternalServerError, msg)
}
