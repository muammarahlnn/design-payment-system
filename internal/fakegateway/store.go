package fakegateway

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
)

// store keeps charges in memory and mirrors them to a JSON file.
//
// A real provider never forgets a charge, and reconciliation depends on that:
// the whole job is asking "what really happened to this payment?", which only
// works if the answer survives a restart. An in-memory map answered 404 after
// every rebuild, which in a real system would mean "that charge never existed"
// — a dangerous lie to tell a reconciler.
//
// A JSON file is enough here. It keeps fake-gateway free of any infrastructure
// dependency, which is the other property worth preserving.
type store struct {
	mu      sync.Mutex
	path    string
	charges map[string]Charge
}

func newStore(path string) *store {
	s := &store{
		path:    path,
		charges: make(map[string]Charge),
	}
	if path == "" {
		log.Printf("fake-gateway: no store path set; charges are memory-only and die on restart")
		return s
	}
	if err := s.load(); err != nil {
		log.Printf("fake-gateway: could not load %s, starting empty: %v", path, err)
	} else {
		log.Printf("fake-gateway: loaded %d charge(s) from %s", len(s.charges), path)
	}
	return s
}

func (s *store) load() error {
	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return nil // first run
	}
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &s.charges)
}

// flush writes the whole map. The caller already holds the lock.
//
// Write to a temp file then rename: rename is atomic on POSIX, so a crash
// mid-write leaves the previous good file rather than a truncated one.
func (s *store) flush() error {
	if s.path == "" {
		return nil
	}

	data, err := json.MarshalIndent(s.charges, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal charges: %w", err)
	}

	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("rename %s: %w", tmp, err)
	}
	return nil
}

func (s *store) get(key string) (Charge, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, found := s.charges[key]
	return c, found
}

func (s *store) put(key string, c Charge) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.charges[key] = c
	if err := s.flush(); err != nil {
		// Losing durability is bad but not worth failing the charge over —
		// the caller already has its answer.
		log.Printf("fake-gateway: could not persist charge %s: %v", key, err)
	}
}

func (s *store) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.charges)
}

// ensureDir creates the directory for path if it does not exist.
func ensureDir(path string) {
	if path == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		log.Printf("fake-gateway: could not create store dir: %v", err)
	}
}
