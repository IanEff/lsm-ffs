// Package stringabase implements the basic in-memory key-value store.
package stringabase

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"sync"
)

// Store is a concurrency-safe in-memory string key-value store. It is accessed through Get(), and
// Set(). Set() will clobber existing stored values and allows for empty string values.
// The zero value is not usable: build with [NewStore].
type Store struct {
	mu       sync.RWMutex
	database map[string]string
	path     string
}

// NewStore returns the pointer to a freshly initialized Store.
func NewStore() *Store {
	return &Store{
		database: make(map[string]string),
	}
}

// Get returns a key's value from the Store. If the key isn't present, it returns an empty string and false.
func (s *Store) Get(key string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	value, ok := s.database[key]
	return value, ok
}

// Set sets the key-value pair in the database. If the key is present, its existing value is clobbered.
func (s *Store) Set(key, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	err := s.logToFile(key, value)
	if err != nil {
		return err
	}
	s.database[key] = value // clobber existing
	return nil
}

func (s *Store) logToFile(key, value string) error {
	f, err := os.OpenFile(s.path, os.O_APPEND|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	e := Entry{
		Action: "SET",
		Key:    key,
		Value:  value,
	}
	err = json.NewEncoder(f).Encode(e)
	if err != nil {
		return err
	}
	return nil
}

type Entry struct {
	Action, Key, Value string
}

func Open(path string) (*Store, error) {
	sb := NewStore()
	sb.path = path
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return sb, nil
		}
		return &Store{}, err
	}
	dec := json.NewDecoder(f)
	var e Entry
	for {
		err := dec.Decode(&e)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return &Store{}, nil
		}
		sb.database[e.Key] = e.Value
	}
	return sb, nil
}
