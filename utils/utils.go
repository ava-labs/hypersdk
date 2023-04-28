// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package utils

import (
	"fmt"
	"math"
	"net"
	"net/url"
	"os"
	"path"
	"strconv"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/hashing"
	"github.com/ava-labs/avalanchego/utils/perms"
	formatter "github.com/onsi/ginkgo/v2/formatter"
)

const NativeDecimals = 9

func ToID(bytes []byte) ids.ID {
	return ids.ID(hashing.ComputeHash256Array(bytes))
}

func InitSubDirectory(rootPath string, name string) (string, error) {
	p := path.Join(rootPath, name)
	return p, os.MkdirAll(p, perms.ReadWriteExecute)
}

func ErrBytes(err error) []byte {
	return []byte(err.Error())
}

// Outputs to stdout.
//
// e.g.,
//
//	Out("{{green}}{{bold}}hi there %q{{/}}", "aa")
//	Out("{{magenta}}{{bold}}hi therea{{/}} {{cyan}}{{underline}}b{{/}}")
//
// ref.
// https://github.com/onsi/ginkgo/blob/v2.0.0/formatter/formatter.go#L52-L73
func Outf(format string, args ...interface{}) {
	s := formatter.F(format, args...)
	fmt.Fprint(formatter.ColorableStdOut, s)
}

func GetHost(uri string) (string, error) {
	purl, err := url.Parse(uri)
	if err != nil {
		return "", err
	}
	host, _, err := net.SplitHostPort(purl.Host)
	return host, err
}

func FormatBalance(bal uint64) string {
	return fmt.Sprintf("%.9f", float64(bal)/math.Pow10(NativeDecimals))
}

func ParseBalance(bal string) (uint64, error) {
	f, err := strconv.ParseFloat(bal, 64)
	if err != nil {
		return 0, err
	}
	return uint64(f * math.Pow10(NativeDecimals)), nil
}

func Repeat[T any](v T, n int) []T {
	arr := make([]T, n)
	for i := 0; i < n; i++ {
		arr[i] = v
	}
	return arr
}
