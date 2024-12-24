// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package auth

import (
	"errors"
	"fmt"
	"os"
)

var (
	ErrAlreadyRegisteredKeyType = errors.New("already registered key type")
	ErrInvalidPrivateKeySize    = errors.New("invalid private key size")
)

// WithDefaultPrivateKeyFactories registers the default PrivateKeyFactories
func WithDefaultPrivateKeyFactories(authProvider *AuthProvider) error {
	return errors.Join(
		authProvider.Register(ED25519Key, NewED25519PrivateKeyFactory()),
		authProvider.Register(Secp256r1Key, NewSECP256R1PrivateKeyFactory()),
		authProvider.Register(BLSKey, NewBLSPrivateKeyFactory()),
	)
}

// AuthProvider stores the used PrivateKeys types
type AuthProvider struct {
	keys map[string]PrivateKeyFactory
}

func NewAuthProvider() *AuthProvider {
	return &AuthProvider{
		keys: make(map[string]PrivateKeyFactory),
	}
}

func (p *AuthProvider) Register(key string, privateKeyProvider PrivateKeyFactory) error {
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
	if privateKeyFactory, ok := p.keys[key]; ok {
		return privateKeyFactory.GeneratePrivateKey()
	}
	return nil, fmt.Errorf("%w: %s", ErrInvalidKeyType, key)
}

func (p *AuthProvider) LoadPrivateKey(key, path string) (*PrivateKey, error) {
	privateKeyFactory, ok := p.keys[key]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrInvalidKeyType, key)
	}
	privateKey, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return privateKeyFactory.LoadPrivateKey(privateKey)
}

type PrivateKeyFactory interface {
	GeneratePrivateKey() (*PrivateKey, error)
	LoadPrivateKey(p []byte) (*PrivateKey, error)
}
