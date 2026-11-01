package store

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"sync"
)

type readPoolKey struct{ path, profile string }

type readPoolEntry struct {
	file    os.FileInfo
	db      *sql.DB
	ready   chan struct{}
	err     error
	users   int
	used    uint64
	retired bool
}

// Pools own connections for the process; request-scoped Stores/readers own
// leases. Separate profiles keep the identity reader's short busy timeout from
// changing ordinary reads. No generation, schema or query result is cached.
type readPoolCache struct {
	mu      sync.Mutex
	entries map[readPoolKey]*readPoolEntry
	limit   int
	clock   uint64
}

var processReadPools = readPoolCache{limit: 32}

func (p *readPoolCache) acquire(ctx context.Context, path, profile string, open func() (*sql.DB, error)) (*sql.DB, func() error, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return nil, nil, err
	}
	file, err := os.Stat(path)
	if err != nil {
		return nil, nil, err
	}
	key := readPoolKey{path, profile}
	for {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		p.mu.Lock()
		if p.entries == nil {
			p.entries = make(map[readPoolKey]*readPoolEntry)
		}
		if entry := p.entries[key]; entry != nil && os.SameFile(entry.file, file) {
			if ready := entry.ready; ready != nil {
				p.mu.Unlock()
				select {
				case <-ctx.Done():
					return nil, nil, ctx.Err()
				case <-ready:
					if entry.err != nil {
						// The initializing request's cancellation must not cancel
						// an independent waiter with a live deadline.
						if errors.Is(entry.err, context.Canceled) || errors.Is(entry.err, context.DeadlineExceeded) {
							continue
						}
						return nil, nil, entry.err
					}
					continue
				}
			}
			entry.users++
			p.clock++
			entry.used = p.clock
			p.mu.Unlock()
			return entry.db, p.release(entry), nil
		}
		var closeDB *sql.DB
		if previous := p.entries[key]; previous != nil {
			delete(p.entries, key)
			previous.retired = true
			if previous.users == 0 {
				closeDB = previous.db
			}
		}
		if len(p.entries) >= p.limit {
			var oldest *readPoolEntry
			var oldestKey readPoolKey
			for candidateKey, candidate := range p.entries {
				if candidate.users == 0 && (oldest == nil || candidate.used < oldest.used) {
					oldest, oldestKey = candidate, candidateKey
				}
			}
			if oldest != nil {
				delete(p.entries, oldestKey)
				oldest.retired = true
				// A replaced entry already freed a slot, so at most one idle
				// connection needs closing here.
				closeDB = oldest.db
			}
		}
		p.clock++
		entry := &readPoolEntry{file: file, users: 1, used: p.clock, ready: make(chan struct{})}
		if len(p.entries) < p.limit {
			p.entries[key] = entry
		} else {
			entry.retired = true // All cached pools are in use; do not grow the cache.
		}
		p.mu.Unlock()
		if closeDB != nil {
			_ = closeDB.Close()
		}
		db, err := open()
		if err == nil {
			if profile == "identity" {
				db.SetMaxOpenConns(1)
			}
			db.SetMaxIdleConns(1)
			err = db.PingContext(ctx)
		}
		if err == nil {
			current, statErr := os.Stat(path)
			if statErr != nil {
				err = statErr
			} else if !os.SameFile(file, current) {
				err = errors.New("database replaced while opening read pool")
			}
		}
		p.mu.Lock()
		entry.db, entry.err = db, err
		ready := entry.ready
		entry.ready = nil
		if err != nil {
			if p.entries[key] == entry {
				delete(p.entries, key)
			}
			entry.users, entry.retired = 0, true
		}
		close(ready)
		p.mu.Unlock()
		if err != nil {
			if db != nil {
				_ = db.Close()
			}
			return nil, nil, err
		}
		return db, p.release(entry), nil
	}
}

// CloseReadPools releases process-owned idle connections on orderly shutdown.
// Active leases are retired and close their connections when released.
func CloseReadPools() error { return processReadPools.close() }

func (p *readPoolCache) close() error {
	p.mu.Lock()
	var databases []*sql.DB
	for key, entry := range p.entries {
		delete(p.entries, key)
		entry.retired = true
		if entry.users == 0 && entry.db != nil {
			databases = append(databases, entry.db)
		}
	}
	p.mu.Unlock()
	var result error
	for _, db := range databases {
		result = errors.Join(result, db.Close())
	}
	return result
}

func (p *readPoolCache) contains(path string) bool {
	path, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	for key := range p.entries {
		if key.path == path {
			return true
		}
	}
	return false
}

func (p *readPoolCache) release(entry *readPoolEntry) func() error {
	var once sync.Once
	var err error
	return func() error {
		once.Do(func() {
			p.mu.Lock()
			entry.users--
			closeDB := entry.retired && entry.users == 0
			p.mu.Unlock()
			if closeDB {
				err = entry.db.Close()
			}
		})
		return err
	}
}

func acquireReadOnlySQL(ctx context.Context, path, profile string) (*sql.DB, func() error, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return nil, nil, err
	}
	u := url.URL{Scheme: "file", Path: path}
	pragmas := "busy_timeout(5000)&_pragma=foreign_keys(1)"
	if profile == "identity" {
		pragmas = "busy_timeout(20)"
	}
	return processReadPools.acquire(ctx, path, profile, func() (*sql.DB, error) {
		return sql.Open("sqlite", u.String()+"?mode=ro&_pragma="+pragmas)
	})
}

// The finite writer has already initialized this database. Prepare its first
// read connection before startup's identity-only deadline begins.
func primeServingReadPool(ctx context.Context, path string) {
	if _, release, err := acquireReadOnlySQL(ctx, path, "identity"); err == nil {
		_ = release()
	}
}
