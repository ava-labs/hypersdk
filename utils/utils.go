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
	"github.com/ava-labs/avalanchego/snow/validators"
	"github.com/ava-labs/avalanchego/utils"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	"github.com/ava-labs/avalanchego/utils/hashing"
	safemath "github.com/ava-labs/avalanchego/utils/math"
	"github.com/ava-labs/avalanchego/utils/perms"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"github.com/ava-labs/hypersdk/consts"
	formatter "github.com/onsi/ginkgo/v2/formatter"
	"golang.org/x/exp/maps"
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

// UnixRDeci returns the current unix time in milliseconds, rounded
// down to the nearsest decisecond.
//
// [now] is used as the current unix time in milliseconds if >= 0.
//
// [add] (in ms) is added to the unix time before it is rounded (typically
// used when generating an expiry time with a validity window).
func UnixRDeci(now, add int64) int64 {
	if now < 0 {
		now = time.Now().UnixMilli()
	}
	t := now + add
	return t - t%consts.MillisecondsPerDecisecond
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

// ConstructCanonicalValidatorSet constructs the validator set order to use
// for warp validation.
//
// Source: https://github.com/ava-labs/avalanchego/blob/813bd481c764970b5c47c3ae9c0a40f2c28da8e4/vms/platformvm/warp/validator.go#L61-L92
func ConstructCanonicalValidatorSet(vdrSet map[ids.NodeID]*validators.GetValidatorOutput) ([]*warp.Validator, uint64, error) {
	var (
		vdrs        = make(map[string]*warp.Validator, len(vdrSet))
		totalWeight uint64
		err         error
	)
	for _, vdr := range vdrSet {
		totalWeight, err = safemath.Add64(totalWeight, vdr.Weight)
		if err != nil {
			return nil, 0, err
		}

		if vdr.PublicKey == nil {
			continue
		}

		pkBytes := bls.SerializePublicKey(vdr.PublicKey)
		uniqueVdr, ok := vdrs[string(pkBytes)]
		if !ok {
			uniqueVdr = &warp.Validator{
				PublicKey:      vdr.PublicKey,
				PublicKeyBytes: pkBytes,
			}
			vdrs[string(pkBytes)] = uniqueVdr
		}

		uniqueVdr.Weight += vdr.Weight // Impossible to overflow here
		uniqueVdr.NodeIDs = append(uniqueVdr.NodeIDs, vdr.NodeID)
	}
	vdrList := maps.Values(vdrs)
	utils.Sort(vdrList)
	return vdrList, totalWeight, nil
}
