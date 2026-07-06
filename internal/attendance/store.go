package attendance

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Store holds the set of days the user was in office.
type Store struct {
	Days []string `json:"days"` // ISO dates in IST, sorted, deduplicated
	mu   sync.Mutex
	path string
}

// Load reads attendance.json. Returns empty store if file absent.
func Load() (*Store, error) {
	dir, err := appSupportDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("attendance: mkdir: %w", err)
	}
	path := filepath.Join(dir, "attendance.json")
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &Store{path: path}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("attendance: read: %w", err)
	}
	var s Store
	if err := json.Unmarshal(data, &s); err != nil {
		// corrupted file — return empty store, log error
		fmt.Fprintf(os.Stderr, "attendance: parse error (using empty store): %v\n", err)
		return &Store{path: path}, nil
	}
	s.path = path
	return &s, nil
}

// MarkToday inserts today's IST date into the store if not already present.
// Returns true if the store was modified.
func (s *Store) MarkToday(loc *time.Location) bool {
	today := time.Now().In(loc).Format("2006-01-02")
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, d := range s.Days {
		if d == today {
			return false
		}
	}
	s.Days = append(s.Days, today)
	sort.Strings(s.Days)
	return true
}

// Save atomically writes the store to disk.
func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("attendance: marshal: %w", err)
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("attendance: write tmp: %w", err)
	}
	return os.Rename(tmp, s.path)
}

// DaysThisMonth returns the count of distinct attended days in the given year/month (IST).
func (s *Store) DaysThisMonth(year int, month time.Month, loc *time.Location) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	prefix := fmt.Sprintf("%04d-%02d-", year, int(month))
	count := 0
	for _, d := range s.Days {
		if len(d) >= len(prefix) && d[:len(prefix)] == prefix {
			count++
		}
	}
	return count
}

// DaysThisWeek returns attended days in the current Mon–Sun week (IST).
func (s *Store) DaysThisWeek(loc *time.Location) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().In(loc)
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7 // Sunday → 7
	}
	monday := now.AddDate(0, 0, -(weekday - 1))
	sunday := monday.AddDate(0, 0, 6)
	mondayStr := monday.Format("2006-01-02")
	sundayStr := sunday.Format("2006-01-02")
	count := 0
	for _, d := range s.Days {
		if d >= mondayStr && d <= sundayStr {
			count++
		}
	}
	return count
}

// MarkDate inserts a specific ISO date (YYYY-MM-DD) into the store if not already present.
// Returns true if the store was modified.
func (s *Store) MarkDate(date string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, d := range s.Days {
		if d == date {
			return false
		}
	}
	s.Days = append(s.Days, date)
	sort.Strings(s.Days)
	return true
}

// IsPresentToday returns whether today (IST) is already marked.
func (s *Store) IsPresentToday(loc *time.Location) bool {
	today := time.Now().In(loc).Format("2006-01-02")
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, d := range s.Days {
		if d == today {
			return true
		}
	}
	return false
}

func appSupportDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("attendance: home dir: %w", err)
	}
	return filepath.Join(home, "Library", "Application Support", "wifi-attendance"), nil
}
