// query, rebuild the payload, and POST it back to the job's recipient URL.
package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"PgPoc/internal/model"
	"PgPoc/internal/persistence"
	"PgPoc/internal/stats"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Job is a queued query plus the moment it was enqueued (for total latency).
type Job struct {
	Query      model.SearchQuery
	EnqueuedAt time.Time
}

// Dispatcher owns the jobs channel and the worker pool.
type Dispatcher struct {
	jobs      chan Job
	pool      *pgxpool.Pool
	collector *stats.Collector
	client    *http.Client
	wg        sync.WaitGroup
	workers   int
}

func NewDispatcher(pool *pgxpool.Pool, collector *stats.Collector, workers, buffer int, callbackTimeout time.Duration) *Dispatcher {
	return &Dispatcher{
		jobs:      make(chan Job, buffer),
		pool:      pool,
		collector: collector,
		client:    &http.Client{Timeout: callbackTimeout},
		workers:   workers,
	}
}

// Start launches the worker goroutines.
func (d *Dispatcher) Start() {
	for i := 0; i < d.workers; i++ {
		d.wg.Add(1)
		go d.worker()
	}
}

// Enqueue pushes a job onto the channel (blocks if the buffer is full, applying
// backpressure to the HTTP handler).
func (d *Dispatcher) Enqueue(q model.SearchQuery) {
	d.jobs <- Job{Query: q, EnqueuedAt: time.Now()}
}

// Stop closes the channel and waits for in-flight jobs to drain.
func (d *Dispatcher) Stop() {
	close(d.jobs)
	d.wg.Wait()
}

func (d *Dispatcher) worker() {
	defer d.wg.Done()
	for job := range d.jobs {
		d.process(job)
	}
}

func (d *Dispatcher) process(job Job) {
	q := job.Query
	rec := stats.Record{
		QueryID:    q.QueryID,
		QueryType:  q.QueryType,
		CategoryID: q.CategoryID,
		Domain:     q.Domain,
		StartedAt:  job.EnqueuedAt,
	}

	// 1. Run the DB query and rebuild the payload.
	dbStart := time.Now()
	payloads, rows, err := persistence.RunSearch(context.Background(), d.pool, q)
	rec.DBms = msSince(dbStart)
	if err != nil {
		rec.Err = "db: " + err.Error()
		rec.TotalMs = msSince(job.EnqueuedAt)
		rec.FinishedAt = time.Now()
		d.collector.Record(rec)
		return
	}
	rec.Rows = rows

	// 2. POST the rebuilt payload back to the recipient.
	sendStart := time.Now()
	status, sendErr := d.sendCallback(q.Recipient, q.QueryID, payloads)
	rec.SendMs = msSince(sendStart)
	rec.SendStatus = status
	if sendErr != nil {
		rec.Err = "callback: " + sendErr.Error()
	}
	rec.TotalMs = msSince(job.EnqueuedAt)
	rec.FinishedAt = time.Now()
	d.collector.Record(rec)
}

func (d *Dispatcher) sendCallback(recipient, queryID string, payloads []model.RebuiltPayload) (int, error) {
	if recipient == "" {
		return 0, fmt.Errorf("empty recipient")
	}
	body, err := json.Marshal(payloads)
	if err != nil {
		return 0, fmt.Errorf("marshal payload: %w", err)
	}
	req, err := http.NewRequest(http.MethodPost, recipient, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Query-Id", queryID) // correlation without polluting the payload

	resp, err := d.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.StatusCode != http.StatusOK {
		return resp.StatusCode, fmt.Errorf("callback returned HTTP %d", resp.StatusCode)
	}
	return resp.StatusCode, nil
}

func msSince(t time.Time) float64 {
	return float64(time.Since(t).Microseconds()) / 1000.0
}
