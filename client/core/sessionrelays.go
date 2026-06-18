package core

import (
	"sort"
	"sync"
)

// SessionRelays is a user's editable, role-tagged relay configuration for one
// session — the set a downstream app reads and edits to inspect or steer
// routing (design §4.3 / §11, "expose the pool + role assignments so callers
// can read/edit them"). Each relay URL carries a [Role] bitmask. Safe for
// concurrent use.
//
// It is a plain, in-memory container: a consumer seeds it from a user's
// resolved event-derived roles and layers local overrides on top. Wiring edits
// back into live routing is incremental — today the one override that affects
// routing is the fixed-relay pin, exposed via [UserContext.PinFixedRelays].
type SessionRelays struct {
	mu    sync.RWMutex
	roles map[string]Role
}

func newSessionRelays() *SessionRelays {
	return &SessionRelays{roles: make(map[string]Role)}
}

// Set replaces the roles assigned to url. Passing the zero Role removes it.
func (s *SessionRelays) Set(url string, roles Role) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if roles == 0 {
		delete(s.roles, url)
		return
	}
	s.roles[url] = roles
}

// Add OR-s roles onto url, keeping any it already holds.
func (s *SessionRelays) Add(url string, roles Role) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.roles[url] |= roles
}

// Remove drops url and all of its roles.
func (s *SessionRelays) Remove(url string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.roles, url)
}

// Get returns the roles assigned to url (the zero Role if absent).
func (s *SessionRelays) Get(url string) Role {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.roles[url]
}

// ByRole returns the relay URLs carrying the given role, sorted for stability.
func (s *SessionRelays) ByRole(role Role) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []string
	for url, r := range s.roles {
		if r&role != 0 {
			out = append(out, url)
		}
	}
	sort.Strings(out)
	return out
}

// All returns a snapshot copy of the full url→roles map.
func (s *SessionRelays) All() map[string]Role {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]Role, len(s.roles))
	for url, r := range s.roles {
		out[url] = r
	}
	return out
}
