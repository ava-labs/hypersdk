/*
 * Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
 * See the file LICENSE for licensing terms.
 */

package rpc

import "errors"

var (
	ErrClosed         = errors.New("closed")
	ErrExpired        = errors.New("expired")
	ErrMessageMissing = errors.New("message missing")
)
