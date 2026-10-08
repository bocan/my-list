package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
)

const (
	listFileName    = "mylist.csv"
	defaultMaxBytes = 150_000
)

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	port := env("PORT", "8080")
	dataDir := env("DATA_DIR", "data")
	maxBytes, err := strconv.ParseInt(env("MAX_BYTES", strconv.Itoa(defaultMaxBytes)), 10, 64)
	if err != nil || maxBytes <= 0 {
		log.Fatalf("MAX_BYTES must be a positive integer")
	}

	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		log.Fatalf("data directory: %v", err)
	}
	path := filepath.Join(dataDir, listFileName)
	store, err := OpenStore(path, maxBytes)
	if err != nil {
		log.Fatalf("open %s: %v", path, err)
	}

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           newHandler(store),
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(shutdownCtx)
	}()

	log.Printf("on-my-list: port %s, file %s, limit %d bytes", port, path, maxBytes)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
