package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

func writeMarkdown(results []BenchResult, path string, cfg Config) {
	f, err := os.Create(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: create markdown file: %v\n", err)
		return
	}
	defer f.Close()

	fmt.Fprintf(f, "# API Response Time Benchmark Report\n\n")
	fmt.Fprintf(f, "## Test Conditions\n\n")
	fmt.Fprintf(f, "| Parameter | Value |\n|-----------|-------|\n")
	fmt.Fprintf(f, "| Target | %s |\n", cfg.BaseURL)
	fmt.Fprintf(f, "| Owner | %s |\n", cfg.Owner)
	fmt.Fprintf(f, "| Samples | %d per route |\n", cfg.Samples)
	fmt.Fprintf(f, "| Concurrency | %d |\n", cfg.Concurrency)
	fmt.Fprintf(f, "| Timeout | %v |\n", cfg.Timeout)
	fmt.Fprintf(f, "| Upload sizes | %s |\n", formatUploadSizes(cfg.UploadSizes))
	fmt.Fprintf(f, "| Big dir | %v |\n", cfg.BigDir)
	authMode := "X-Bfl-User (internal)"
	if cfg.Cookie != "" {
		authMode = "Cookie (external)"
	}
	fmt.Fprintf(f, "| Auth mode | %s |\n", authMode)
	proto := "HTTP"
	if strings.HasPrefix(cfg.BaseURL, "https://") {
		proto = "HTTPS"
	}
	fmt.Fprintf(f, "| Protocol | %s |\n", proto)
	fmt.Fprintf(f, "| Generated | %s |\n\n", time.Now().Format("2006-01-02 15:04:05"))

	grouped := groupByCategory(results)
	cats := sortedCategories(grouped)

	// summary
	fmt.Fprintf(f, "## Summary\n\n")
	var ok, errored, setup, cleanup, skipped int
	for _, r := range results {
		switch {
		case r.Note == "skip" || r.Note == "skip-dep":
			skipped++
		case r.Note == "setup":
			setup++
		case r.Note == "cleanup":
			cleanup++
		case r.Status <= 0:
			errored++
		default:
			ok++
		}
	}
	fmt.Fprintf(f, "| Metric | Count |\n|--------|-------|\n")
	fmt.Fprintf(f, "| Total routes | %d |\n", len(results))
	fmt.Fprintf(f, "| Benchmarked | %d |\n", ok)
	fmt.Fprintf(f, "| Setup | %d |\n", setup)
	fmt.Fprintf(f, "| Cleanup | %d |\n", cleanup)
	fmt.Fprintf(f, "| Skipped (unsafe) | %d |\n", skipped)
	fmt.Fprintf(f, "| Errors | %d |\n\n", errored)

	for _, cat := range cats {
		group := grouped[cat]
		fmt.Fprintf(f, "## %s\n\n", cat)
		fmt.Fprintf(f, "| Method | Pattern | Description | ReqSize | Avg | P50 | P95 | TTFB(avg) | Min | Max | Status | CurTimeout | Note |\n")
		fmt.Fprintf(f, "|--------|---------|-------------|---------|-----|-----|-----|-----------|-----|-----|--------|------------|------|\n")

		for _, r := range group {
			if r.Note == "skip" || r.Note == "skip-dep" {
				reason := r.Route.SkipReason
				if r.Note == "skip-dep" {
					reason = "prerequisite not available"
				} else if reason == "" {
					reason = "unsafe"
				}
				fmt.Fprintf(f, "| %s | `%s` | %s | - | - | - | - | - | - | - | SKIP | %s | %s |\n",
				r.Route.Method, r.Route.Pattern, r.Route.Description, r.Route.CurrentTimeout, reason)
				continue
			}

			if r.Status <= 0 && r.Note == "" {
				errMsg := "request failed"
				if len(r.Samples) > 0 && r.Samples[0].Error != "" {
					errMsg = truncate(r.Samples[0].Error, 60)
				}
				fmt.Fprintf(f, "| %s | `%s` | %s | - | - | - | - | - | - | - | ERR | %s | %s |\n",
					r.Route.Method, r.Route.Pattern, r.Route.Description, r.Route.CurrentTimeout, errMsg)
				continue
			}

			note := r.Note
			if r.Status >= 400 && note == "" {
				note = fmt.Sprintf("HTTP %d", r.Status)
			}

			reqSize := avgReqSize(r.Samples)
			avgTTFB := avgTTFBDuration(r.Samples)

			fmt.Fprintf(f, "| %s | `%s` | %s | %s | %s | %s | %s | %s | %s | %s | %d | %s | %s |\n",
				r.Route.Method, r.Route.Pattern, r.Route.Description,
				fmtBytes(reqSize),
				fmtDuration(r.Avg), fmtDuration(r.P50), fmtDuration(r.P95),
				fmtDuration(avgTTFB),
				fmtDuration(r.Min), fmtDuration(r.Max),
				r.Status, r.Route.CurrentTimeout, note)
		}
		fmt.Fprintln(f)
	}

	// collect benchmarked P95 values for estimation
	var benchedP95s []time.Duration
	for _, r := range results {
		if r.Note == "skip" || r.Note == "skip-dep" || r.Note == "setup" || r.Note == "cleanup" {
			continue
		}
		if r.Status > 0 && r.P95 > 0 {
			benchedP95s = append(benchedP95s, r.P95)
		}
	}
	sort.Slice(benchedP95s, func(i, j int) bool { return benchedP95s[i] < benchedP95s[j] })

	// timeout recommendations
	fmt.Fprintf(f, "## Timeout Recommendations\n\n")
	fmt.Fprintf(f, "Based on P95 latency with safety multiplier (3x normal, 5x stream; floor: 5s normal, 10s stream):\n\n")
	fmt.Fprintf(f, "| Category | Method | Pattern | P95 | Current Timeout | Suggested Timeout | Basis |\n")
	fmt.Fprintf(f, "|----------|--------|---------|-----|-----------------|-------------------|-------|\n")
	for _, cat := range cats {
		for _, r := range grouped[cat] {
			if r.Note == "setup" || r.Note == "cleanup" {
				continue
			}
			if r.Note == "skip" || r.Note == "skip-dep" {
				continue
			}
			if r.Status <= 0 {
				continue
			}
			suggested := suggestTimeout(r.P95, r.Route.Stream)
			fmt.Fprintf(f, "| %s | %s | `%s` | %s | %s | %s | measured |\n",
				cat, r.Route.Method, r.Route.Pattern,
				fmtDuration(r.P95), r.Route.CurrentTimeout, fmtDuration(suggested))
		}
	}

	// estimated timeouts for skipped routes
	for _, cat := range cats {
		for _, r := range grouped[cat] {
			if r.Note != "skip" && r.Note != "skip-dep" {
				continue
			}
			estimated := estimateSkippedTimeout(r, benchedP95s)
			fmt.Fprintf(f, "| %s | %s | `%s` | - | %s | %s | estimated |\n",
				cat, r.Route.Method, r.Route.Pattern, r.Route.CurrentTimeout, fmtDuration(estimated))
		}
	}
	fmt.Fprintln(f)

	// skipped routes summary table
	var skippedRoutes []BenchResult
	for _, cat := range cats {
		for _, r := range grouped[cat] {
			if r.Note == "skip" || r.Note == "skip-dep" {
				skippedRoutes = append(skippedRoutes, r)
			}
		}
	}

	if len(skippedRoutes) > 0 {
		fmt.Fprintf(f, "## Skipped Routes Summary\n\n")
		fmt.Fprintf(f, "%d routes were skipped.\n\n", len(skippedRoutes))
		fmt.Fprintf(f, "| Category | Method | Pattern | Description | CurTimeout | Suggested | Skip Reason |\n")
		fmt.Fprintf(f, "|----------|--------|---------|-------------|------------|-----------|-------------|\n")
		for _, r := range skippedRoutes {
			reason := r.Route.SkipReason
			if r.Note == "skip-dep" {
				reason = "prerequisite not available"
			}
			suggested := fmtDuration(estimateSkippedTimeout(r, benchedP95s))
			fmt.Fprintf(f, "| %s | %s | `%s` | %s | %s | %s | %s |\n",
				r.Route.Category, r.Route.Method, r.Route.Pattern,
				r.Route.Description, r.Route.CurrentTimeout, suggested, reason)
		}
		fmt.Fprintln(f)

		hasAnalysis := false
		for _, r := range skippedRoutes {
			if r.Route.Recommendation != "" {
				hasAnalysis = true
				break
			}
		}
		if hasAnalysis {
			fmt.Fprintf(f, "## Skipped Routes — Detailed Analysis & Recommendations\n\n")
			for _, r := range skippedRoutes {
				if r.Route.Recommendation == "" {
					continue
				}
				fmt.Fprintf(f, "### %s `%s` (%s)\n\n", r.Route.Method, r.Route.Pattern, r.Route.Category)
				fmt.Fprintf(f, "**Skip reason**: %s\n\n", r.Route.SkipReason)
				fmt.Fprintf(f, "**Current timeout**: %s\n\n", r.Route.CurrentTimeout)
				fmt.Fprintf(f, "**Analysis**: %s\n\n", r.Route.Recommendation)
			}
		}
	}
}

func writeCSV(results []BenchResult, path string) {
	f, err := os.Create(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: create csv file: %v\n", err)
		return
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	_ = w.Write([]string{
		"category", "method", "pattern", "test_path", "description",
		"stream", "phase", "note",
		"skip", "skip_reason", "recommendation",
		"status", "samples",
		"avg_ms", "p50_ms", "p95_ms", "min_ms", "max_ms",
		"avg_ttfb_ms", "avg_req_bytes", "avg_resp_bytes",
		"current_timeout",
		"error",
	})

	for _, r := range results {
		stream := "false"
		if r.Route.Stream {
			stream = "true"
		}

		errMsg := ""
		if len(r.Samples) > 0 && r.Samples[0].Error != "" {
			errMsg = r.Samples[0].Error
		}

		testPath := r.Route.TestPath
		if r.Route.DynPath != nil {
			if resolved := r.Route.ResolvePath(); resolved != "" {
				testPath = resolved
			}
		}

		skipStr := "false"
		if r.Route.Skip {
			skipStr = "true"
		}

		reqBytes := avgReqSize(r.Samples)
		respBytes := avgRespSize(r.Samples)
		ttfb := avgTTFBDuration(r.Samples)

		_ = w.Write([]string{
			r.Route.Category,
			r.Route.Method,
			r.Route.Pattern,
			testPath,
			r.Route.Description,
			stream,
			strconv.Itoa(r.Route.Phase),
			r.Note,
			skipStr,
			r.Route.SkipReason,
			r.Route.Recommendation,
			statusStr(r.Status),
			strconv.Itoa(len(r.Samples)),
			msStr(r.Avg),
			msStr(r.P50),
			msStr(r.P95),
			msStr(r.Min),
			msStr(r.Max),
			msStr(ttfb),
			fmt.Sprintf("%d", reqBytes),
			fmt.Sprintf("%d", respBytes),
			r.Route.CurrentTimeout,
			errMsg,
		})
	}
}

func suggestTimeout(p95 time.Duration, stream bool) time.Duration {
	multiplier := 3.0
	minTimeout := 5
	if stream {
		multiplier = 5.0
		minTimeout = 10
	}
	secs := float64(p95) * multiplier / float64(time.Second)
	rounded := (int(secs)/5 + 1) * 5 // round up to next multiple of 5s
	if rounded < minTimeout {
		rounded = minTimeout
	}
	return time.Duration(rounded) * time.Second
}

// estimateSkippedTimeout parses the "Suggest timeout: Xs" from the
// Recommendation text. If that is not present, it falls back to the
// 95th percentile of all benchmarked P95s times a 5x multiplier.
func estimateSkippedTimeout(r BenchResult, benchedP95s []time.Duration) time.Duration {
	rec := r.Route.Recommendation
	if idx := strings.Index(rec, "Suggest timeout:"); idx >= 0 {
		seg := rec[idx+len("Suggest timeout:"):]
		seg = strings.TrimSpace(seg)
		if end := strings.IndexAny(seg, " (."); end > 0 {
			seg = seg[:end]
		}
		if d, err := time.ParseDuration(seg); err == nil {
			return d
		}
	}
	if len(benchedP95s) > 0 {
		top := percentile(benchedP95s, 0.95)
		est := float64(top) * 5 / float64(time.Second)
		rounded := (int(est)/5 + 1) * 5 // round up to next multiple of 5s
		if rounded < 10 {
			rounded = 10
		}
		return time.Duration(rounded) * time.Second
	}
	return 10 * time.Second
}

func avgReqSize(samples []SampleResult) int64 {
	if len(samples) == 0 {
		return 0
	}
	var total int64
	for _, s := range samples {
		total += s.ReqSize
	}
	return total / int64(len(samples))
}

func avgRespSize(samples []SampleResult) int64 {
	if len(samples) == 0 {
		return 0
	}
	var total int64
	for _, s := range samples {
		total += s.BodySize
	}
	return total / int64(len(samples))
}

func avgTTFBDuration(samples []SampleResult) time.Duration {
	if len(samples) == 0 {
		return 0
	}
	var count int
	var total time.Duration
	for _, s := range samples {
		if s.Error == "" {
			total += s.TTFB
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return total / time.Duration(count)
}

func fmtBytes(b int64) string {
	if b == 0 {
		return "-"
	}
	if b < 1024 {
		return fmt.Sprintf("%dB", b)
	}
	if b < 1024*1024 {
		return fmt.Sprintf("%.1fKB", float64(b)/1024)
	}
	return fmt.Sprintf("%.1fMB", float64(b)/(1024*1024))
}

func formatUploadSizes(sizes []int) string {
	parts := make([]string, len(sizes))
	for i, s := range sizes {
		parts[i] = fmt.Sprintf("%dMB", s)
	}
	return strings.Join(parts, ", ")
}

func groupByCategory(results []BenchResult) map[string][]BenchResult {
	m := make(map[string][]BenchResult)
	for _, r := range results {
		m[r.Route.Category] = append(m[r.Route.Category], r)
	}
	return m
}

func sortedCategories(m map[string][]BenchResult) []string {
	order := []string{
		"health", "resources", "tree", "raw", "preview",
		"upload", "paste", "search", "share", "users",
		"repos", "permission", "md5", "external", "callback", "media",
	}
	seen := make(map[string]bool)
	var result []string
	for _, c := range order {
		if _, ok := m[c]; ok {
			result = append(result, c)
			seen[c] = true
		}
	}
	var extra []string
	for k := range m {
		if !seen[k] {
			extra = append(extra, k)
		}
	}
	sort.Strings(extra)
	return append(result, extra...)
}

func fmtDuration(d time.Duration) string {
	if d == 0 {
		return "-"
	}
	if d < time.Millisecond {
		return fmt.Sprintf("%.1fµs", float64(d)/float64(time.Microsecond))
	}
	if d < time.Second {
		return fmt.Sprintf("%.1fms", float64(d)/float64(time.Millisecond))
	}
	return fmt.Sprintf("%.2fs", d.Seconds())
}

func msStr(d time.Duration) string {
	if d == 0 {
		return ""
	}
	return fmt.Sprintf("%.2f", float64(d)/float64(time.Millisecond))
}

func statusStr(code int) string {
	if code <= 0 {
		return ""
	}
	return strconv.Itoa(code)
}

func truncate(s string, maxLen int) string {
	if maxLen < 4 || len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
