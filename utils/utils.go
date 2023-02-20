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
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/utils/hashing"
	"github.com/ava-labs/avalanchego/utils/json"
	"github.com/ava-labs/avalanchego/utils/perms"
	"github.com/gorilla/rpc/v2"
	formatter "github.com/onsi/ginkgo/v2/formatter"
)

func ToID(bytes []byte) ids.ID {
	return ids.ID(hashing.ComputeHash256Array(bytes))
}

func Max(x, y uint64) uint64 {
	if x > y {
		return x
	}
	return y
}

func Min(x, y uint64) uint64 {
	if x < y {
		return x
	}
	return y
}

func InitSubDirectory(rootPath string, name string) (string, error) {
	p := path.Join(rootPath, name)
	return p, os.MkdirAll(p, perms.ReadWriteExecute)
}

// NewHandler returns a new Handler for a service where:
//   - The handler's functionality is defined by [service]
//     [service] should be a gorilla RPC service (see https://www.gorillatoolkit.org/pkg/rpc/v2)
//   - The name of the service is [name]
//   - The LockOption is the first element of [lockOption]
//     By default the LockOption is WriteLock
//     [lockOption] should have either 0 or 1 elements. Elements beside the first are ignored.
func NewHandler(
	name string,
	service interface{},
	lockOption ...common.LockOption,
) (*common.HTTPHandler, error) {
	server := rpc.NewServer()
	server.RegisterCodec(json.NewCodec(), "application/json")
	server.RegisterCodec(json.NewCodec(), "application/json;charset=UTF-8")
	if err := server.RegisterService(service, name); err != nil {
		return nil, err
	}

	var lock common.LockOption = common.NoLock
	if len(lockOption) != 0 {
		lock = lockOption[0]
	}
	return &common.HTTPHandler{LockOptions: lock, Handler: server}, nil
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
	host, _, _ := net.SplitHostPort(purl.Host)
	return host, nil
}

func FormatBalance(bal uint64) string {
	return fmt.Sprintf("%f", float64(bal)/math.Pow10(9))
}

func ParseBalance(bal string) (uint64, error) {
	f, err := strconv.ParseFloat(bal, 64)
	if err != nil {
		return 0, err
	}
	return uint64(f * math.Pow10(9)), nil
}

func Repeat[T any](v T, n int) []T {
	arr := make([]T, n)
	for i := 0; i < n; i++ {
		arr[i] = v
	}
	return arr
}
