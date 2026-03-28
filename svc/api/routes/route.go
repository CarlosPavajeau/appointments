// Package routes defines the HTTP routing layer for the API service.
// It provides the [Route] and [Handler] interfaces that every endpoint must
// implement, the [Services] dependency container, and the [Register] function
// that wires everything onto a [gin.Engine].
package routes

import (
	"github.com/gin-gonic/gin"
)

// Handler is the core interface for an HTTP endpoint. Every route handler
// must implement Handle to process a request and write a response.
type Handler interface {
	Handle(ctx *gin.Context)
}

// HandleFunc is a function type that satisfies [Handler], allowing plain
// functions to be used as handlers without defining a struct.
type HandleFunc func(ctx *gin.Context)

// Route extends [Handler] with the HTTP method and URL path that the handler
// should be mounted on. Implementing this interface allows each handler to be
// self-describing and registered via [RegisterRoute] without a central routing
// table.
type Route interface {
	Handler

	// Method returns the HTTP method (e.g. "GET", "POST") for this route.
	Method() string
	// Path returns the URL path pattern (e.g. "/v1/tenants/:id") for this route.
	Path() string
}
