package utils

import (
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
)

func Address(acct codec.ShortBytes) (string, error) {
	return codec.Address(consts.HRP, acct)
}

func ParseAddress(s string) (codec.ShortBytes, error) {
	return codec.ParseAnyAddress(consts.HRP, s)
}
