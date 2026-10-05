package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"PgPoc/internal/persistence"
	"PgPoc/internal/server"
	"PgPoc/internal/stats"
	"PgPoc/internal/worker"
)

func main() {
	dsn := getenv("DB_DSN", "postgres://postgres:postgres@localhost:6432/gcr")
	workers := getenvInt("WORKER_COUNT", 32)
	buffer := getenvInt("JOB_BUFFER", 1024)
	statsDir := getenv("STATS_DIR", "benchmark/results")
	callbackTimeout := time.Duration(getenvInt("CALLBACK_TIMEOUT_MS", 30000)) * time.Millisecond
	addr := getenv("LISTEN_ADDR", ":8080")

	pool := persistence.NewPostgresClient(dsn)
	defer pool.Close()

	collector := stats.NewCollector()
	dispatcher := worker.NewDispatcher(pool, collector, workers, buffer, callbackTimeout)
	dispatcher.Start()
	log.Printf("Started %d search workers (buffer=%d, callback_timeout=%s)", workers, buffer, callbackTimeout)

	router := server.NewRouter(pool, dispatcher)
	httpServer := &http.Server{Addr: addr, Handler: router}

	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalln(err)
		}
	}()
	log.Printf("Listening on %s", addr)

	// Wait for SIGINT/SIGTERM, then drain and dump stats.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	log.Println("Shutdown signal received; draining ...")

	// Stop accepting new requests (in-flight handlers finish enqueuing).
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("http shutdown: %v", err)
	}

	// Drain the queue: close the channel and wait for workers to finish.
	dispatcher.Stop()

	csvPath, jsonPath, err := collector.Dump(statsDir)
	if err != nil {
		log.Printf("stats dump failed: %v", err)
	} else {
		log.Printf("Stats written:\n  %s\n  %s", csvPath, jsonPath)
	}
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getenvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
		log.Printf("invalid int for %s=%q, using default %d", key, v, def)
	}
	return def
}
