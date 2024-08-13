package pubsub

import "errors"

var (
	ErrFilterNotInitialized = errors.New("filter not initialized")
	ErrAddressLimit         = errors.New("address limit exceeded")
	ErrInvalidFilterParam   = errors.New("invalid bloom filter params")
	ErrInvalidCommand       = errors.New("invalid command")
	ErrMessageTooLarge      = errors.New("message too large")
	ErrClosed               = errors.New("closed")
)
