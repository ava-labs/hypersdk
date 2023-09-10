// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package codec

import "github.com/ava-labs/hypersdk/consts"

type SizeType interface {
	Size() int
}

func CummSize[T SizeType](arr []T) int {
	size := 0
	for _, item := range arr {
		size += item.Size()
	}
	return size
}

func BytesLen(msg []byte) int {
	return consts.IntLen + len(msg)
}

func BytesLenSize(msgSize int) int {
	return consts.IntLen + msgSize
}

func StringLen(msg string) int {
	return consts.IntLen + len(msg)
}
