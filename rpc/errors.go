package rpc

import "errors"

var (
	ErrClosed         = errors.New("closed")
	ErrExpired        = errors.New("expired")
	ErrMessageMissing = errors.New("message missing")
)
