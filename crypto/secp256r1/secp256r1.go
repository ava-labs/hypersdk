package secp256r1

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"math/big"
	"os"

	"github.com/ava-labs/avalanchego/utils/formatting/address"
	"github.com/ava-labs/hypersdk/crypto"
	"golang.org/x/crypto/cryptobyte"
	"golang.org/x/crypto/cryptobyte/asn1"
)

const (
	PublicKeyLen  = 64 // x || y
	PrivateKeyLen = 32
	SignatureLen  = 64 // r || s

	coordinateLen = 32
	rsLen         = 32
)

type (
	PublicKey  [PublicKeyLen]byte
	PrivateKey [PrivateKeyLen]byte
	Signature  [SignatureLen]byte
)

var (
	EmptyPublicKey  = [PublicKeyLen]byte{}
	EmptyPrivateKey = [PrivateKeyLen]byte{}
	EmptySignature  = [SignatureLen]byte{}
)

// secp256r1Order returns the curve order for the secp256r1 (P-256) curve.
//
// source: https://github.com/cosmos/cosmos-sdk/blob/b71ec62807628b9a94bef32071e1c8686fcd9d36/crypto/keys/internal/ecdsa/privkey.go#L12-L37
// source: https://github.com/bitcoin/bips/blob/master/bip-0062.mediawiki#low-s-values-in-signatures
var secp256r1Order = elliptic.P256().Params().N

// secp256r1HalfOrder returns half the curve order of the secp256r1 (P-256) curve.
//
// source: https://github.com/cosmos/cosmos-sdk/blob/b71ec62807628b9a94bef32071e1c8686fcd9d36/crypto/keys/internal/ecdsa/privkey.go#L12-L37
// source: https://github.com/bitcoin/bips/blob/master/bip-0062.mediawiki#low-s-values-in-signatures
var secp256r1HalfOrder = new(big.Int).Div(secp256r1Order, big.NewInt(2))

// IsNormalized returns true if [s] falls in the lower half of the curve order (inclusive).
// This should be used when verifying signatures to ensure they are not malleable.
//
// source: https://github.com/cosmos/cosmos-sdk/blob/b71ec62807628b9a94bef32071e1c8686fcd9d36/crypto/keys/internal/ecdsa/privkey.go#L12-L37
// source: https://github.com/bitcoin/bips/blob/master/bip-0062.mediawiki#low-s-values-in-signatures
func IsNormalized(s *big.Int) bool {
	return s.Cmp(secp256r1HalfOrder) != 1
}

// NormalizeSignature inverts [s] if it is not in the lower half of the curve order.
//
// source: https://github.com/cosmos/cosmos-sdk/blob/b71ec62807628b9a94bef32071e1c8686fcd9d36/crypto/keys/internal/ecdsa/privkey.go#L12-L37
// source: https://github.com/bitcoin/bips/blob/master/bip-0062.mediawiki#low-s-values-in-signatures
func NormalizeSignature(s *big.Int) *big.Int {
	if IsNormalized(s) {
		return s
	}
	return new(big.Int).Sub(secp256r1Order, s)
}

// ParseASN1Signature parses an ASN.1 encoded (using DER serialization) secp256r1 signature.
// This function does not normalize the extracted signature.
//
// source: https://cs.opensource.google/go/go/+/refs/tags/go1.21.3:src/crypto/ecdsa/ecdsa.go;l=549
func ParseASN1Signature(sig []byte) (r, s []byte, err error) {
	var inner cryptobyte.String
	input := cryptobyte.String(sig)
	if !input.ReadASN1(&inner, asn1.SEQUENCE) ||
		!input.Empty() ||
		!inner.ReadASN1Integer(&r) ||
		!inner.ReadASN1Integer(&s) ||
		!inner.Empty() {
		return nil, nil, errors.New("invalid ASN.1")
	}
	return r, s, nil
}

// Address returns a Bech32 address from hrp and p.
// This function uses avalanchego's FormatBech32 function.
func Address(hrp string, p PublicKey) string {
	// TODO: handle error
	addrString, _ := address.FormatBech32(hrp, p[:])
	return addrString
}

// ParseAddress parses a Bech32 encoded address string and extracts
// its public key. If there is an error reading the address or the hrp
// value is not valid, ParseAddress returns an EmptyPublicKey and error.
func ParseAddress(hrp, saddr string) (PublicKey, error) {
	phrp, pk, err := address.ParseBech32(saddr)
	if err != nil {
		return EmptyPublicKey, err
	}
	if phrp != hrp {
		return EmptyPublicKey, crypto.ErrIncorrectHrp
	}
	// The parsed public key may be greater than [PublicKeyLen] because the
	// underlying Bech32 implementation requires bytes to each encode 5 bits
	// instead of 8 (and we must pad the input to ensure we fill all bytes):
	// https://github.com/btcsuite/btcd/blob/902f797b0c4b3af3f7196d2f5d2343931d1b2bdf/btcutil/bech32/bech32.go#L325-L331
	if len(pk) < PublicKeyLen {
		return EmptyPublicKey, crypto.ErrInvalidPublicKey
	}
	return PublicKey(pk[:PublicKeyLen]), nil
}

// GeneratePrivateKey returns a secp256r1 PrivateKey.
func GeneratePrivateKey() (PrivateKey, error) {
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return EmptyPrivateKey, err
	}
	return PrivateKey(k.D.Bytes()), nil
}

// PublicKey returns a PublicKey associated with the secp256r1 PrivateKey p.
//
// source: https://github.com/cosmos/cosmos-sdk/blob/b71ec62807628b9a94bef32071e1c8686fcd9d36/crypto/keys/internal/ecdsa/privkey.go#L120-L121
func (p PrivateKey) PublicKey() PublicKey {
	x, y := elliptic.P256().ScalarBaseMult(p[:])
	pk := make([]byte, PublicKeyLen)
	copy(pk[:coordinateLen], x.Bytes())
	copy(pk[coordinateLen:], y.Bytes())
	return PublicKey(pk)
}

// ToHex converts a PrivateKey to a hex string.
func (p PrivateKey) ToHex() string {
	return hex.EncodeToString(p[:])
}

// Save writes [PrivateKey] to a file [filename]. If filename does
// not exist, it creates a new file with read/write permissions (0o600).
func (p PrivateKey) Save(filename string) error {
	return os.WriteFile(filename, p[:], 0o600)
}

// LoadKey returns a PrivateKey from a file filename.
// If there is an error reading the file, or the file contains an
// invalid PrivateKey, LoadKey returns an EmptyPrivateKey and an error.
func LoadKey(filename string) (PrivateKey, error) {
	bytes, err := os.ReadFile(filename)
	if err != nil {
		return EmptyPrivateKey, err
	}
	if len(bytes) != PrivateKeyLen {
		return EmptyPrivateKey, crypto.ErrInvalidPrivateKey
	}
	return PrivateKey(bytes), nil
}

// Sign returns a valid signature for msg using pk.
//
// This function also adjusts [s] to be in the lower
// half of the curve order.
func Sign(msg []byte, pk PrivateKey) (Signature, error) {
	// Parse PrivateKey
	pub := pk.PublicKey()
	x := new(big.Int).SetBytes(pub[:coordinateLen])
	y := new(big.Int).SetBytes(pub[coordinateLen:])
	priv := &ecdsa.PrivateKey{
		PublicKey: ecdsa.PublicKey{
			Curve: elliptic.P256(),
			X:     x,
			Y:     y,
		},
		D: new(big.Int).SetBytes(pk[:]),
	}

	// Sign message
	digest := sha256.Sum256(msg)
	r, s, err := ecdsa.Sign(rand.Reader, priv, digest[:])
	if err != nil {
		return EmptySignature, err
	}

	// Construct signature
	ns := NormalizeSignature(s)
	sig := make([]byte, SignatureLen)
	copy(sig[:rsLen], r.Bytes())
	copy(sig[rsLen:], ns.Bytes())
	return Signature(sig), nil
}

// Verify returns whether sig is a valid signature of msg by p.
//
// The value of [s] in [sig] must be in the lower half of the curve
// order for the signature to be considered valid.
func Verify(msg []byte, p PublicKey, sig Signature) bool {
	// Perform sanity checks
	if len(p) != PublicKeyLen {
		return false
	}
	if len(sig) != SignatureLen {
		return false
	}

	// Parse PublicKey
	x := new(big.Int).SetBytes(p[:coordinateLen])
	y := new(big.Int).SetBytes(p[coordinateLen:])
	pk := &ecdsa.PublicKey{
		Curve: elliptic.P256(),
		X:     x,
		Y:     y,
	}

	// Check if the public key coordinates are on the curve
	//
	// source: https://github.com/decred/dcrd/blob/672c3458fb0ab4761661adafbc034922a12d68d7/dcrec/secp256k1/ecdsa/signature.go#L944-L949
	if x == nil || y == nil || !elliptic.P256().IsOnCurve(x, y) {
		return false
	}

	// Check if the public key coordinates are infinity
	//
	// source: https://github.com/decred/dcrd/blob/672c3458fb0ab4761661adafbc034922a12d68d7/dcrec/secp256k1/ecdsa/signature.go#L985-L994
	if x.Sign() == 0 && y.Sign() == 0 {
		return false
	}

	// Parse Signature
	r := new(big.Int).SetBytes(sig[:rsLen])
	s := new(big.Int).SetBytes(sig[rsLen:])

	// Check if s is normalized
	if !IsNormalized(s) {
		return false
	}

	// Check if signature is valid
	digest := sha256.Sum256(msg)
	return ecdsa.Verify(pk, digest[:], r, s)
}

// HexToKey Converts a hexadecimal encoded key into a PrivateKey. Returns
// an EmptyPrivateKey and error if key is invalid.
func HexToKey(key string) (PrivateKey, error) {
	bytes, err := hex.DecodeString(key)
	if err != nil {
		return EmptyPrivateKey, err
	}
	if len(bytes) != PrivateKeyLen {
		return EmptyPrivateKey, crypto.ErrInvalidPrivateKey
	}
	return PrivateKey(bytes), nil
}
