package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"sort"
	"strconv"
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
	fmt.Fprintf(f, "- **Target**: %s\n", cfg.BaseURL)
	fmt.Fprintf(f, "- **Owner**: %s\n", cfg.Owner)
	fmt.Fprintf(f, "- **Samples**: %d per route\n", cfg.Samples)
	fmt.Fprintf(f, "- **Timeout**: %v\n", cfg.Timeout)
	fmt.Fprintf(f, "- **Generated**: %s\n\n", time.Now().Format("2006-01-02 15:04:05"))

	grouped := groupByCategory(results)
	cats := sortedCategories(grouped)

	// summary
	fmt.Fprintf(f, "## Summary\n\n")
	var ok, errored, setup, cleanup int
	for _, r := range results {
		switch {
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
	fmt.Fprintf(f, "| Errors | %d |\n\n", errored)

	for _, cat := range cats {
		group := grouped[cat]
		fmt.Fprintf(f, "## %s\n\n", cat)
		fmt.Fprintf(f, "| Method | Pattern | Description | Avg | P50 | P95 | Min | Max | Status | Stream | Note |\n")
		fmt.Fprintf(f, "|--------|---------|-------------|-----|-----|-----|-----|-----|--------|--------|------|\n")

		for _, r := range group {
			stream := ""
			if r.Route.Stream {
				stream = "stream"
			}

			if r.Status <= 0 && r.Note == "" {
				errMsg := "request failed"
				if len(r.Samples) > 0 && r.Samples[0].Error != "" {
					errMsg = truncate(r.Samples[0].Error, 60)
				}
				fmt.Fprintf(f, "| %s | `%s` | %s | - | - | - | - | - | ERR | %s | %s |\n",
					r.Route.Method, r.Route.Pattern, r.Route.Description, stream, errMsg)
				continue
			}

			note := r.Note
			if r.Status >= 400 && note == "" {
				note = fmt.Sprintf("HTTP %d", r.Status)
			}

			fmt.Fprintf(f, "| %s | `%s` | %s | %s | %s | %s | %s | %s | %d | %s | %s |\n",
				r.Route.Method, r.Route.Pattern, r.Route.Description,
				fmtDuration(r.Avg), fmtDuration(r.P50), fmtDuration(r.P95),
				fmtDuration(r.Min), fmtDuration(r.Max),
				r.Status, stream, note)
		}
		fmt.Fprintln(f)
	}

	// timeout recommendations
	fmt.Fprintf(f, "## Timeout Recommendations\n\n")
	fmt.Fprintf(f, "Based on P95 latency with safety multiplier (3x normal, 5x stream):\n\n")
	fmt.Fprintf(f, "| Category | Method | Pattern | P95 | Suggested Timeout |\n")
	fmt.Fprintf(f, "|----------|--------|---------|-----|-------------------|\n")
	for _, cat := range cats {
		for _, r := range grouped[cat] {
			if r.Status <= 0 || r.Note == "setup" || r.Note == "cleanup" {
				continue
			}
			multiplier := 3.0
			if r.Route.Stream {
				multiplier = 5.0
			}
			suggested := time.Duration(float64(r.P95) * multiplier)
			if suggested < time.Second {
				suggested = time.Second
			}
			suggested = suggested.Round(time.Second)
			if suggested == 0 {
				suggested = time.Second
			}
			fmt.Fprintf(f, "| %s | %s | `%s` | %s | %s |\n",
				cat, r.Route.Method, r.Route.Pattern,
				fmtDuration(r.P95), fmtDuration(suggested))
		}
	}
	fmt.Fprintln(f)
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
		"status", "samples",
		"avg_ms", "p50_ms", "p95_ms", "min_ms", "max_ms",
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
			testPath = r.Route.ResolvePath()
		}

		_ = w.Write([]string{
			r.Route.Category,
			r.Route.Method,
			r.Route.Pattern,
			testPath,
			r.Route.Description,
			stream,
			strconv.Itoa(r.Route.Phase),
			r.Note,
			statusStr(r.Status),
			strconv.Itoa(len(r.Samples)),
			msStr(r.Avg),
			msStr(r.P50),
			msStr(r.P95),
			msStr(r.Min),
			msStr(r.Max),
			errMsg,
		})
	}
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
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
