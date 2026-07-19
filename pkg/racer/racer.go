package racer

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/D3wier/race-the-web/pkg/config"
)

type Response struct {
	Index      int           `json:"index"`
	StatusCode int           `json:"status_code"`
	Body       string        `json:"body"`
	Headers    http.Header   `json:"headers"`
	Duration   time.Duration `json:"duration_ms"`
	Timestamp  time.Time     `json:"timestamp"`
	Error      string        `json:"error,omitempty"`
}

type RoundResult struct {
	Round       int        `json:"round"`
	Responses   []Response `json:"responses"`
	TimingSpread time.Duration `json:"timing_spread_ms"`
}

type Racer struct {
	config  *config.Config
	verbose bool
	client  *http.Client
}

func New(cfg *config.Config, verbose bool) *Racer {
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 100,
		IdleConnTimeout:     30 * time.Second,
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
		DisableKeepAlives:   false,
	}

	if cfg.HTTP2 {
		transport.ForceAttemptHTTP2 = true
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   cfg.Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	return &Racer{config: cfg, verbose: verbose, client: client}
}

func (r *Racer) Run() []RoundResult {
	var allResults []RoundResult

	r.warmConnections()

	for round := 1; round <= r.config.Rounds; round++ {
		if r.config.Rounds > 1 {
			fmt.Printf("─── Round %d ───────────────────────────────────────────\n", round)
		}

		result := r.runRound(round)
		allResults = append(allResults, result)
		r.printRoundResult(result)

		if round < r.config.Rounds {
			time.Sleep(r.config.Delay)
		}
	}

	r.printSummary(allResults)
	return allResults
}

func (r *Racer) warmConnections() {
	if r.verbose {
		fmt.Println("[*] Warming connections...")
	}
	for _, reqCfg := range r.config.Requests {
		req, _ := http.NewRequest("HEAD", reqCfg.URL, nil)
		resp, err := r.client.Do(req)
		if err == nil {
			resp.Body.Close()
		}
	}
}

func (r *Racer) runRound(round int) RoundResult {
	reqCfg := r.config.Requests[0]
	count := reqCfg.Count

	responses := make([]Response, count)
	var wg sync.WaitGroup
	barrier := make(chan struct{})

	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			var bodyReader io.Reader
			if reqCfg.Body != "" {
				bodyReader = strings.NewReader(reqCfg.Body)
			}

			req, err := http.NewRequest(reqCfg.Method, reqCfg.URL, bodyReader)
			if err != nil {
				responses[idx] = Response{Index: idx, Error: err.Error()}
				return
			}

			for k, v := range reqCfg.Headers {
				req.Header.Set(k, v)
			}
			if req.Header.Get("User-Agent") == "" {
				req.Header.Set("User-Agent", "Mozilla/5.0 (race-the-web)")
			}

			<-barrier

			start := time.Now()
			resp, err := r.client.Do(req)
			duration := time.Since(start)

			if err != nil {
				responses[idx] = Response{Index: idx, Duration: duration, Error: err.Error(), Timestamp: start}
				return
			}
			defer resp.Body.Close()

			body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))

			responses[idx] = Response{
				Index:      idx,
				StatusCode: resp.StatusCode,
				Body:       string(body),
				Headers:    resp.Header,
				Duration:   duration,
				Timestamp:  start,
			}
		}(i)
	}

	time.Sleep(50 * time.Millisecond)
	close(barrier)
	wg.Wait()

	var timestamps []time.Time
	for _, resp := range responses {
		if !resp.Timestamp.IsZero() {
			timestamps = append(timestamps, resp.Timestamp)
		}
	}

	var spread time.Duration
	if len(timestamps) > 1 {
		sort.Slice(timestamps, func(i, j int) bool { return timestamps[i].Before(timestamps[j]) })
		spread = timestamps[len(timestamps)-1].Sub(timestamps[0])
	}

	return RoundResult{
		Round:        round,
		Responses:    responses,
		TimingSpread: spread,
	}
}

func (r *Racer) printRoundResult(result RoundResult) {
	fmt.Printf("  Timing spread: %v (first → last request)\n\n", result.TimingSpread)

	statusCounts := make(map[int]int)
	bodyCounts := make(map[string]int)
	errors := 0

	for _, resp := range result.Responses {
		if resp.Error != "" {
			errors++
			continue
		}
		statusCounts[resp.StatusCode]++
		body := truncate(resp.Body, 80)
		bodyCounts[body]++
	}

	fmt.Println("  Status codes:")
	for code, count := range statusCounts {
		marker := ""
		if count > 1 && (code == 200 || code == 201) {
			marker = " ← RACE?"
		}
		fmt.Printf("    %d .......... %d%s\n", code, count, marker)
	}

	if errors > 0 {
		fmt.Printf("    errors ...... %d\n", errors)
	}

	if len(bodyCounts) > 1 {
		fmt.Println("\n  Response bodies (unique):")
		for body, count := range bodyCounts {
			fmt.Printf("    %q ... %d\n", body, count)
		}
	}
	fmt.Println()
}

func (r *Racer) printSummary(results []RoundResult) {
	fmt.Println("─── Summary ───────────────────────────────────────────")

	totalSuccess := 0
	totalRequests := 0
	for _, round := range results {
		for _, resp := range round.Responses {
			totalRequests++
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				totalSuccess++
			}
		}
	}

	perRound := totalRequests / len(results)
	if totalSuccess > 1 && totalSuccess > len(results) {
		fmt.Printf("  ⚠ POTENTIAL RACE CONDITION DETECTED\n")
		fmt.Printf("  Expected: ~1 success per round | Got: %d successes across %d requests\n",
			totalSuccess, totalRequests)
		fmt.Printf("  The server processed %d requests successfully instead of 1.\n", totalSuccess)
	} else if totalSuccess <= len(results) {
		fmt.Printf("  ✓ No obvious race condition detected (%d/%d requests succeeded)\n",
			totalSuccess, totalRequests)
		fmt.Printf("  Server appears to handle concurrent requests correctly.\n")
	}

	_ = perRound
}

func SaveResults(results []RoundResult, path string) {
	data, _ := json.MarshalIndent(results, "", "  ")
	os.WriteFile(path, data, 0644)
}

func truncate(s string, max int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.TrimSpace(s)
	if len(s) > max {
		return s[:max] + "..."
	}
	return s
}
