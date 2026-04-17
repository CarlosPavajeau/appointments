package prometheus

import (
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func New() (*gin.Engine, error) {
	r := gin.New()

	// Register the Prometheus metrics handler at the /metrics endpoint
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	return r, nil
}
