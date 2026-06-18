package core

import "sync"

// RelayListStore is the pluggable persistence seam for the relay directory's
// per-user resolutions (a user's outbox / inbox / DM relays). The default is
// in-memory; a consumer can plug a shared or persistent store — e.g. a database
// — so resolutions survive restarts or are shared across instances.
//
// The RelayDirectory owns the TTL and single-flight logic and serializes access,
// so an implementation only needs to be a correct key-value map of
// pubkey -> *UserRelays; it does not need its own freshness or de-duplication
// logic. (Because the directory holds its lock across store calls, a store whose
// Get/Set hit slow I/O will serialize lookups — a DB-backed store that cares
// should keep a fast in-memory layer in front.)
type RelayListStore interface {
	// Get returns the stored resolution for a pubkey and whether one exists.
	Get(pubkey string) (*UserRelays, bool)
	// Set stores (or replaces) the resolution for a pubkey.
	Set(pubkey string, ur *UserRelays)
	// Delete removes any stored resolution for a pubkey.
	Delete(pubkey string)
	// Range calls fn for each stored entry until fn returns false. Used to build
	// the union "known relays" set; iteration order is unspecified.
	Range(fn func(pubkey string, ur *UserRelays) bool)
}

// memRelayListStore is the default in-memory RelayListStore: a mutex-guarded map.
type memRelayListStore struct {
	mu      sync.Mutex
	entries map[string]*UserRelays
}

func newMemRelayListStore() *memRelayListStore {
	return &memRelayListStore{entries: make(map[string]*UserRelays)}
}

func (s *memRelayListStore) Get(pubkey string) (*UserRelays, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ur, ok := s.entries[pubkey]
	return ur, ok
}

func (s *memRelayListStore) Set(pubkey string, ur *UserRelays) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries[pubkey] = ur
}

func (s *memRelayListStore) Delete(pubkey string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.entries, pubkey)
}

func (s *memRelayListStore) Range(fn func(string, *UserRelays) bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, v := range s.entries {
		if !fn(k, v) {
			return
		}
	}
}
