// Package stats collects per-query timing records and, on shutdown, writes a
// per-query CSV plus an aggregate JSON (percentile execution times per query
// type and overall) into the results directory.
package stats

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"time"
)

// Record is one processed query.
type Record struct {
	QueryID    string
	QueryType  string
	CategoryID string
	Domain     string
	DBms       float64   // DB query + scan + group time
	SendMs     float64   // callback POST time
	TotalMs    float64   // enqueue-to-callback-sent time
	Rows       int
	SendStatus int       // HTTP status of the callback POST, 0 if it never completed
	Err        string    // "" on success
	StartedAt  time.Time // when the job was enqueued (processing window start)
	FinishedAt time.Time // when the callback POST completed (processing window end)
}

// Collector is safe for concurrent use by worker goroutines.
type Collector struct {
	mu      sync.Mutex
	records []Record
	started time.Time
}

func NewCollector() *Collector {
	return &Collector{started: time.Now()}
}

func (c *Collector) Record(r Record) {
	c.mu.Lock()
	c.records = append(c.records, r)
	c.mu.Unlock()
}

// Dump writes the per-query CSV and aggregate JSON into dir (created if needed),
// timestamped. Returns the two file paths.
func (c *Collector) Dump(dir string) (csvPath string, jsonPath string, err error) {
	c.mu.Lock()
	records := make([]Record, len(c.records))
	copy(records, c.records)
	uptime := time.Since(c.started).Seconds()
	c.mu.Unlock()

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", "", err
	}
	ts := time.Now().Format("20060102_150405")
	csvPath = filepath.Join(dir, fmt.Sprintf("search_query_stats_%s.csv", ts))
	jsonPath = filepath.Join(dir, fmt.Sprintf("search_query_agg_%s.json", ts))

	if err := writeCSV(csvPath, records); err != nil {
		return "", "", err
	}
	if err := writeAggJSON(jsonPath, records, uptime, processingSpanSeconds(records)); err != nil {
		return "", "", err
	}
	return csvPath, jsonPath, nil
}

// processingSpanSeconds is the wall-clock time from the first job enqueued to the
// last callback completed — the real "time to process all queries", independent
// of how long the server then sat idle before shutdown.
func processingSpanSeconds(records []Record) float64 {
	if len(records) == 0 {
		return 0
	}
	first := records[0].StartedAt
	last := records[0].FinishedAt
	for _, r := range records[1:] {
		if !r.StartedAt.IsZero() && r.StartedAt.Before(first) {
			first = r.StartedAt
		}
		if r.FinishedAt.After(last) {
			last = r.FinishedAt
		}
	}
	if last.Before(first) {
		return 0
	}
	return last.Sub(first).Seconds()
}

func writeCSV(path string, records []Record) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	header := []string{
		"query_id", "query_type", "category_id", "domain",
		"db_ms", "send_ms", "total_ms", "rows", "send_status", "error",
	}
	if err := w.Write(header); err != nil {
		return err
	}
	for _, r := range records {
		row := []string{
			r.QueryID, r.QueryType, r.CategoryID, r.Domain,
			strconv.FormatFloat(r.DBms, 'f', 3, 64),
			strconv.FormatFloat(r.SendMs, 'f', 3, 64),
			strconv.FormatFloat(r.TotalMs, 'f', 3, 64),
			strconv.Itoa(r.Rows),
			strconv.Itoa(r.SendStatus),
			r.Err,
		}
		if err := w.Write(row); err != nil {
			return err
		}
	}
	return nil
}

// --- aggregation -----------------------------------------------------------

type numSummary struct {
	Count int     `json:"count"`
	Min   float64 `json:"min"`
	P50   float64 `json:"p50"`
	P95   float64 `json:"p95"`
	P99   float64 `json:"p99"`
	Max   float64 `json:"max"`
	Mean  float64 `json:"mean"`
}

type groupSummary struct {
	Count    int        `json:"count"`
	Errors   int        `json:"errors"`
	RowsMean float64    `json:"rows_mean"`
	DBms     numSummary `json:"db_ms"`
	SendMs   numSummary `json:"send_ms"`
	TotalMs  numSummary `json:"total_ms"`
}

type aggregate struct {
	GeneratedAt       string                  `json:"generated_at"`
	ServerUptimeSecs  float64                 `json:"server_uptime_seconds"`
	ProcessingSeconds float64                 `json:"processing_seconds"`
	Throughput        float64                 `json:"throughput_qps"`
	Overall           groupSummary            `json:"overall"`
	ByType            map[string]groupSummary `json:"by_type"`
}

func writeAggJSON(path string, records []Record, uptime, processingSecs float64) error {
	agg := aggregate{
		GeneratedAt:       time.Now().Format(time.RFC3339),
		ServerUptimeSecs:  uptime,
		ProcessingSeconds: processingSecs,
		Overall:           summarize(records),
		ByType:            map[string]groupSummary{},
	}
	// Throughput is over the real processing window, not server uptime.
	if processingSecs > 0 {
		agg.Throughput = float64(len(records)) / processingSecs
	}

	byType := map[string][]Record{}
	for _, r := range records {
		byType[r.QueryType] = append(byType[r.QueryType], r)
	}
	for qtype, recs := range byType {
		agg.ByType[qtype] = summarize(recs)
	}

	data, err := json.MarshalIndent(agg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func summarize(records []Record) groupSummary {
	g := groupSummary{Count: len(records)}
	if len(records) == 0 {
		return g
	}
	db := make([]float64, 0, len(records))
	send := make([]float64, 0, len(records))
	total := make([]float64, 0, len(records))
	rowsSum := 0
	for _, r := range records {
		if r.Err != "" {
			g.Errors++
		}
		db = append(db, r.DBms)
		send = append(send, r.SendMs)
		total = append(total, r.TotalMs)
		rowsSum += r.Rows
	}
	g.RowsMean = float64(rowsSum) / float64(len(records))
	g.DBms = summarizeFloats(db)
	g.SendMs = summarizeFloats(send)
	g.TotalMs = summarizeFloats(total)
	return g
}

func summarizeFloats(values []float64) numSummary {
	n := len(values)
	if n == 0 {
		return numSummary{}
	}
	sort.Float64s(values)
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return numSummary{
		Count: n,
		Min:   values[0],
		P50:   percentile(values, 50),
		P95:   percentile(values, 95),
		P99:   percentile(values, 99),
		Max:   values[n-1],
		Mean:  sum / float64(n),
	}
}

// percentile uses linear interpolation on an already-sorted slice.
func percentile(sorted []float64, pct float64) float64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	if n == 1 {
		return sorted[0]
	}
	rank := pct / 100.0 * float64(n-1)
	lo := int(rank)
	hi := lo + 1
	if hi >= n {
		return sorted[n-1]
	}
	frac := rank - float64(lo)
	return sorted[lo] + (sorted[hi]-sorted[lo])*frac
}
