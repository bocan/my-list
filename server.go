package main

import (
	"embed"
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"net/http"
)

//go:embed static
var staticFiles embed.FS

const maxRequestBytes = 1 << 20

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeResult(w http.ResponseWriter, st State, err error) {
	if err == nil {
		writeJSON(w, http.StatusOK, st)
		return
	}
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, ErrTooLarge):
		status = http.StatusRequestEntityTooLarge
	case errors.Is(err, ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, ErrEmpty):
		status = http.StatusBadRequest
	case errors.Is(err, ErrOrderMismatch):
		status = http.StatusConflict
	default:
		log.Printf("store error: %v", err)
	}
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func readJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBytes)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return false
	}
	return true
}

func newHandler(store *Store) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/items", func(w http.ResponseWriter, r *http.Request) {
		st, err := store.State()
		writeResult(w, st, err)
	})
	mux.HandleFunc("POST /api/items", func(w http.ResponseWriter, r *http.Request) {
		var body struct{ Text string }
		if !readJSON(w, r, &body) {
			return
		}
		st, err := store.Add(body.Text)
		writeResult(w, st, err)
	})
	mux.HandleFunc("PUT /api/items/{id}", func(w http.ResponseWriter, r *http.Request) {
		var body struct{ Text string }
		if !readJSON(w, r, &body) {
			return
		}
		st, err := store.Update(r.PathValue("id"), body.Text)
		writeResult(w, st, err)
	})
	mux.HandleFunc("DELETE /api/items/{id}", func(w http.ResponseWriter, r *http.Request) {
		st, err := store.Delete(r.PathValue("id"))
		writeResult(w, st, err)
	})
	mux.HandleFunc("PUT /api/order", func(w http.ResponseWriter, r *http.Request) {
		var body struct{ IDs []string }
		if !readJSON(w, r, &body) {
			return
		}
		st, err := store.Reorder(body.IDs)
		writeResult(w, st, err)
	})

	static, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic(err)
	}
	mux.Handle("GET /", http.FileServerFS(static))
	return mux
}
