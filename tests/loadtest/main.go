package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

const (
	baseURL         = "http://127.0.0.1:18090"
	numWorkers      = 50
	testDuration    = 10 * time.Second
	numIDs          = 500
	numChannels     = 5
	numFingerprints = 100
)

var channels = []string{"default", "mobile", "desktop", "tablet", "api"}

var httpClient = &http.Client{
	Timeout: 5 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:        200,
		MaxIdleConnsPerHost: 200,
		IdleConnTimeout:     30 * time.Second,
		DialContext: (&net.Dialer{
			Timeout:   2 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
	},
}

type result struct {
	endpoint string
	status   int
	latency  time.Duration
	err      bool
}

type stats struct {
	count     int64
	errors    int64
	latencies []time.Duration
}

func main() {
	fmt.Println("=== SSD Load Test ===")
	fmt.Printf("Workers: %d | Duration: %s\n", numWorkers, testDuration)
	fmt.Printf("IDs: %d | Channels: %d | Fingerprints: %d\n\n", numIDs, numChannels, numFingerprints)

	// Wait for server
	fmt.Print("Waiting for server... ")
	for i := 0; i < 30; i++ {
		resp, err := httpClient.Get(baseURL + "/channels")
		if err == nil {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			break
		}
		if i == 29 {
			fmt.Println("FAILED: server not responding")
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	fmt.Println("OK")

	// Phase 1: Seed data with POST requests
	fmt.Println("\n--- Phase 1: Seeding data (POST /) ---")
	runPhase(testDuration, func(rng *rand.Rand) result {
		return doPost(rng)
	})

	// Wait for aggregation
	fmt.Println("\nWaiting 2s for aggregation...")
	time.Sleep(2 * time.Second)

	// Phase 2: Mixed read/write load
	fmt.Println("\n--- Phase 2: Mixed load (70% POST, 30% GET) ---")
	runPhase(testDuration, func(rng *rand.Rand) result {
		r := rng.Float64()
		switch {
		case r < 0.70:
			return doPost(rng)
		case r < 0.80:
			return doGetList(rng)
		case r < 0.87:
			return doGetFingerprints(rng)
		case r < 0.94:
			return doGetFingerprint(rng)
		default:
			return doGetChannels()
		}
	})

	// Phase 3: Read-heavy load
	fmt.Println("\n--- Phase 3: Read-heavy load (10% POST, 90% GET) ---")
	runPhase(testDuration, func(rng *rand.Rand) result {
		r := rng.Float64()
		switch {
		case r < 0.10:
			return doPost(rng)
		case r < 0.40:
			return doGetList(rng)
		case r < 0.60:
			return doGetFingerprints(rng)
		case r < 0.80:
			return doGetFingerprint(rng)
		default:
			return doGetChannels()
		}
	})
}

func runPhase(duration time.Duration, workFn func(rng *rand.Rand) result) {
	results := make(chan result, 10000)
	var wg sync.WaitGroup
	var totalOps atomic.Int64
	stop := make(chan struct{})

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(seed int64) {
			defer wg.Done()
			rng := rand.New(rand.NewSource(seed))
			for {
				select {
				case <-stop:
					return
				default:
					r := workFn(rng)
					totalOps.Add(1)
					results <- r
				}
			}
		}(rand.Int63() + int64(i))
	}

	allResults := make(map[string]*stats)
	done := make(chan struct{})
	go func() {
		for r := range results {
			s, ok := allResults[r.endpoint]
			if !ok {
				s = &stats{}
				allResults[r.endpoint] = s
			}
			s.count++
			if r.err {
				s.errors++
			}
			s.latencies = append(s.latencies, r.latency)
		}
		close(done)
	}()

	time.Sleep(duration)
	close(stop)
	wg.Wait()
	close(results)
	<-done

	printResults(allResults, duration)
}

func printResults(allResults map[string]*stats, duration time.Duration) {
	var totalOps int64
	var totalErrors int64

	endpoints := make([]string, 0, len(allResults))
	for ep := range allResults {
		endpoints = append(endpoints, ep)
	}
	sort.Strings(endpoints)

	fmt.Printf("\n  %-22s %8s %6s %10s %10s %10s %10s\n",
		"Endpoint", "Reqs", "Errs", "Avg", "P50", "P95", "P99")
	fmt.Println("  " + repeat("-", 88))

	for _, ep := range endpoints {
		s := allResults[ep]
		totalOps += s.count
		totalErrors += s.errors

		sort.Slice(s.latencies, func(i, j int) bool {
			return s.latencies[i] < s.latencies[j]
		})

		avg := avgDuration(s.latencies)
		p50 := percentile(s.latencies, 0.50)
		p95 := percentile(s.latencies, 0.95)
		p99 := percentile(s.latencies, 0.99)

		fmt.Printf("  %-22s %8d %6d %10s %10s %10s %10s\n",
			ep, s.count, s.errors, fmtDur(avg), fmtDur(p50), fmtDur(p95), fmtDur(p99))
	}

	rps := float64(totalOps) / duration.Seconds()
	fmt.Println("  " + repeat("-", 88))
	fmt.Printf("  Total: %d reqs | Errors: %d (%.1f%%) | RPS: %.0f\n",
		totalOps, totalErrors, float64(totalErrors)/float64(totalOps)*100, rps)
}

func doPost(rng *rand.Rand) result {
	nViews := rng.Intn(5) + 1
	nClicks := rng.Intn(3)
	views := make([]string, nViews)
	clicks := make([]string, nClicks)
	for i := range views {
		views[i] = fmt.Sprintf("%d", rng.Intn(numIDs)+1)
	}
	for i := range clicks {
		clicks[i] = fmt.Sprintf("%d", rng.Intn(numIDs)+1)
	}

	body := map[string]interface{}{
		"v": views,
		"c": clicks,
		"f": fmt.Sprintf("fp_%d", rng.Intn(numFingerprints)),
	}
	if rng.Float64() < 0.6 {
		body["ch"] = channels[rng.Intn(len(channels))]
	}

	data, _ := json.Marshal(body)
	start := time.Now()
	resp, err := httpClient.Post(baseURL+"/", "application/json", bytes.NewReader(data))
	lat := time.Since(start)
	if err != nil {
		return result{"POST /", 0, lat, true}
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	return result{"POST /", resp.StatusCode, lat, resp.StatusCode != 201}
}

func doGetList(rng *rand.Rand) result {
	ch := channels[rng.Intn(len(channels))]
	url := fmt.Sprintf("%s/list?ch=%s", baseURL, ch)
	start := time.Now()
	resp, err := httpClient.Get(url)
	lat := time.Since(start)
	if err != nil {
		return result{"GET /list", 0, lat, true}
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	return result{"GET /list", resp.StatusCode, lat, resp.StatusCode != 200}
}

func doGetFingerprints(rng *rand.Rand) result {
	ch := channels[rng.Intn(len(channels))]
	url := fmt.Sprintf("%s/fingerprints?ch=%s", baseURL, ch)
	start := time.Now()
	resp, err := httpClient.Get(url)
	lat := time.Since(start)
	if err != nil {
		return result{"GET /fingerprints", 0, lat, true}
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	return result{"GET /fingerprints", resp.StatusCode, lat, resp.StatusCode != 200}
}

func doGetFingerprint(rng *rand.Rand) result {
	ch := channels[rng.Intn(len(channels))]
	fp := fmt.Sprintf("fp_%d", rng.Intn(numFingerprints))
	url := fmt.Sprintf("%s/fingerprint?ch=%s&f=%s", baseURL, ch, fp)
	start := time.Now()
	resp, err := httpClient.Get(url)
	lat := time.Since(start)
	if err != nil {
		return result{"GET /fingerprint", 0, lat, true}
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	return result{"GET /fingerprint", resp.StatusCode, lat, resp.StatusCode != 200}
}

func doGetChannels() result {
	start := time.Now()
	resp, err := httpClient.Get(baseURL + "/channels")
	lat := time.Since(start)
	if err != nil {
		return result{"GET /channels", 0, lat, true}
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	return result{"GET /channels", resp.StatusCode, lat, resp.StatusCode != 200}
}

func avgDuration(d []time.Duration) time.Duration {
	if len(d) == 0 {
		return 0
	}
	var sum time.Duration
	for _, v := range d {
		sum += v
	}
	return sum / time.Duration(len(d))
}

func percentile(d []time.Duration, p float64) time.Duration {
	if len(d) == 0 {
		return 0
	}
	idx := int(float64(len(d)) * p)
	if idx >= len(d) {
		idx = len(d) - 1
	}
	return d[idx]
}

func fmtDur(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%dµs", d.Microseconds())
	}
	return fmt.Sprintf("%.1fms", float64(d.Microseconds())/1000.0)
}

func repeat(s string, n int) string {
	out := ""
	for i := 0; i < n; i++ {
		out += s
	}
	return out
}
