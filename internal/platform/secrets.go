package platform

import (
	"context"
	"errors"
	"sync"
)

var (
	ErrSecretNotFound = errors.New("secret_not_found")
	ErrSecretExists   = errors.New("secret_exists")
)

// SecretStore keeps credential values outside AgentDeck's SQLite state.
type SecretStore interface {
	Create(context.Context, string, string) error
	Put(context.Context, string, string) error
	Get(context.Context, string) (string, error)
	Delete(context.Context, string) error
	Exists(context.Context, string) (bool, error)
}

// MemorySecretStore is a test-only isolated secret store. Production callers
// receive a platform implementation through their composition root.
type MemorySecretStore struct {
	mu      sync.Mutex
	secrets map[string]string
}

func NewMemorySecretStore() *MemorySecretStore {
	return &MemorySecretStore{secrets: map[string]string{}}
}
func (s *MemorySecretStore) Create(_ context.Context, ref, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.secrets[ref]; exists {
		return ErrSecretExists
	}
	s.secrets[ref] = value
	return nil
}
func (s *MemorySecretStore) Put(_ context.Context, ref, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.secrets[ref] = value
	return nil
}
func (s *MemorySecretStore) Get(_ context.Context, ref string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, ok := s.secrets[ref]
	if !ok {
		return "", ErrSecretNotFound
	}
	return value, nil
}
func (s *MemorySecretStore) Delete(_ context.Context, ref string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.secrets[ref]; !ok {
		return ErrSecretNotFound
	}
	delete(s.secrets, ref)
	return nil
}
func (s *MemorySecretStore) Exists(_ context.Context, ref string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.secrets[ref]
	return ok, nil
}
