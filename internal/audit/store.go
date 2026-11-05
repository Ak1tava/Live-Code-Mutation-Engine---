package audit

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Store keeps sessions in memory and mirrors each one to <dir>/<id>.json.
type Store struct {
	dir string
	mu  sync.Mutex
	m   map[string]*Session
}

// OpenStore creates dir if needed and loads every <id>.json in it. Other
// files, including leftover temp files, are ignored.
func OpenStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	st := &Store{dir: dir, m: map[string]*Session{}}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		var s Session
		if err := json.Unmarshal(b, &s); err != nil {
			return nil, fmt.Errorf("audit: load %s: %w", e.Name(), err)
		}
		if s.ID+".json" != e.Name() {
			return nil, fmt.Errorf("audit: load %s: id mismatch", e.Name())
		}
		if s.Plan != nil {
			s.Plan.Mutated = s.Mutated // Plan.Mutated is json:"-"
		}
		st.m[s.ID] = &s
	}
	return st, nil
}

func validID(id string) bool {
	return id != "" && id != "." && id != ".." && !strings.ContainsAny(id, `/\`) && filepath.Base(id) == id
}

// write saves s atomically: temp file in the same directory, then rename.
func (st *Store) write(s *Session) error {
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(st.dir, ".tmp-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	fail := func(err error) error {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Chmod(0o600); err != nil {
		return fail(err)
	}
	if _, err := tmp.Write(b); err != nil {
		return fail(err)
	}
	if err := tmp.Sync(); err != nil {
		return fail(err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Rename(name, filepath.Join(st.dir, s.ID+".json")); err != nil {
		os.Remove(name)
		return err
	}
	return nil
}

// clone returns a deep copy of s.
func (s *Session) clone() *Session {
	c := *s
	c.Hints = append(s.Hints[:0:0], s.Hints...)
	c.Files = copyMap(s.Files)
	c.Tests = copyMap(s.Tests)
	c.Mutated = copyMap(s.Mutated)
	if s.Plan != nil {
		p := *s.Plan
		p.Mutations = append(s.Plan.Mutations[:0:0], s.Plan.Mutations...)
		p.BaselineTests = append(s.Plan.BaselineTests[:0:0], s.Plan.BaselineTests...)
		if s.Plan.KillingTests != nil {
			p.KillingTests = make([][]string, len(s.Plan.KillingTests))
			for i, k := range s.Plan.KillingTests {
				p.KillingTests[i] = append([]string(nil), k...)
			}
		}
		p.Mutated = c.Mutated
		c.Plan = &p
	}
	if s.Flags != nil {
		c.Flags = make([]Flag, len(s.Flags))
		for i, f := range s.Flags {
			if f.Accepted != nil {
				a := *f.Accepted
				f.Accepted = &a
			}
			c.Flags[i] = f
		}
	}
	c.Runs = append(s.Runs[:0:0], s.Runs...)
	c.Events = append(s.Events[:0:0], s.Events...)
	return &c
}

// Create registers a new session and writes it to disk.
func (st *Store) Create(s *Session) error {
	if !validID(s.ID) {
		return errors.New("audit: invalid session id")
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	if _, dup := st.m[s.ID]; dup {
		return fmt.Errorf("audit: session %s already exists", s.ID)
	}
	c := s.clone()
	if err := st.write(c); err != nil {
		return err
	}
	st.m[c.ID] = c
	return nil
}

// Update runs fn on a copy of the session under the lock and persists the
// result. If fn or the write fails, the stored session is left unchanged.
func (st *Store) Update(id string, fn func(*Session) error) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	cur, ok := st.m[id]
	if !ok {
		return os.ErrNotExist
	}
	c := cur.clone()
	if err := fn(c); err != nil {
		return err
	}
	c.ID = cur.ID
	if err := st.write(c); err != nil {
		return err
	}
	*cur = *c
	return nil
}

// View runs fn on the session under the lock; fn must not modify or keep it.
func (st *Store) View(id string, fn func(*Session)) {
	st.mu.Lock()
	defer st.mu.Unlock()
	if s, ok := st.m[id]; ok {
		fn(s)
	}
}

// ByToken finds the session and role for an auditor or student token.
func (st *Store) ByToken(tok string) (id string, role Role, ok bool) {
	if tok == "" {
		return "", "", false
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	for _, s := range st.m {
		if s.AuditorToken == tok {
			return s.ID, RoleAuditor, true
		}
		if s.StudentToken == tok {
			return s.ID, RoleStudent, true
		}
	}
	return "", "", false
}

// IDs lists all session IDs.
func (st *Store) IDs() []string {
	st.mu.Lock()
	defer st.mu.Unlock()
	ids := make([]string, 0, len(st.m))
	for id := range st.m {
		ids = append(ids, id)
	}
	return ids
}
