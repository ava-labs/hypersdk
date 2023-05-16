// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

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
