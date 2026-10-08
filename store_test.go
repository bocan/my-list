package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func tempStore(t *testing.T, maxBytes int64) (*Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), listFileName)
	s, err := OpenStore(path, maxBytes)
	if err != nil {
		t.Fatal(err)
	}
	return s, path
}

func texts(st State) string {
	out := make([]string, len(st.Items))
	for i, it := range st.Items {
		out[i] = it.Text
	}
	return strings.Join(out, "|")
}

func snap(t *testing.T, s *Store) State {
	t.Helper()
	st, err := s.State()
	if err != nil {
		t.Fatalf("State: %v", err)
	}
	return st
}

func mustAdd(t *testing.T, s *Store, text string) State {
	t.Helper()
	st, err := s.Add(text)
	if err != nil {
		t.Fatalf("Add(%q): %v", text, err)
	}
	return st
}

func TestFileMirrorsTheList(t *testing.T) {
	s, path := tempStore(t, defaultMaxBytes)
	mustAdd(t, s, "first")
	mustAdd(t, s, `second, with "quotes"`)
	st := mustAdd(t, s, "  line one\nline two  ")

	want := `line one line two|second, with "quotes"|first`
	if got := texts(st); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}

	data, _ := os.ReadFile(path)
	wantFile := "line one line two\n\"second, with \"\"quotes\"\"\"\nfirst\n"
	if string(data) != wantFile {
		t.Fatalf("file:\n%s\nwant:\n%s", data, wantFile)
	}

	reloaded, err := OpenStore(path, defaultMaxBytes)
	if err != nil {
		t.Fatal(err)
	}
	rst := snap(t, reloaded)
	if got := texts(rst); got != want {
		t.Fatalf("reloaded: got %q, want %q", got, want)
	}
	if rst.Bytes != int64(len(data)) {
		t.Fatalf("Bytes = %d, file size = %d", rst.Bytes, len(data))
	}
}

func TestIsCategory(t *testing.T) {
	cases := map[string]bool{
		"> Work":         true,
		"> > Projects":   true,
		">> Projects":    true,
		"> > > Deep one": true,
		"Work":           false,
		">Work":          false,
		">":              false,
		"a > b":          false,
	}
	for text, want := range cases {
		if got := isCategory(text); got != want {
			t.Errorf("isCategory(%q) = %v, want %v", text, got, want)
		}
	}
}

func TestNewCategoryGoesToTheBottom(t *testing.T) {
	s, path := tempStore(t, defaultMaxBytes)
	mustAdd(t, s, "a")
	mustAdd(t, s, "> Work")
	mustAdd(t, s, "b")
	st := mustAdd(t, s, "> > Projects")

	want := "b|a|> Work|> > Projects"
	if got := texts(st); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	data, _ := os.ReadFile(path)
	if wantFile := "b\na\n> Work\n> > Projects\n"; string(data) != wantFile {
		t.Fatalf("file:\n%s\nwant:\n%s", data, wantFile)
	}
}

func TestEmptyTextIsRefused(t *testing.T) {
	s, _ := tempStore(t, defaultMaxBytes)
	if _, err := s.Add(" \n "); !errors.Is(err, ErrEmpty) {
		t.Fatalf("Add: got %v, want ErrEmpty", err)
	}
	st := mustAdd(t, s, "a")
	if _, err := s.Update(st.Items[0].ID, ""); !errors.Is(err, ErrEmpty) {
		t.Fatalf("Update: got %v, want ErrEmpty", err)
	}
}

func TestWriteAboveLimitIsRefused(t *testing.T) {
	s, path := tempStore(t, 100)
	st := mustAdd(t, s, "small")
	before, _ := os.ReadFile(path)

	if _, err := s.Add(strings.Repeat("x", 200)); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("Add: got %v, want ErrTooLarge", err)
	}
	if _, err := s.Update(st.Items[0].ID, strings.Repeat("x", 200)); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("Update: got %v, want ErrTooLarge", err)
	}

	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatalf("file changed after a refused write")
	}
	if got := texts(snap(t, s)); got != "small" {
		t.Fatalf("memory changed after a refused write: %q", got)
	}
}

func TestLimitIsExact(t *testing.T) {
	// "12345678\n" is 9 bytes, "x\n" is 2 bytes.
	s, path := tempStore(t, 11)
	mustAdd(t, s, "12345678")
	mustAdd(t, s, "x")
	if _, err := s.Add("y"); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("Add: got %v, want ErrTooLarge", err)
	}
	if info, _ := os.Stat(path); info.Size() != 11 {
		t.Fatalf("file size = %d, want 11", info.Size())
	}
}

func TestOversizedFileCanShrinkButNotGrow(t *testing.T) {
	path := filepath.Join(t.TempDir(), listFileName)
	content := strings.Repeat("a", 100) + "\n" + strings.Repeat("b", 100) + "\n"
	os.WriteFile(path, []byte(content), 0o644)

	s, err := OpenStore(path, 50)
	if err != nil {
		t.Fatal(err)
	}
	st := snap(t, s)
	if _, err := s.Add("c"); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("Add: got %v, want ErrTooLarge", err)
	}
	if _, err := s.Reorder([]string{st.Items[1].ID, st.Items[0].ID}); err != nil {
		t.Fatalf("Reorder: %v", err)
	}
	if _, err := s.Delete(st.Items[0].ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
}

func TestUpdateDeleteReorder(t *testing.T) {
	s, _ := tempStore(t, defaultMaxBytes)
	mustAdd(t, s, "c")
	mustAdd(t, s, "b")
	st := mustAdd(t, s, "a")
	a, b, c := st.Items[0].ID, st.Items[1].ID, st.Items[2].ID

	st, err := s.Reorder([]string{c, a, b})
	if err != nil || texts(st) != "c|a|b" {
		t.Fatalf("Reorder: %q, %v", texts(st), err)
	}
	st, err = s.Update(a, "A")
	if err != nil || texts(st) != "c|A|b" {
		t.Fatalf("Update: %q, %v", texts(st), err)
	}
	st, err = s.Delete(c)
	if err != nil || texts(st) != "A|b" {
		t.Fatalf("Delete: %q, %v", texts(st), err)
	}

	if _, err := s.Update("nope", "x"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Update: got %v, want ErrNotFound", err)
	}
	if _, err := s.Delete("nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Delete: got %v, want ErrNotFound", err)
	}
	for _, ids := range [][]string{{a}, {a, a}, {a, "nope"}, {a, b, c}} {
		if _, err := s.Reorder(ids); !errors.Is(err, ErrOrderMismatch) {
			t.Fatalf("Reorder(%v): got %v, want ErrOrderMismatch", ids, err)
		}
	}
}

func TestIdenticalTextsAreSeparateEntries(t *testing.T) {
	s, _ := tempStore(t, defaultMaxBytes)
	mustAdd(t, s, "same")
	st := mustAdd(t, s, "same")
	st, err := s.Update(st.Items[1].ID, "changed")
	if err != nil || texts(st) != "same|changed" {
		t.Fatalf("Update: %q, %v", texts(st), err)
	}
}

func TestHandEditedFileIsPickedUp(t *testing.T) {
	s, path := tempStore(t, defaultMaxBytes)
	mustAdd(t, s, "from the app")

	edited := "plain line\nmilk, eggs, bread\n\n\"quoted, text\"\nsay \"hi\" now\n"
	os.WriteFile(path, []byte(edited), 0o644)

	want := `plain line|milk, eggs, bread|quoted, text|say "hi" now`
	st := snap(t, s)
	if got := texts(st); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}

	os.Remove(path)
	if st := snap(t, s); len(st.Items) != 0 || st.Bytes != 0 {
		t.Fatalf("after file removal: %+v", st)
	}
}
