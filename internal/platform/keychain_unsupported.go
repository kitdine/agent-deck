//go:build !darwin || !cgo

package platform

import (
	"context"
	"errors"
)

var ErrKeychainUnsupported = errors.New("keychain_unsupported")

type KeychainSecretStore struct{ Service string }

func NewKeychainSecretStore(service string) *KeychainSecretStore {
	return &KeychainSecretStore{Service: service}
}
func (s *KeychainSecretStore) Create(context.Context, string, string) error {
	return ErrKeychainUnsupported
}
func (s *KeychainSecretStore) Put(context.Context, string, string) error {
	return ErrKeychainUnsupported
}
func (s *KeychainSecretStore) Get(context.Context, string) (string, error) {
	return "", ErrKeychainUnsupported
}
func (s *KeychainSecretStore) Delete(context.Context, string) error { return ErrKeychainUnsupported }
func (s *KeychainSecretStore) Exists(context.Context, string) (bool, error) {
	return false, ErrKeychainUnsupported
}
