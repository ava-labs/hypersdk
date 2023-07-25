// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package codec

import "github.com/ava-labs/hypersdk/consts"

func BytesLen(msg []byte) int {
	return consts.IntLen + len(msg)
}

func StringLen(msg string) int {
	return consts.IntLen + len(msg)
}
