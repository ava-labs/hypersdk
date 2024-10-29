// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package auth

import (
	"errors"
	"fmt"

	"github.com/ava-labs/avalanchego/utils/wrappers"
)

var ErrAlreadyRegisteredKeyType = errors.New("already registered key type")

// WithDefaultPrivateKeyProviders registers the default PrivateKeyProviders
func WithDefaultPrivateKeyProviders(authProvider *AuthProvider, errs *wrappers.Errs) {
	errs.Add(
		authProvider.Register(ED25519Key, NewED25519PrivateKeyProvider()),
		authProvider.Register(Secp256r1Key, NewSECP256R1PrivateKeyProvider()),
		authProvider.Register(BLSKey, NewBLSPrivateKeyProvider()),
	)
}

// AuthProvider stores the used PrivateKeys types
type AuthProvider struct {
	keys map[string]PrivateKeyProvider
}

func NewAuthProvider() *AuthProvider {
	return &AuthProvider{
		keys: make(map[string]PrivateKeyProvider),
	}
}

func (p *AuthProvider) Register(key string, privateKeyProvider PrivateKeyProvider) error {
	if _, ok := p.keys[key]; ok {
		return fmt.Errorf("%w: %s", ErrAlreadyRegisteredKeyType, key)
	}
	p.keys[key] = privateKeyProvider
	return nil
}

func (p *AuthProvider) CheckType(key string) error {
	if _, ok := p.keys[key]; ok {
		return nil
	}
	return fmt.Errorf("%w: %s", ErrInvalidKeyType, key)
}

func (p *AuthProvider) GeneratePrivateKey(key string) (*PrivateKey, error) {
	if provider, ok := p.keys[key]; ok {
		return provider.GeneratePrivateKey()
	}
	return nil, fmt.Errorf("%w: %s", ErrInvalidKeyType, key)
}

func (p *AuthProvider) LoadPrivateKey(key, path string) (*PrivateKey, error) {
	if provider, ok := p.keys[key]; ok {
		return loadPrivateKeyFromFile(path, provider.GetExpectedBytesLength(), provider.LoadPrivateKey)
	}
	return nil, fmt.Errorf("%w: %s", ErrInvalidKeyType, key)
}

type PrivateKeyProvider interface {
	GeneratePrivateKey() (*PrivateKey, error)
	LoadPrivateKey(p []byte) (*PrivateKey, error)
	GetExpectedBytesLength() int
}
