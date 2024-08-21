// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package api

import (
	"net/http"
)

const Name = "hypersdk"

type Handler struct {
	Path    string
	Handler http.Handler
}

type HandlerFactory[T any] interface {
	New(t T) (Handler, error)
}
