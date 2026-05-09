package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

// categoryDescriptions provides human-readable summaries per category.
var categoryDescriptions = map[string]string{
	"health":     "健康检查接口用于 K8s 存活探针和就绪探针，是集群调度的基础。响应时间应保持在毫秒级。",
	"resources":  "资源管理接口涵盖文件与目录的增删改查以及大文件上传，是用户日常操作最频繁的接口。上传接口的耗时与文件分片大小强相关。",
	"tree":       "目录树接口需要递归遍历整个文件系统目录结构，是所有接口中计算量最大的。响应时间与目录深度和文件总数成正比。",
	"raw":        "文件下载接口通过 Nginx 反向代理到后端，延迟主要取决于文件大小和网络带宽。小文件场景下响应迅速。",
	"preview":    "文件预览接口需要对图片进行缩略图生成或格式转换，首次请求会触发图片处理，后续请求可命中缓存。",
	"upload":     "文件上传接口支持分片上传，延迟与分片大小和网络带宽强相关。",
	"paste":      "粘贴/任务接口负责文件复制、移动及后台任务管理。粘贴操作的延迟取决于文件大小和数量。",
	"search":     "搜索接口用于目录检查和全文检索。同步搜索在小规模数据集下表现良好。",
	"share":      "分享接口涵盖分享链接、分享成员、SMB 分享等功能。部分接口因前置条件不满足而跳过测试。",
	"users":      "用户管理接口主要用于列出系统用户列表，涉及 Seafile 后端的 RPC 调用。",
	"repos":      "仓库管理接口用于 Seafile 资料库的创建、列出、重命名和删除操作，涉及多次后端 RPC 调用。",
	"permission": "权限管理接口用于获取和设置文件/目录的访问权限，操作轻量、响应迅速。",
	"md5":        "MD5 校验接口需要读取完整文件并计算哈希值，耗时与文件大小成线性关系。因基准测试不具代表性而跳过。",
	"external":   "外部存储接口用于管理云盘账户、SMB 挂载和挂载历史记录。操作涉及外部系统调用。",
	"callback":   "回调接口用于用户创建/删除的生命周期管理，涉及 Seafile 用户和资料库的创建/删除，因安全原因未执行实际测试。",
	"media":      "媒体接口用于视频转码配置和 HLS 流媒体播放。HLS 相关接口标记为流式，需要更宽裕的超时设置。",
}

func writeDocxEnhanced(results []BenchResult, path string, cfg Config) {
	doc := newDoc()
	var items []interface{}

	// Title
	titleP := wParagraph{
		PPr: &wParagraphPr{
			PStyle: &wPStyle{Val: "Title"},
			Jc:     &wJc{Val: "center"},
		},
		Runs: []wRun{mkRun("Files 服务 API 响应时间基准测试报告", true, 36)},
	}
	items = append(items, titleP)

	// Test conditions
	authMode := "X-Bfl-User（内部）"
	if cfg.Cookie != "" {
		authMode = "Cookie（外部链路）"
	}
	proto := "HTTP"
	if len(cfg.BaseURL) > 5 && cfg.BaseURL[:5] == "https" {
		proto = "HTTPS"
	}
	bigDir := "否"
	if cfg.BigDir {
		bigDir = "是"
	}
	conditions := []struct{ label, value string }{
		{"目标地址", cfg.BaseURL},
		{"测试用户", cfg.Owner},
		{"采样次数", fmt.Sprintf("每接口 %d 次", cfg.Samples)},
		{"并发数", fmt.Sprintf("%d", cfg.Concurrency)},
		{"超时设置", cfg.Timeout.String()},
		{"上传分片大小", formatUploadSizes(cfg.UploadSizes)},
		{"大目录测试", bigDir},
		{"鉴权方式", authMode},
		{"协议", proto},
		{"生成时间", time.Now().Format("2006-01-02 15:04:05")},
	}
	for _, c := range conditions {
		items = append(items, labelValue(c.label, c.value))
	}

	items = append(items, emptyPara())

	// Executive summary
	items = append(items, para("本报告通过对 Files 服务全部 API 接口进行自动化基准测试，采集各接口在真实外部链路下的响应时间数据，"+
		"并据此给出 Nginx 反向代理超时配置的调优建议。报告采用\u201c结论先行\u201d的结构：首先呈现超时配置建议（核心决策依据），"+
		"随后提供各分类接口的详细测试数据作为支撑。", 18))

	// Summary
	items = append(items, heading("一、测试概要", 1))
	items = append(items, para("下表汇总了本次测试覆盖的接口总数及各状态分布情况。"+
		"其中\u201c已测试\u201d为执行了完整多轮采样的接口，\u201c前置准备\u201d和\u201c清理操作\u201d是为保证测试环境而自动执行的辅助操作，"+
		"\u201c跳过\u201d的接口因涉及数据安全或缺少前置条件而未实际执行。", 18))

	var okN, erroredN, setupN, cleanupN, skippedN int
	for _, r := range results {
		switch {
		case r.Note == "skip" || r.Note == "skip-dep":
			skippedN++
		case r.Note == "setup":
			setupN++
		case r.Note == "cleanup":
			cleanupN++
		case r.Status <= 0:
			erroredN++
		default:
			okN++
		}
	}
	summaryData := [][]string{
		{"指标", "数量"},
		{"接口总数", fmt.Sprintf("%d", len(results))},
		{"已测试", fmt.Sprintf("%d", okN)},
		{"前置准备", fmt.Sprintf("%d", setupN)},
		{"清理操作", fmt.Sprintf("%d", cleanupN)},
		{"跳过（不安全）", fmt.Sprintf("%d", skippedN)},
		{"错误", fmt.Sprintf("%d", erroredN)},
	}
	sumTbl := buildSimpleTable(summaryData, 2, true)
	items = append(items, sumTbl)

	// Collect P95 for later use
	grouped := groupByCategory(results)
	cats := sortedCategories(grouped)

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

	// === Section 2: Timeout recommendations (moved up, was section 3) ===
	items = append(items, heading("二、超时配置建议（核心结论）", 1))

	items = append(items, para("本节是报告的核心结论。基于实测 P95 延迟数据，"+
		"我们为每个接口计算了建议超时值。计算规则为：普通接口取 P95 延迟的 3 倍（最低 5 秒），"+
		"流式接口取 P95 延迟的 5 倍（最低 10 秒）。该策略在保障正常请求不被误杀的同时，"+
		"能够及时释放因后端异常而卡住的连接。", 18))

	items = append(items, emptyPara())

	items = append(items, para("对于因安全原因跳过的接口，我们根据同类接口的 P95 分布进行了合理估算，"+
		"并在\u201c依据\u201d列标注为\u201c估算\u201d以作区分。", 18))

	// Separate measured and estimated for clarity
	timeoutHeaders := []string{"分类", "方法", "路径", "P95", "当前超时", "建议超时", "依据"}
	var timeoutMeasured [][]string
	var timeoutEstimated [][]string

	for _, cat := range cats {
		for _, r := range grouped[cat] {
			if r.Note == "setup" || r.Note == "cleanup" || r.Note == "skip" || r.Note == "skip-dep" {
				continue
			}
			if r.Status <= 0 {
				continue
			}
			suggested := suggestTimeout(r.P95, r.Route.Stream)
			timeoutMeasured = append(timeoutMeasured, []string{
				zhCat(cat), r.Route.Method, r.Route.Pattern,
				fmtDuration(r.P95), r.Route.CurrentTimeout, fmtDuration(suggested), "实测",
			})
		}
	}
	for _, cat := range cats {
		for _, r := range grouped[cat] {
			if r.Note != "skip" && r.Note != "skip-dep" {
				continue
			}
			estimated := estimateSkippedTimeout(r, benchedP95s)
			timeoutEstimated = append(timeoutEstimated, []string{
				zhCat(cat), r.Route.Method, r.Route.Pattern,
				"-", r.Route.CurrentTimeout, fmtDuration(estimated), "估算",
			})
		}
	}

	// Measured sub-section
	items = append(items, heading("2.1 实测接口超时建议", 2))
	items = append(items, para(fmt.Sprintf("以下 %d 个接口完成了实际基准测试，建议值基于实测 P95 数据计算得出：", len(timeoutMeasured)), 18))

	if len(timeoutMeasured) > 0 {
		var measuredData [][]string
		measuredData = append(measuredData, timeoutHeaders)
		measuredData = append(measuredData, timeoutMeasured...)
		tbl := buildSimpleTable(measuredData, len(timeoutHeaders), true)
		items = append(items, tbl)
	}

	// Estimated sub-section
	if len(timeoutEstimated) > 0 {
		items = append(items, heading("2.2 跳过接口超时估算", 2))
		items = append(items, para(fmt.Sprintf("以下 %d 个接口因安全原因或前置条件不满足而未执行实际测试。"+
			"建议值根据同类已测接口的 P95 分位数估算得出，仅供参考：", len(timeoutEstimated)), 18))

		var estimatedData [][]string
		estimatedData = append(estimatedData, timeoutHeaders)
		estimatedData = append(estimatedData, timeoutEstimated...)
		tbl := buildSimpleTable(estimatedData, len(timeoutHeaders), true)
		items = append(items, tbl)
	}

	// === Section 3: Per-category detail (was section 2) ===
	items = append(items, heading("三、各分类接口响应时间明细", 1))

	items = append(items, para("本节提供各分类接口的完整测试数据，作为超时配置建议的支撑依据。"+
		"每个分类下按接口列出平均响应时间、P50/P95 分位数、首字节时间等关键指标。"+
		"前置准备（setup）和清理（cleanup）操作也一并展示，以便了解测试环境的完整执行情况。", 18))

	for _, cat := range cats {
		group := collapseGroup(grouped[cat])
		items = append(items, heading(zhCat(cat)+"（"+cat+"）", 2))

		if desc, ok := categoryDescriptions[cat]; ok {
			items = append(items, para(desc, 18))
			items = append(items, emptyPara())
		}

		headers := []string{"方法", "路径", "说明", "请求大小", "平均", "P50", "P95", "首字节", "最小", "最大", "状态码", "当前超时", "备注"}
		var tableData [][]string
		tableData = append(tableData, headers)

		for _, r := range group {
			if r.Note == "skip" || r.Note == "skip-dep" {
				reason := zhSkipReason(r.Route.SkipReason)
				if r.Note == "skip-dep" {
					reason = enhancedSkipDepReason(r)
				}
				row := []string{r.Route.Method, r.Route.Pattern, zhDesc(r.Route.Description),
					"-", "-", "-", "-", "-", "-", "-", "跳过", r.Route.CurrentTimeout, reason}
				tableData = append(tableData, row)
				continue
			}
			if r.Status <= 0 && r.Note == "" {
				errMsg := "请求失败"
				if len(r.Samples) > 0 && r.Samples[0].Error != "" {
					errMsg = r.Samples[0].Error
				}
				row := []string{r.Route.Method, r.Route.Pattern, zhDesc(r.Route.Description),
					"-", "-", "-", "-", "-", "-", "-", "错误", r.Route.CurrentTimeout, errMsg}
				tableData = append(tableData, row)
				continue
			}

			note := zhNote(r.Note, r.Status)
			reqSize := avgReqSize(r.Samples)
			avgTTFB := avgTTFBDuration(r.Samples)

			row := []string{
				r.Route.Method,
				r.Route.Pattern,
				zhDesc(r.Route.Description),
				fmtBytes(reqSize),
				fmtDuration(r.Avg), fmtDuration(r.P50), fmtDuration(r.P95),
				fmtDuration(avgTTFB),
				fmtDuration(r.Min), fmtDuration(r.Max),
				fmt.Sprintf("%d", r.Status),
				r.Route.CurrentTimeout,
				note,
			}
			tableData = append(tableData, row)
		}

		tbl := buildSimpleTable(tableData, len(headers), true)
		items = append(items, tbl)
	}

	// === Section 4: Skipped routes ===
	var skippedRoutes []BenchResult
	for _, cat := range cats {
		for _, r := range grouped[cat] {
			if r.Note == "skip" || r.Note == "skip-dep" {
				skippedRoutes = append(skippedRoutes, r)
			}
		}
	}

	if len(skippedRoutes) > 0 {
		items = append(items, heading("四、跳过的接口说明", 1))
		items = append(items, para(fmt.Sprintf("共 %d 个接口在本次测试中被跳过。"+
			"跳过原因主要包括：接口会实际修改生产数据（如创建/删除 Seafile 用户）、"+
			"缺少必要的前置条件（如需要先创建分享令牌才能测试相关查询接口）、"+
			"以及基准测试不具代表性（如 MD5 计算耗时取决于文件大小）。"+
			"下表汇总了这些接口的基本信息和跳过原因。", len(skippedRoutes)), 18))

		skipHeaders := []string{"分类", "方法", "路径", "说明", "当前超时", "建议超时", "跳过原因"}
		var skipTableData [][]string
		skipTableData = append(skipTableData, skipHeaders)
		for _, r := range skippedRoutes {
			reason := zhSkipReason(r.Route.SkipReason)
			if r.Note == "skip-dep" {
				reason = enhancedSkipDepReason(r)
			}
			suggested := fmtDuration(estimateSkippedTimeout(r, benchedP95s))
			skipTableData = append(skipTableData, []string{
				zhCat(r.Route.Category),
				r.Route.Method,
				r.Route.Pattern,
				zhDesc(r.Route.Description),
				r.Route.CurrentTimeout,
				suggested,
				reason,
			})
		}
		skipTbl := buildSimpleTable(skipTableData, len(skipHeaders), true)
		items = append(items, skipTbl)

		hasAnalysis := false
		for _, r := range skippedRoutes {
			if r.Route.Recommendation != "" {
				hasAnalysis = true
				break
			}
		}
		if hasAnalysis {
			items = append(items, heading("五、跳过接口的深度分析与建议", 1))
			items = append(items, para("以下接口因安全或环境限制未执行实际测试，但我们通过代码走读和调用链分析，"+
				"对其预期性能和合理超时值给出了评估。这些分析可作为超时配置的补充参考依据。", 18))

			for _, r := range skippedRoutes {
				if r.Route.Recommendation == "" {
					continue
				}
				items = append(items, heading(r.Route.Method+" "+r.Route.Pattern+"（"+zhCat(r.Route.Category)+"）", 3))
				items = append(items, wParagraph{
					Runs: []wRun{
						mkRun("跳过原因：", true, 18),
						mkRun(zhSkipReason(r.Route.SkipReason), false, 18),
					},
				})
				items = append(items, wParagraph{
					Runs: []wRun{
						mkRun("当前 Nginx 超时：", true, 18),
						mkRun(r.Route.CurrentTimeout, false, 18),
					},
				})
				items = append(items, para("分析建议："+zhRecommendation(r.Route.Recommendation), 18))
			}
		}
	}

	doc.Body.Items = items

	if err := writeDocxFile(doc, path); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: write enhanced docx: %v\n", err)
	}
}

func enhancedSkipDepReason(r BenchResult) string {
	p := r.Route.Pattern
	switch {
	case strings.Contains(p, "share_token") || strings.Contains(p, "get_token"):
		return "依赖 share_path 前置创建的分享令牌，该令牌未能在 setup 阶段成功获取"
	case strings.Contains(p, "get_share"):
		return "依赖 share_path + share_token 前置创建的分享路径和令牌"
	case strings.Contains(p, "share_path") && (strings.Contains(p, "DELETE") || r.Route.Method == "DELETE"):
		return "依赖 share_path 前置创建的分享路径 ID，该 ID 未能在 setup 阶段捕获"
	case strings.Contains(p, "share_path") || strings.Contains(p, "share_password"):
		return "依赖 share_path 前置创建的分享路径 ID，该 ID 未能在 setup 阶段捕获"
	case strings.Contains(p, "share_member") || strings.Contains(p, "smb_share_member"):
		return "依赖 share_path 前置创建的分享路径 ID，该 ID 未能在 setup 阶段捕获"
	default:
		return "前置接口未返回预期数据，动态参数无法组装"
	}
}

func emptyPara() wParagraph {
	return wParagraph{Runs: []wRun{mkRun("", false, 18)}}
}

func isBigdirSetup(r BenchResult) bool {
	return strings.Contains(r.Route.Pattern, "bigdir setup")
}

// collapseGroup collapses repetitive bigdir setup rows into one summary row,
// keeping all other rows as-is.
func collapseGroup(group []BenchResult) []BenchResult {
	var out []BenchResult
	var bigdirRows []BenchResult

	flush := func() {
		if len(bigdirRows) == 0 {
			return
		}
		merged := bigdirRows[0]
		merged.Route.Pattern = "/api/resources/*path (bigdir setup)"
		merged.Route.Description = fmt.Sprintf("setup: big-dir files x%d (max)", len(bigdirRows))
		for _, r := range bigdirRows[1:] {
			if r.Max > merged.Max {
				merged.Max = r.Max
				merged.Avg = r.Avg
				merged.P50 = r.P50
				merged.P95 = r.P95
				merged.Min = r.Min
			}
		}
		merged.Avg = merged.Max
		merged.P50 = merged.Max
		merged.P95 = merged.Max
		merged.Min = merged.Max
		if len(merged.Samples) > 0 {
			merged.Samples[0].Duration = merged.Max
			merged.Samples[0].TTFB = merged.Max
		}
		out = append(out, merged)
		bigdirRows = nil
	}

	for _, r := range group {
		if isBigdirSetup(r) {
			bigdirRows = append(bigdirRows, r)
		} else {
			flush()
			out = append(out, r)
		}
	}
	flush()
	return out
}
