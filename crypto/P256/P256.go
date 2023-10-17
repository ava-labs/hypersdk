package P256

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"encoding/hex"
	"os"

	"github.com/ava-labs/avalanchego/utils/formatting/address"
	"github.com/ava-labs/hypersdk/crypto"
)

// TODO: ensure signature is not malleable (txID is just tx bytes): https://github.com/cosmos/cosmos-sdk/issues/9723
// -> https://github.com/golang/go/issues/35680
// -> https://eips.ethereum.org/EIPS/eip-7212#elliptic-curve-signature-verification-steps
// -> https://github.com/ethereum/go-ethereum/pull/27540
// -> https://github.com/ethereum/EIPs/pull/7676 (malleability check removed)
// ->-> https://github.com/ethereum/go-ethereum/pull/27540/files/7e0bc9271bc8ede1ca96c199d506368b7552ea51..cec0b058115282168c5afc5197de3f6b5479dc4a#diff-42aa89445835b22ad5e89bbea13ecfe2fa10b69084397e2dca6a826194f542e0

type (
	PublicKey  [ed25519.PublicKeySize]byte
	PrivateKey [ed25519.PrivateKeySize]byte
	Signature  [ed25519.SignatureSize]byte
)

const (
	PublicKeyLen  = ed25519.PublicKeySize
	PrivateKeyLen = ed25519.PrivateKeySize
	// PrivateKeySeedLen is defined because ed25519.PrivateKey
	// is formatted as privateKey = seed|publicKey. We use this const
	// to extract the publicKey below.
	PrivateKeySeedLen = ed25519.SeedSize
	SignatureLen      = ed25519.SignatureSize

	// TODO: make this tunable
	MinBatchSize = 16

	// TODO: make this tunable
	cacheSize = 128_000 // ~179MB (keys are ~1.4KB each)
)

var (
	EmptyPublicKey  = [ed25519.PublicKeySize]byte{}
	EmptyPrivateKey = [ed25519.PrivateKeySize]byte{}
	EmptySignature  = [ed25519.SignatureSize]byte{}
)

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

// GeneratePrivateKey returns a Ed25519 PrivateKey.
func GeneratePrivateKey() (PrivateKey, error) {
	_, k, err := ed25519.GenerateKey(nil)
	if err != nil {
		return EmptyPrivateKey, err
	}
	return PrivateKey(k), nil
}

// PublicKey returns a PublicKey associated with the Ed25519 PrivateKey p.
// The PublicKey is the last 32 bytes of p.
func (p PrivateKey) PublicKey() PublicKey {
	return PublicKey(p[PrivateKeySeedLen:])
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
	if len(bytes) != ed25519.PrivateKeySize {
		return EmptyPrivateKey, crypto.ErrInvalidPrivateKey
	}
	return PrivateKey(bytes), nil
}

// Sign returns a valid signature for msg using pk.
func Sign(msg []byte, pk PrivateKey) Signature {
	sig := ed25519.Sign(pk[:], msg)
	return Signature(sig)
}

// Verify returns whether s is a valid signature of msg by p.
func Verify(msg []byte, p PublicKey, s Signature) bool {
	return ecdsa.VerifyASN1(nil, nil, nil)
}

// HexToKey Converts a hexadecimal encoded key into a PrivateKey. Returns
// an EmptyPrivateKey and error if key is invalid.
func HexToKey(key string) (PrivateKey, error) {
	bytes, err := hex.DecodeString(key)
	if err != nil {
		return EmptyPrivateKey, err
	}
	if len(bytes) != ed25519.PrivateKeySize {
		return EmptyPrivateKey, crypto.ErrInvalidPrivateKey
	}
	return PrivateKey(bytes), nil
}

// TODO: offer DER to fixed conversion as utility
