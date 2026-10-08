package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func call(t *testing.T, h http.Handler, method, url, body string) (int, State) {
	t.Helper()
	req := httptest.NewRequest(method, url, strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var st State
	json.Unmarshal(rec.Body.Bytes(), &st)
	return rec.Code, st
}

func TestAPIFlow(t *testing.T) {
	s, _ := tempStore(t, 200)
	h := newHandler(s)

	code, st := call(t, h, "GET", "/api/items", "")
	if code != 200 || st.Items == nil || len(st.Items) != 0 || st.MaxBytes != 200 {
		t.Fatalf("empty list: %d %+v", code, st)
	}

	call(t, h, "POST", "/api/items", `{"text":"one"}`)
	code, st = call(t, h, "POST", "/api/items", `{"text":"two"}`)
	if code != 200 || texts(st) != "two|one" {
		t.Fatalf("add: %d %q", code, texts(st))
	}
	two, one := st.Items[0].ID, st.Items[1].ID

	code, st = call(t, h, "PUT", "/api/order", `{"ids":["`+one+`","`+two+`"]}`)
	if code != 200 || texts(st) != "one|two" {
		t.Fatalf("order: %d %q", code, texts(st))
	}
	code, st = call(t, h, "PUT", "/api/items/"+one, `{"text":"uno"}`)
	if code != 200 || texts(st) != "uno|two" {
		t.Fatalf("edit: %d %q", code, texts(st))
	}
	code, st = call(t, h, "DELETE", "/api/items/"+two, "")
	if code != 200 || texts(st) != "uno" {
		t.Fatalf("delete: %d %q", code, texts(st))
	}
}

func TestAPIErrorStatus(t *testing.T) {
	s, _ := tempStore(t, 100)
	h := newHandler(s)
	_, st := call(t, h, "POST", "/api/items", `{"text":"one"}`)
	id := st.Items[0].ID

	cases := []struct {
		method, url, body string
		want              int
	}{
		{"POST", "/api/items", `{"text":"` + strings.Repeat("x", 200) + `"}`, http.StatusRequestEntityTooLarge},
		{"POST", "/api/items", `{"text":"  "}`, http.StatusBadRequest},
		{"POST", "/api/items", `not json`, http.StatusBadRequest},
		{"PUT", "/api/items/nope", `{"text":"x"}`, http.StatusNotFound},
		{"DELETE", "/api/items/nope", ``, http.StatusNotFound},
		{"PUT", "/api/order", `{"ids":["` + id + `","nope"]}`, http.StatusConflict},
	}
	for _, c := range cases {
		if code, _ := call(t, h, c.method, c.url, c.body); code != c.want {
			t.Errorf("%s %s: got %d, want %d", c.method, c.url, code, c.want)
		}
	}
	if got := texts(snap(t, s)); got != "one" {
		t.Fatalf("list changed after refused requests: %q", got)
	}
}

func TestServesIndexPage(t *testing.T) {
	s, _ := tempStore(t, 100)
	rec := httptest.NewRecorder()
	newHandler(s).ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "<title>") {
		t.Fatalf("index: %d", rec.Code)
	}
}
