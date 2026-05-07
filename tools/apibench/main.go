package main

import (
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
)

type SampleResult struct {
	StatusCode int
	Duration   time.Duration
	TTFB       time.Duration
	BodySize   int64
	Error      string
}

type BenchResult struct {
	Route   RouteCase
	Samples []SampleResult
	Min     time.Duration
	Max     time.Duration
	Avg     time.Duration
	P50     time.Duration
	P95     time.Duration
	Status  int
	Note    string
}

type Config struct {
	BaseURL   string
	Owner     string
	Samples   int
	Timeout   time.Duration
	OutputDir string
	Category  string
	Verbose   bool
}

func main() {
	cfg := Config{}
	flag.StringVar(&cfg.BaseURL, "base-url", "http://localhost:8080", "target service base URL")
	flag.StringVar(&cfg.Owner, "owner", "", "test user (X-Bfl-User header)")
	flag.IntVar(&cfg.Samples, "samples", 5, "samples per route")
	flag.DurationVar(&cfg.Timeout, "timeout", 5*time.Minute, "per-request timeout")
	flag.StringVar(&cfg.OutputDir, "output", "./results", "output directory")
	flag.StringVar(&cfg.Category, "category", "", "only run routes matching this category")
	flag.BoolVar(&cfg.Verbose, "verbose", false, "print each sample result")
	flag.Parse()

	if cfg.Owner == "" {
		fmt.Fprintln(os.Stderr, "ERROR: --owner is required (e.g. --owner admin)")
		os.Exit(1)
	}

	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")

	if err := os.MkdirAll(cfg.OutputDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: cannot create output dir: %v\n", err)
		os.Exit(1)
	}

	routes := AllRoutes()
	if cfg.Category != "" {
		var filtered []RouteCase
		for _, r := range routes {
			if r.Category == cfg.Category {
				filtered = append(filtered, r)
			}
		}
		routes = filtered
	}

	client := &http.Client{
		Timeout: cfg.Timeout,
		Transport: &http.Transport{
			TLSClientConfig:       &tls.Config{InsecureSkipVerify: true},
			MaxIdleConnsPerHost:   1,
			DisableKeepAlives:     true,
			ResponseHeaderTimeout: cfg.Timeout,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// Group routes by phase: negative = setup, 0 = normal, positive = teardown
	type phaseGroup struct {
		phase  int
		routes []RouteCase
	}
	phaseMap := make(map[int][]RouteCase)
	for _, r := range routes {
		phaseMap[r.Phase] = append(phaseMap[r.Phase], r)
	}
	var phases []int
	for p := range phaseMap {
		phases = append(phases, p)
	}
	sort.Ints(phases)

	total := len(routes)
	seq := 0
	var results []BenchResult

	for _, phase := range phases {
		group := phaseMap[phase]

		if phase < 0 {
			fmt.Printf("\n=== SETUP (phase %d) ===\n", phase)
		} else if phase > 0 && phase < 99 {
			fmt.Printf("\n=== LATE (phase %d) ===\n", phase)
		} else if phase >= 99 {
			fmt.Printf("\n=== CLEANUP (phase %d) ===\n", phase)
		}

		for _, route := range group {
			seq++
			fmt.Printf("[%d/%d] %s %s — %s", seq, total, route.Method, route.Pattern, route.Description)

			if route.Skip {
				fmt.Printf("  [SKIP: %s]\n", route.SkipReason)
				results = append(results, BenchResult{
					Route:  route,
					Status: -1,
					Note:   "skip",
				})
				continue
			}

			if route.DynPath != nil && route.DynPath() == "" {
				fmt.Println("  [SKIP: prerequisite not available]")
				results = append(results, BenchResult{
					Route:  route,
					Status: -1,
					Note:   "skip-dep",
				})
				continue
			}

			if phase < 0 {
				// Setup phase: single request, capture state, don't benchmark
				fmt.Println(" [SETUP]")
				sr := doRequest(client, cfg, route)
				captureSetupState(route, sr)
				if sr.Error != "" {
					fmt.Printf("    → SETUP ERROR: %s\n", sr.Error)
				} else {
					fmt.Printf("    → status=%d duration=%v\n", sr.StatusCode, sr.Duration.Round(time.Millisecond))
				}
				results = append(results, BenchResult{
					Route:  route,
					Status: sr.StatusCode,
					Avg:    sr.Duration,
					Min:    sr.Duration,
					Max:    sr.Duration,
					P50:    sr.Duration,
					P95:    sr.Duration,
					Note:   "setup",
					Samples: []SampleResult{sr},
				})
				continue
			}

			fmt.Println()
			br := benchmark(client, cfg, route)
			results = append(results, br)

			if phase >= 99 {
				br.Note = "cleanup"
			}

			if br.Status > 0 {
				fmt.Printf("    → avg=%v  p50=%v  p95=%v  min=%v  max=%v  status=%d\n",
					br.Avg.Round(time.Millisecond), br.P50.Round(time.Millisecond),
					br.P95.Round(time.Millisecond), br.Min.Round(time.Millisecond),
					br.Max.Round(time.Millisecond), br.Status)
			} else {
				errMsg := "no successful samples"
				if len(br.Samples) > 0 && br.Samples[0].Error != "" {
					errMsg = br.Samples[0].Error
				}
				fmt.Printf("    → ERROR: %s\n", errMsg)
			}
		}
	}

	ts := time.Now().Format("20060102_150405")
	mdPath := fmt.Sprintf("%s/api_benchmark_%s.md", cfg.OutputDir, ts)
	csvPath := fmt.Sprintf("%s/api_benchmark_%s.csv", cfg.OutputDir, ts)

	writeMarkdown(results, mdPath, cfg)
	writeCSV(results, csvPath)

	fmt.Printf("\nDone. %d routes benchmarked. Results written to:\n  %s\n  %s\n", total, mdPath, csvPath)
}

func captureSetupState(route RouteCase, sr SampleResult) {
	if sr.Error != "" || sr.StatusCode < 200 || sr.StatusCode >= 300 {
		return
	}

	// Try to extract IDs from response bodies for dynamic routes.
	// The exact field names depend on the API; we try common patterns.
	switch {
	case route.Pattern == "/api/share/share_path/*path" && route.Method == "POST":
		// share create response may contain path_id
		var resp map[string]interface{}
		// We stored the body in sr — but we don't have it.
		// The share list can be queried later. For now, we'll
		// extract from the next list call.
		_ = resp
	}
}

func benchmark(client *http.Client, cfg Config, route RouteCase) BenchResult {
	br := BenchResult{Route: route}

	// warmup
	_ = doRequest(client, cfg, route)

	for i := 0; i < cfg.Samples; i++ {
		sr := doRequest(client, cfg, route)
		br.Samples = append(br.Samples, sr)
		if cfg.Verbose {
			fmt.Printf("    sample %d: status=%d duration=%v ttfb=%v body=%d err=%q\n",
				i+1, sr.StatusCode, sr.Duration, sr.TTFB, sr.BodySize, sr.Error)
		}
	}

	if len(br.Samples) == 0 {
		return br
	}

	durations := make([]time.Duration, 0, len(br.Samples))
	for _, s := range br.Samples {
		if s.Error == "" {
			durations = append(durations, s.Duration)
		}
	}

	if len(durations) == 0 {
		br.Status = br.Samples[0].StatusCode
		return br
	}

	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	br.Min = durations[0]
	br.Max = durations[len(durations)-1]
	br.P50 = percentile(durations, 0.50)
	br.P95 = percentile(durations, 0.95)

	var sum time.Duration
	for _, d := range durations {
		sum += d
	}
	br.Avg = sum / time.Duration(len(durations))
	br.Status = br.Samples[0].StatusCode

	return br
}

func doRequest(client *http.Client, cfg Config, route RouteCase) SampleResult {
	path := route.ResolvePath()
	url := cfg.BaseURL + path

	body := route.ResolveBody()

	req, err := http.NewRequest(route.Method, url, body)
	if err != nil {
		return SampleResult{Error: fmt.Sprintf("build request: %v", err)}
	}

	req.Header.Set("X-Bfl-User", cfg.Owner)
	for k, v := range route.Headers {
		req.Header.Set(k, v)
	}

	start := time.Now()
	resp, err := client.Do(req)
	ttfb := time.Since(start)
	if err != nil {
		return SampleResult{Duration: ttfb, TTFB: ttfb, Error: fmt.Sprintf("do request: %v", err)}
	}

	respBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	total := time.Since(start)

	sr := SampleResult{
		StatusCode: resp.StatusCode,
		Duration:   total,
		TTFB:       ttfb,
		BodySize:   int64(len(respBody)),
	}

	// Try to capture dynamic IDs from responses
	captureResponseIDs(route, resp.StatusCode, respBody)

	return sr
}

func captureResponseIDs(route RouteCase, status int, body []byte) {
	if status < 200 || status >= 300 {
		return
	}

	switch {
	case route.Method == "POST" && strings.Contains(route.Pattern, "/api/share/share_path"):
		var resp map[string]interface{}
		if json.Unmarshal(body, &resp) == nil {
			if id, ok := resp["path_id"]; ok {
				createdSharePath = fmt.Sprintf("%v", id)
				fmt.Printf("    [captured share path_id: %s]\n", createdSharePath)
			}
			if data, ok := resp["data"]; ok {
				if dm, ok2 := data.(map[string]interface{}); ok2 {
					if id, ok3 := dm["path_id"]; ok3 {
						createdSharePath = fmt.Sprintf("%v", id)
						fmt.Printf("    [captured share path_id: %s]\n", createdSharePath)
					}
					if id, ok3 := dm["id"]; ok3 && createdSharePath == "" {
						createdSharePath = fmt.Sprintf("%v", id)
						fmt.Printf("    [captured share id: %s]\n", createdSharePath)
					}
				}
			}
		}

	case route.Method == "POST" && strings.Contains(route.Pattern, "/api/share/share_token"):
		var resp map[string]interface{}
		if json.Unmarshal(body, &resp) == nil {
			for _, key := range []string{"token", "data"} {
				if v, ok := resp[key]; ok {
					if s, ok2 := v.(string); ok2 && s != "" {
						createdTokenID = s
						fmt.Printf("    [captured token: %s]\n", createdTokenID)
						return
					}
					if dm, ok2 := v.(map[string]interface{}); ok2 {
						if t, ok3 := dm["token"]; ok3 {
							createdTokenID = fmt.Sprintf("%v", t)
							fmt.Printf("    [captured token: %s]\n", createdTokenID)
							return
						}
					}
				}
			}
		}

	case route.Method == "POST" && route.Pattern == "/api/repos":
		var resp map[string]interface{}
		if json.Unmarshal(body, &resp) == nil {
			for _, key := range []string{"repo_id", "repoId", "id", "data"} {
				if v, ok := resp[key]; ok {
					if s, ok2 := v.(string); ok2 && s != "" {
						createdRepoID = s
						fmt.Printf("    [captured repo id: %s]\n", createdRepoID)
						return
					}
					if dm, ok2 := v.(map[string]interface{}); ok2 {
						for _, rk := range []string{"repo_id", "repoId", "id"} {
							if rv, ok3 := dm[rk]; ok3 {
								createdRepoID = fmt.Sprintf("%v", rv)
								fmt.Printf("    [captured repo id: %s]\n", createdRepoID)
								return
							}
						}
					}
				}
			}
		}
	}
}

func percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(float64(len(sorted)-1) * p)
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}
