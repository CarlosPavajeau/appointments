package routes

import (
	"github.com/gin-gonic/gin"
)

type Handler interface {
	Handle(ctx *gin.Context)
}

type HandleFunc func(ctx *gin.Context)

type Route interface {
	Handler

	Method() string
	Path() string
}
