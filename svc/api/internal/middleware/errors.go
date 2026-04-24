package middleware

import (
	"net/http"
	"wappiz/pkg/codes"
	"wappiz/pkg/fault"
	"wappiz/svc/api/openapi"

	"github.com/gin-gonic/gin"
)

// WithErrorHandling returns middleware that translates errors into appropriate
func WithErrorHandling() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) > 0 {
			err := c.Errors.Last().Err
			urn, ok := fault.GetCode(err)

			if !ok {
				urn = "unknown"
			}

			switch urn {
			case codes.ErrorsForbiddenResourceQuotaExceeded:
				c.AbortWithStatusJSON(http.StatusForbidden, openapi.ForbiddenErrorResponse{
					Meta: openapi.Meta{
						RequestId: c.GetString("request_id"),
					},
					Error: openapi.BaseError{
						Title:  "Forbidden",
						Type:   string(urn),
						Detail: fault.UserFacingMessage(err),
						Status: http.StatusForbidden,
					},
				})

			default:
				c.AbortWithStatusJSON(http.StatusInternalServerError, openapi.InternalServerErrorResponse{
					Meta: openapi.Meta{
						RequestId: c.GetString("request_id"),
					},
					Error: openapi.BaseError{
						Title:  "Internal Server Error",
						Type:   string(urn),
						Detail: fault.UserFacingMessage(err),
						Status: http.StatusInternalServerError,
					},
				})
			}
		}
	}
}
