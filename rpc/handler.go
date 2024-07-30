// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package rpc

import (
	"net/http"
)

type HTTPHandler struct {
	Path    string
	Handler http.Handler
}

type HandlerFactory[T any] interface {
	New(t T) (HTTPHandler, error)
}
