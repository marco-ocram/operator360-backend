// Package respond centralizes the JSON response shapes used by handlers,
// so every endpoint emits errors in the same {"error":..., "details":...}
// envelope instead of re-declaring it inline.
package respond

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Error writes a JSON error response. If err is nil, the "details" field is
// omitted (matches handlers that report a validation error with no
// underlying Go error). Extra carries any additional context fields
// (e.g. "operator_id", "regional_office", "file_path") merged into the body.
func Error(c *gin.Context, status int, message string, err error, extra gin.H) {
	body := gin.H{"error": message}
	if err != nil {
		body["details"] = err.Error()
	}
	for k, v := range extra {
		body[k] = v
	}
	c.JSON(status, body)
}

// Unauthorized is shorthand for the 401 "missing/invalid user" responses
// repeated across every authenticated handler.
func Unauthorized(c *gin.Context, message string) {
	Error(c, http.StatusUnauthorized, message, nil, nil)
}

// OK writes a 200 JSON response.
func OK(c *gin.Context, payload gin.H) {
	c.JSON(http.StatusOK, payload)
}
