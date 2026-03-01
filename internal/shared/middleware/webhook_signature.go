package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func WhatsAppSignature(appSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		signature := c.GetHeader("X-Hub-Signature-256")
		if signature == "" {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}

		if !validSignature(body, signature, appSecret) {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		c.Request.Body = io.NopCloser(strings.NewReader(string(body)))
		c.Next()
	}
}

func validSignature(body []byte, signature, secret string) bool {
	// Headers coming like "sha256=<hash>"
	parts := strings.SplitN(signature, "=", 2)
	if len(parts) != 2 || parts[0] != "sha256" {
		return false
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(parts[1]), []byte(expected))
}
