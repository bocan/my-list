package main

import (
	"bytes"
	"crypto/rand"
	"encoding/csv"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

var (
	ErrTooLarge      = errors.New("the list file would exceed the size limit")
	ErrNotFound      = errors.New("entry not found")
	ErrEmpty         = errors.New("entry text is empty")
	ErrOrderMismatch = errors.New("the order does not match the current entries")
)

// Item is one entry. The ID exists only in memory; the file holds text only.
type Item struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

type State struct {
	Items    []Item `json:"items"`
	Bytes    int64  `json:"bytes"`
	MaxBytes int64  `json:"maxBytes"`
}

// Store keeps the list in a CSV file that mirrors the UI: one row for each
// entry, one column with the text, rows in display order, no header.
type Store struct {
	mu       sync.Mutex
	path     string
	maxBytes int64
	items    []Item
	size     int64
	modTime  time.Time
}

var newlines = strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ")

// An entry is one line of text.
func normalize(text string) string {
	return strings.TrimSpace(newlines.Replace(text))
}

// A category is an entry that starts with one or more ">" marks, then a name.
// The number of marks is the depth: "> Work", "> > Projects".
var categoryPattern = regexp.MustCompile(`^>( ?>)* +\S`)

func isCategory(text string) bool {
	return categoryPattern.MatchString(text)
}

func newID() string {
	b := make([]byte, 6)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// OpenStore loads the CSV file at path. A missing file is an empty list.
func OpenStore(path string, maxBytes int64) (*Store, error) {
	s := &Store{path: path, maxBytes: maxBytes}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) load() error {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, fs.ErrNotExist) {
		s.items, s.size, s.modTime = nil, 0, time.Time{}
		return nil
	}
	if err != nil {
		return err
	}
	info, err := os.Stat(s.path)
	if err != nil {
		return err
	}
	r := csv.NewReader(bytes.NewReader(data))
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	rows, err := r.ReadAll()
	if err != nil {
		return err
	}
	var items []Item
	for _, row := range rows {
		// A hand-written line with unquoted commas is still one entry.
		if text := normalize(strings.Join(row, ",")); text != "" {
			items = append(items, Item{ID: newID(), Text: text})
		}
	}
	s.items, s.size, s.modTime = items, int64(len(data)), info.ModTime()
	return nil
}

// refresh loads the file again if something other than this server changed it.
func (s *Store) refresh() error {
	info, err := os.Stat(s.path)
	if errors.Is(err, fs.ErrNotExist) {
		if s.modTime.IsZero() {
			return nil
		}
		return s.load()
	}
	if err != nil {
		return err
	}
	if info.Size() == s.size && info.ModTime().Equal(s.modTime) {
		return nil
	}
	return s.load()
}

func encode(items []Item) ([]byte, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	for _, it := range items {
		w.Write([]string{it.Text})
	}
	w.Flush()
	return buf.Bytes(), w.Error()
}

// commit writes items to disk, then makes them the current list. It refuses
// a write that goes above the limit, unless the write makes the file smaller.
func (s *Store) commit(items []Item) error {
	data, err := encode(items)
	if err != nil {
		return err
	}
	size := int64(len(data))
	if size > s.maxBytes && size > s.size {
		return ErrTooLarge
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".mylist-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
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
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), s.path); err != nil {
		return err
	}
	s.items, s.size = items, size
	if info, err := os.Stat(s.path); err == nil {
		s.modTime = info.ModTime()
	}
	return nil
}

func (s *Store) state() State {
	return State{Items: append([]Item{}, s.items...), Bytes: s.size, MaxBytes: s.maxBytes}
}

func (s *Store) State() (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	err := s.refresh()
	return s.state(), err
}

// Add puts a new entry at the top of the list. A new category goes to the
// bottom, because a category at the top would take the entries below it.
func (s *Store) Add(text string) (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.refresh(); err != nil {
		return s.state(), err
	}
	text = normalize(text)
	if text == "" {
		return s.state(), ErrEmpty
	}
	item := Item{ID: newID(), Text: text}
	var items []Item
	if isCategory(text) {
		items = append(append(items, s.items...), item)
	} else {
		items = append(append(items, item), s.items...)
	}
	err := s.commit(items)
	return s.state(), err
}

func (s *Store) Update(id, text string) (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.refresh(); err != nil {
		return s.state(), err
	}
	text = normalize(text)
	if text == "" {
		return s.state(), ErrEmpty
	}
	items := append([]Item{}, s.items...)
	for i := range items {
		if items[i].ID == id {
			items[i].Text = text
			err := s.commit(items)
			return s.state(), err
		}
	}
	return s.state(), ErrNotFound
}

func (s *Store) Delete(id string) (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.refresh(); err != nil {
		return s.state(), err
	}
	for i, it := range s.items {
		if it.ID == id {
			items := append(append([]Item{}, s.items[:i]...), s.items[i+1:]...)
			err := s.commit(items)
			return s.state(), err
		}
	}
	return s.state(), ErrNotFound
}

// Reorder sets the list order. ids must contain each current ID one time.
func (s *Store) Reorder(ids []string) (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.refresh(); err != nil {
		return s.state(), err
	}
	if len(ids) != len(s.items) {
		return s.state(), ErrOrderMismatch
	}
	byID := make(map[string]Item, len(s.items))
	for _, it := range s.items {
		byID[it.ID] = it
	}
	items := make([]Item, 0, len(ids))
	for _, id := range ids {
		it, ok := byID[id]
		if !ok {
			return s.state(), ErrOrderMismatch
		}
		delete(byID, id)
		items = append(items, it)
	}
	err := s.commit(items)
	return s.state(), err
}
