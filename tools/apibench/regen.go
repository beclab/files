package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

func regenFromCSV(csvPath, docxPath string) {
	f, err := os.Open(csvPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: open csv: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	reader := csv.NewReader(f)
	records, err := reader.ReadAll()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: read csv: %v\n", err)
		os.Exit(1)
	}

	if len(records) < 2 {
		fmt.Fprintln(os.Stderr, "ERROR: csv has no data rows")
		os.Exit(1)
	}

	header := records[0]
	colIdx := make(map[string]int)
	for i, h := range header {
		colIdx[h] = i
	}

	col := func(row []string, name string) string {
		if idx, ok := colIdx[name]; ok && idx < len(row) {
			return row[idx]
		}
		return ""
	}

	parseMs := func(s string) time.Duration {
		if s == "" {
			return 0
		}
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0
		}
		return time.Duration(v * float64(time.Millisecond))
	}

	var results []BenchResult
	for _, row := range records[1:] {
		stream := col(row, "流式") == "是"
		skip := col(row, "跳过") == "是"

		status := 0
		if s := col(row, "状态码"); s != "" {
			status, _ = strconv.Atoi(s)
		}

		note := col(row, "备注")

		avgD := parseMs(col(row, "平均(ms)"))
		p50D := parseMs(col(row, "P50(ms)"))
		p95D := parseMs(col(row, "P95(ms)"))
		minD := parseMs(col(row, "最小(ms)"))
		maxD := parseMs(col(row, "最大(ms)"))
		ttfb := parseMs(col(row, "首字节均值(ms)"))

		reqBytes, _ := strconv.ParseInt(col(row, "请求大小(字节)"), 10, 64)

		var samples []SampleResult
		sampleCount, _ := strconv.Atoi(col(row, "采样数"))
		if sampleCount == 0 {
			sampleCount = 1
		}
		for i := 0; i < sampleCount; i++ {
			samples = append(samples, SampleResult{
				StatusCode: status,
				Duration:   avgD,
				TTFB:       ttfb,
				ReqSize:    reqBytes,
				Error:      col(row, "错误"),
			})
		}

		// Reverse translate Chinese category back to English key
		catZh := col(row, "分类")
		catEn := reverseCatZh(catZh)

		br := BenchResult{
			Route: RouteCase{
				Method:         col(row, "方法"),
				Pattern:        col(row, "路径"),
				Description:    col(row, "说明"),
				Category:       catEn,
				Stream:         stream,
				Skip:           skip,
				SkipReason:     col(row, "跳过原因"),
				Recommendation: col(row, "分析建议"),
				CurrentTimeout: col(row, "当前超时"),
			},
			Samples: samples,
			Status:  status,
			Avg:     avgD,
			P50:     p50D,
			P95:     p95D,
			Min:     minD,
			Max:     maxD,
			Note:    reverseNoteZh(note),
		}

		if skip && br.Note == "" {
			br.Note = "skip"
			br.Status = -1
		}

		results = append(results, br)
	}

	cfg := inferConfig(results)
	writeDocxEnhanced(results, docxPath, cfg)
	fmt.Printf("Enhanced docx generated: %s\n", docxPath)
}

func reverseCatZh(zh string) string {
	for en, z := range categoryZh {
		if z == zh {
			return en
		}
	}
	return zh
}

func reverseNoteZh(zh string) string {
	switch zh {
	case "前置":
		return "setup"
	case "清理":
		return "cleanup"
	case "跳过":
		return "skip"
	case "依赖跳过":
		return "skip-dep"
	}
	if strings.HasPrefix(zh, "HTTP ") {
		return ""
	}
	return zh
}

func inferConfig(results []BenchResult) Config {
	return Config{
		BaseURL:     "https://files.wangrongxiang.olares.cn",
		Owner:       "wangrongxiang",
		Samples:     10,
		Timeout:     5 * time.Minute,
		Concurrency: 1,
		UploadSizes: []int{1, 8},
		BigDir:      true,
		Cookie:      "external",
	}
}
