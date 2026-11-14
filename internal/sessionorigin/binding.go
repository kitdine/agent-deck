package sessionorigin

import (
	"context"

	"github.com/kitdine/agent-deck/internal/store"
)

// SessionSourceReader reads a canonical session's indexed copies.
type SessionSourceReader interface {
	SessionOriginSourcesFor(ctx context.Context, client, sessionID string) ([]store.SessionOriginSource, error)
}

// ResolveBinding derives the override binding of an indexed canonical
// session from its stable birth. It returns nil when the index holds no copy,
// so the caller reports the target as unconfirmed instead of binding to it.
// Visibility (exclusions) stays with the caller's existing session lookup.
func ResolveBinding(ctx context.Context, index SessionSourceReader, client, sessionID string) (*store.OriginBinding, error) {
	sources, err := index.SessionOriginSourcesFor(ctx, client, sessionID)
	if err != nil || len(sources) == 0 {
		return nil, err
	}
	binding := BindingFor(client, sessionID, BirthOf(sources))
	return &binding, nil
}
