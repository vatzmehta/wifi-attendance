// Package daysoff persists the user's holidays and leaves.
package daysoff

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/vatzmehta/wifi-attendance/internal/policy"
)

// Entry is one marked date.
type Entry struct {
	Date string
	Kind policy.OffKind
}

// Store holds the dates the user has marked as holiday or leave.
type Store struct {
	Holidays []string `json:"holidays"` // ISO dates in IST, sorted, deduplicated
	Leaves   []string `json:"leaves"`   // ISO dates in IST, sorted, deduplicated
	mu       sync.Mutex
	path     string
}

// Load reads daysoff.json. Returns empty store if file absent.
func Load() (*Store, error) {
	dir, err := appSupportDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("daysoff: mkdir: %w", err)
	}
	path := filepath.Join(dir, "daysoff.json")
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &Store{path: path}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("daysoff: read: %w", err)
	}
	var s Store
	if err := json.Unmarshal(data, &s); err != nil {
		// corrupted file — return empty store, log error
		fmt.Fprintf(os.Stderr, "daysoff: parse error (using empty store): %v\n", err)
		return &Store{path: path}, nil
	}
	s.path = path
	return &s, nil
}

// Save atomically writes the store to disk.
func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("daysoff: marshal: %w", err)
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("daysoff: write tmp: %w", err)
	}
	return os.Rename(tmp, s.path)
}

// Mark records date as kind, replacing a mark of the other kind if present.
// Returns true if the store was modified.
func (s *Store) Mark(kind policy.OffKind, date string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	modified := false
	switch kind {
	case policy.Holiday:
		if contains(s.Leaves, date) {
			s.Leaves = remove(s.Leaves, date)
			modified = true
		}
		if !contains(s.Holidays, date) {
			s.Holidays = append(s.Holidays, date)
			sort.Strings(s.Holidays)
			modified = true
		}
	case policy.Leave:
		if contains(s.Holidays, date) {
			s.Holidays = remove(s.Holidays, date)
			modified = true
		}
		if !contains(s.Leaves, date) {
			s.Leaves = append(s.Leaves, date)
			sort.Strings(s.Leaves)
			modified = true
		}
	}
	return modified
}

// Unmark removes date from both lists. Returns true if the store was modified.
func (s *Store) Unmark(date string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	modified := false
	if contains(s.Holidays, date) {
		s.Holidays = remove(s.Holidays, date)
		modified = true
	}
	if contains(s.Leaves, date) {
		s.Leaves = remove(s.Leaves, date)
		modified = true
	}
	return modified
}

// KindOf returns the kind of date, or "" if it is not marked.
func (s *Store) KindOf(date string) policy.OffKind {
	s.mu.Lock()
	defer s.mu.Unlock()

	if contains(s.Holidays, date) {
		return policy.Holiday
	}
	if contains(s.Leaves, date) {
		return policy.Leave
	}
	return ""
}

// Between returns entries with from <= Date <= to (ISO strings), sorted by date.
func (s *Store) Between(from, to string) []Entry {
	s.mu.Lock()
	defer s.mu.Unlock()

	var entries []Entry
	for _, d := range s.Holidays {
		if d >= from && d <= to {
			entries = append(entries, Entry{Date: d, Kind: policy.Holiday})
		}
	}
	for _, d := range s.Leaves {
		if d >= from && d <= to {
			entries = append(entries, Entry{Date: d, Kind: policy.Leave})
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Date < entries[j].Date
	})
	return entries
}

// OffDays returns a snapshot map of every marked date for policy.Calculate.
func (s *Store) OffDays() policy.OffDays {
	s.mu.Lock()
	defer s.mu.Unlock()

	off := make(policy.OffDays, len(s.Holidays)+len(s.Leaves))
	for _, d := range s.Holidays {
		off[d] = policy.Holiday
	}
	for _, d := range s.Leaves {
		off[d] = policy.Leave
	}
	return off
}

func contains(list []string, date string) bool {
	for _, d := range list {
		if d == date {
			return true
		}
	}
	return false
}

func remove(list []string, date string) []string {
	out := list[:0]
	for _, d := range list {
		if d != date {
			out = append(out, d)
		}
	}
	return out
}

func appSupportDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("daysoff: home dir: %w", err)
	}
	return filepath.Join(home, "Library", "Application Support", "wifi-attendance"), nil
}
