package workstation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

var ErrRevisionConflict = errors.New("saved state revision conflict")

type Store struct {
	path string
	mu   sync.Mutex
}

func NewStore(path string) *Store { return &Store{path: path} }

func (s *Store) Path() string { return s.path }

func (s *Store) Ensure() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := os.Stat(s.path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	state := DefaultState()
	return s.writeLocked(state)
}

func (s *Store) Load() (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadLocked()
}

func (s *Store) Save(next State) (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, err := s.loadLocked()
	if err != nil {
		return State{}, err
	}
	if next.Revision != current.Revision {
		return State{}, ErrRevisionConflict
	}
	next.Authority = "LOCAL"
	next.SchemaVersion = SchemaVersion
	next.Revision++
	if err := next.Validate(); err != nil {
		return State{}, err
	}
	if err := s.writeLocked(next); err != nil {
		return State{}, err
	}
	return next, nil
}

func (s *Store) Hash() (string, error) {
	state, err := s.Load()
	if err != nil {
		return "", err
	}
	b, err := json.Marshal(state)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func (s *Store) loadLocked() (State, error) {
	b, err := os.ReadFile(s.path)
	if err != nil {
		return State{}, err
	}
	var state State
	if err := json.Unmarshal(b, &state); err != nil {
		return State{}, fmt.Errorf("decode state: %w", err)
	}
	if err := state.Validate(); err != nil {
		return State{}, fmt.Errorf("validate state: %w", err)
	}
	return state, nil
}

func (s *Store) writeLocked(state State) error {
	if err := state.Validate(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".state-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, s.path)
}
