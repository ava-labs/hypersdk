package server

import "net/http"

type Wrapper interface {
	// WrapHandler wraps an http.Handler.
	WrapHandler(h http.Handler) http.Handler
}
