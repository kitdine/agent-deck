package usage

// SessionKey names one canonical session by client and session ID.
type SessionKey struct {
	Client    string
	SessionID string
}

// SessionFilter restricts work signals to a selected session set. It is a
// selection over existing facts: events, turns and calls outside it are left
// out of the aggregates, never deleted, repriced or attributed elsewhere. A
// nil filter selects every session.
//
// An include filter selects exactly its keys. An exclude filter selects every
// session except its keys, so a session the selection never classified (for
// example a usage-only session with no visible index entry) stays selected.
type SessionFilter struct {
	keys    map[SessionKey]struct{}
	include bool
}

// IncludeSessions selects only the given sessions.
func IncludeSessions(keys []SessionKey) *SessionFilter {
	return newSessionFilter(keys, true)
}

// ExcludeSessions selects every session except the given ones.
func ExcludeSessions(keys []SessionKey) *SessionFilter {
	return newSessionFilter(keys, false)
}

func newSessionFilter(keys []SessionKey, include bool) *SessionFilter {
	filter := &SessionFilter{keys: make(map[SessionKey]struct{}, len(keys)), include: include}
	for _, key := range keys {
		filter.keys[key] = struct{}{}
	}
	return filter
}

// Allows reports whether the session is selected.
func (f *SessionFilter) Allows(client, sessionID string) bool {
	if f == nil {
		return true
	}
	_, listed := f.keys[SessionKey{Client: client, SessionID: sessionID}]
	return listed == f.include
}
