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
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/hashing"
	"github.com/ava-labs/avalanchego/utils/perms"
	"github.com/onsi/ginkgo/v2/formatter"

	"github.com/ava-labs/hypersdk/consts"
)

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

func GetPort(uri string) (string, error) {
	purl, err := url.Parse(uri)
	if err != nil {
		return "", err
	}
	return purl.Port(), err
}

func FormatBalance(bal uint64, decimals uint8) string {
	return strconv.FormatFloat(float64(bal)/math.Pow10(int(decimals)), 'f', int(decimals), 64)
}

func ParseBalance(bal string, decimals uint8) (uint64, error) {
	f, err := strconv.ParseFloat(bal, 64)
	if err != nil {
		return 0, err
	}
	return uint64(f * math.Pow10(int(decimals))), nil
}

func Repeat[T any](v T, n int) []T {
	arr := make([]T, n)
	for i := 0; i < n; i++ {
		arr[i] = v
	}
	return arr
}

// UnixRMilli returns the current unix time in milliseconds, rounded
// down to the nearsest second.
//
// [now] is used as the current unix time in milliseconds if >= 0.
//
// [add] (in ms) is added to the unix time before it is rounded (typically
// used when generating an expiry time with a validity window).
func UnixRMilli(now, add int64) int64 {
	if now < 0 {
		now = time.Now().UnixMilli()
	}
	t := now + add
	return t - t%consts.MillisecondsPerSecond
}

// SaveBytes writes [b] to a file [filename]. If filename does
// not exist, it creates a new file with read/write permissions (0o600).
func SaveBytes(filename string, b []byte) error {
	return os.WriteFile(filename, b, 0o600)
}

// LoadBytes returns bytes stored at a file [filename].
func LoadBytes(filename string, expectedSize int) ([]byte, error) {
	bytes, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	if expectedSize != -1 && len(bytes) != expectedSize {
		return nil, ErrInvalidSize
	}
	return bytes, nil
}
