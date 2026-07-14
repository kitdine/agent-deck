//go:build darwin && cgo

package platform

/*
#cgo LDFLAGS: -framework Security -framework CoreFoundation
#include <Security/Security.h>
#include <stdlib.h>

static OSStatus putSecret(const char* service, unsigned long serviceLen, const char* account, unsigned long accountLen, const char* value, unsigned long valueLen) {
	SecKeychainItemRef item = NULL;
	OSStatus status = SecKeychainFindGenericPassword(NULL, serviceLen, service, accountLen, account, NULL, NULL, &item);
	if (status == errSecSuccess) {
		status = SecKeychainItemModifyAttributesAndData(item, NULL, valueLen, value);
		CFRelease(item);
		return status;
	}
	if (status != errSecItemNotFound) return status;
	return SecKeychainAddGenericPassword(NULL, serviceLen, service, accountLen, account, valueLen, value, NULL);
}

static OSStatus createSecret(const char* service, unsigned long serviceLen, const char* account, unsigned long accountLen, const char* value, unsigned long valueLen) {
	return SecKeychainAddGenericPassword(NULL, serviceLen, service, accountLen, account, valueLen, value, NULL);
}

static OSStatus getSecret(const char* service, unsigned long serviceLen, const char* account, unsigned long accountLen, char** value, unsigned long* valueLen) {
	UInt32 n = 0; void* data = NULL;
	OSStatus status = SecKeychainFindGenericPassword(NULL, serviceLen, service, accountLen, account, &n, &data, NULL);
	if (status == errSecSuccess) { *value = data; *valueLen = n; }
	return status;
}

static OSStatus deleteSecret(const char* service, unsigned long serviceLen, const char* account, unsigned long accountLen) {
	SecKeychainItemRef item = NULL;
	OSStatus status = SecKeychainFindGenericPassword(NULL, serviceLen, service, accountLen, account, NULL, NULL, &item);
	if (status != errSecSuccess) return status;
	status = SecKeychainItemDelete(item); CFRelease(item); return status;
}
*/
import "C"

import (
	"context"
	"errors"
	"unsafe"
)

type KeychainSecretStore struct{ Service string }

func NewKeychainSecretStore(service string) *KeychainSecretStore {
	return &KeychainSecretStore{Service: service}
}

func (s *KeychainSecretStore) Put(_ context.Context, reference, value string) error {
	service, account, secret := C.CString(s.Service), C.CString(reference), C.CString(value)
	defer C.free(unsafe.Pointer(service))
	defer C.free(unsafe.Pointer(account))
	defer C.free(unsafe.Pointer(secret))
	return keychainError(C.putSecret(service, C.ulong(len(s.Service)), account, C.ulong(len(reference)), secret, C.ulong(len(value))))
}
func (s *KeychainSecretStore) Create(_ context.Context, reference, value string) error {
	service, account, secret := C.CString(s.Service), C.CString(reference), C.CString(value)
	defer C.free(unsafe.Pointer(service))
	defer C.free(unsafe.Pointer(account))
	defer C.free(unsafe.Pointer(secret))
	return keychainError(C.createSecret(service, C.ulong(len(s.Service)), account, C.ulong(len(reference)), secret, C.ulong(len(value))))
}
func (s *KeychainSecretStore) Get(_ context.Context, reference string) (string, error) {
	service, account := C.CString(s.Service), C.CString(reference)
	defer C.free(unsafe.Pointer(service))
	defer C.free(unsafe.Pointer(account))
	var value *C.char
	var length C.ulong
	if err := keychainError(C.getSecret(service, C.ulong(len(s.Service)), account, C.ulong(len(reference)), &value, &length)); err != nil {
		return "", err
	}
	defer C.SecKeychainItemFreeContent(nil, unsafe.Pointer(value))
	return C.GoStringN(value, C.int(length)), nil
}
func (s *KeychainSecretStore) Delete(_ context.Context, reference string) error {
	service, account := C.CString(s.Service), C.CString(reference)
	defer C.free(unsafe.Pointer(service))
	defer C.free(unsafe.Pointer(account))
	return keychainError(C.deleteSecret(service, C.ulong(len(s.Service)), account, C.ulong(len(reference))))
}
func (s *KeychainSecretStore) Exists(ctx context.Context, reference string) (bool, error) {
	_, err := s.Get(ctx, reference)
	if errors.Is(err, ErrSecretNotFound) {
		return false, nil
	}
	return err == nil, err
}
func keychainError(status C.OSStatus) error {
	if status == C.errSecItemNotFound {
		return ErrSecretNotFound
	}
	if status == C.errSecDuplicateItem {
		return ErrSecretExists
	}
	if status != C.errSecSuccess {
		return errors.New("keychain_error")
	}
	return nil
}
