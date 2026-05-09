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

var categoryZh = map[string]string{
	"health": "健康检查", "resources": "资源管理", "tree": "目录树",
	"raw": "文件下载", "preview": "文件预览", "upload": "文件上传",
	"paste": "粘贴/任务", "search": "搜索", "share": "分享",
	"users": "用户管理", "repos": "仓库管理", "permission": "权限管理",
	"md5": "MD5校验", "external": "外部存储", "callback": "回调",
	"media": "媒体",
}

var descZh = map[string]string{
	"liveness ping":                    "存活探针",
	"health check (k8s)":               "健康检查(K8s)",
	"health check (docker)":            "健康检查(Docker)",
	"list root directory":              "列出根目录",
	"list test subdirectory":           "列出测试子目录",
	"get directory tree":               "获取目录树",
	"list storage nodes":               "列出存储节点",
	"raw file download":                "原始文件下载",
	"preview file":                     "预览文件",
	"get upload link":                  "获取上传链接",
	"query uploaded bytes":             "查询已上传字节数",
	"list tasks":                       "列出任务",
	"list users":                       "列出用户",
	"list repos":                       "列出仓库",
	"rename repo":                      "重命名仓库",
	"get repo download info":           "获取仓库下载信息",
	"sync account info":                "同步账户信息",
	"sync search":                      "同步搜索",
	"list external shares":             "列出外部分享",
	"list share members":               "列出分享成员",
	"list share paths":                 "列出分享路径",
	"list share tokens":                "列出分享令牌",
	"list SMB users":                   "列出SMB用户",
	"get share token":                  "获取分享令牌",
	"update share path name":           "更新分享路径名称",
	"reset share password":             "重置分享密码",
	"get permission":                   "获取权限",
	"compute file MD5":                 "计算文件MD5",
	"list cloud accounts":              "列出云盘账户",
	"list mounted drives":              "列出已挂载驱动器",
	"get media config":                 "获取媒体配置",
	"get SMB history":                  "获取SMB历史",
	"update SMB history":               "更新SMB历史",
	"delete SMB history":               "删除SMB历史",
	"check if directory exists":        "检查目录是否存在",
	"get directory listing for search": "获取搜索目录列表",
	"get internal SMB share":           "获取内部SMB分享",
}

func zhDesc(en string) string {
	if zh, ok := descZh[en]; ok {
		return zh
	}
	return en
}

func zhCat(en string) string {
	if zh, ok := categoryZh[en]; ok {
		return zh
	}
	return en
}

// zhSkipReason translates skip reasons to Chinese.
func zhSkipReason(en string) string {
	m := map[string]string{
		"latency = network + O(file_size) MD5 computation; single-file benchmark is not representative":
			"延迟 = 网络 + O(文件大小) MD5 计算；单文件基准测试不具代表性",
		"creates real Seafile user + library; affects shared Seafile DB":
			"会创建真实的 Seafile 用户和资料库，影响共享的 Seafile 数据库",
		"DELETES real Seafile user + all shares; affects shared Seafile DB":
			"会删除真实的 Seafile 用户及所有分享，影响共享的 Seafile 数据库",
		"sends empty JSON body which may corrupt encoding config":
			"发送空 JSON 体可能损坏编码配置",
		"prerequisite not available":
			"前置条件不满足",
	}
	if zh, ok := m[en]; ok {
		return zh
	}
	if strings.Contains(en, "Sync upload requires") {
		return "Sync 上传需要 Seafile 内部 GetUploadLink API 获取的真实访问令牌，无法从外部客户端测试"
	}
	return en
}

// zhRecommendation translates recommendation text to Chinese.
func zhRecommendation(en string) string {
	if strings.Contains(en, "MD5 computation is CPU-bound") {
		return "MD5 计算是 CPU 密集型操作，耗时与文件大小成线性关系。" +
			"处理器读取整个文件进行哈希计算并返回十六进制摘要。" +
			"对于 100MB 文件约需 0.5-2 秒，1GB 文件约需 5-15 秒。" +
			"建议超时设置：5 秒（小文件）至 1800 秒（大文件），与 nginx 的 proxy_read_timeout 1800s 一致。"
	}
	if strings.Contains(en, "HandleCallbackCreate") {
		return "调用链路：HandleCallbackCreate → CreateUser → ListAllUsers" +
			"（3 次 Ccnet RPC + 每个用户 O(N) 次 Redis HGetAll 用于邮箱映射）→ " +
			"CreatePersonalRepo（Seafile RPC）。涉及多次网络往返和 DB 操作。" +
			"建议超时：5 秒（K8s API 在 etcd 压力下可能出现峰值）。"
	}
	if strings.Contains(en, "RemoveUserRelativeAdjustShare") {
		return "调用链路：RemoveUserRelativeAdjustShare（Postgres 事务：" +
			"QuerySharePath + 每路径 DeleteSharePathRelations + 每同步分享 " +
			"DeleteSyncShareRelations）→ RemoveAllReposByOwner（Seafile RPC）→ " +
			"RemoveUser（Ccnet RPC）。O(用户拥有的仓库数 + 分享数) 次数据库操作。" +
			"建议超时：5 秒（K8s API 在 etcd 压力下可能出现峰值）。"
	}
	if strings.Contains(en, "UpdateNamedConfiguration") {
		return "调用链路：UpdateNamedConfiguration → JSON 反序列化 → " +
			"校验 → GetConfiguration → GetConfigurationFromConfigMap" +
			"（K8s API 调用获取 ConfigMap）→ 写入 ConfigMap。" +
			"建议超时：5 秒（K8s API 在 etcd 压力下可能出现峰值）。"
	}
	if strings.Contains(en, "Sync upload shares the same nginx") {
		return "Sync 上传与 Posix 上传共享相同的 nginx location /seafhttp/（proxy_read_timeout 600s），" +
			"请求代理到 seafile:8082。实际上传 I/O 路径与 Posix 类似，但多了一层 Seafile 文件服务器转发。" +
			"以 Posix 上传的实测数据为基准；Sync 上传可能因额外的 Seafile 跳转而略慢。" +
			"建议超时：与 Posix 上传相同或更高（最差网络下至少每 MB 30 秒）。"
	}
	return en
}

func zhNote(note string, status int) string {
	switch note {
	case "setup":
		return "前置"
	case "cleanup":
		return "清理"
	case "skip":
		return "跳过"
	case "skip-dep":
		return "依赖跳过"
	}
	if note == "" && status >= 400 {
		return fmt.Sprintf("HTTP %d", status)
	}
	return note
}

func writeMarkdownZh(results []BenchResult, path string, cfg Config) {
	f, err := os.Create(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: create zh markdown file: %v\n", err)
		return
	}
	defer f.Close()

	fmt.Fprintf(f, "# Files 服务 API 响应时间基准测试报告\n\n")
	fmt.Fprintf(f, "## 测试条件\n\n")
	fmt.Fprintf(f, "| 参数 | 值 |\n|------|----|\n")
	fmt.Fprintf(f, "| 目标地址 | %s |\n", cfg.BaseURL)
	fmt.Fprintf(f, "| 测试用户 | %s |\n", cfg.Owner)
	fmt.Fprintf(f, "| 采样次数 | 每接口 %d 次 |\n", cfg.Samples)
	fmt.Fprintf(f, "| 并发数 | %d |\n", cfg.Concurrency)
	fmt.Fprintf(f, "| 超时设置 | %v |\n", cfg.Timeout)
	fmt.Fprintf(f, "| 上传分片大小 | %s |\n", formatUploadSizes(cfg.UploadSizes))
	bigDirZh := "否"
	if cfg.BigDir {
		bigDirZh = "是"
	}
	fmt.Fprintf(f, "| 大目录测试 | %s |\n", bigDirZh)
	authMode := "X-Bfl-User（内部）"
	if cfg.Cookie != "" {
		authMode = "Cookie（外部链路）"
	}
	fmt.Fprintf(f, "| 鉴权方式 | %s |\n", authMode)
	proto := "HTTP"
	if strings.HasPrefix(cfg.BaseURL, "https://") {
		proto = "HTTPS"
	}
	fmt.Fprintf(f, "| 协议 | %s |\n", proto)
	fmt.Fprintf(f, "| 生成时间 | %s |\n\n", time.Now().Format("2006-01-02 15:04:05"))

	grouped := groupByCategory(results)
	cats := sortedCategories(grouped)

	fmt.Fprintf(f, "## 测试概要\n\n")
	var ok, errored, setupN, cleanupN, skipped int
	for _, r := range results {
		switch {
		case r.Note == "skip" || r.Note == "skip-dep":
			skipped++
		case r.Note == "setup":
			setupN++
		case r.Note == "cleanup":
			cleanupN++
		case r.Status <= 0:
			errored++
		default:
			ok++
		}
	}
	fmt.Fprintf(f, "| 指标 | 数量 |\n|------|------|\n")
	fmt.Fprintf(f, "| 接口总数 | %d |\n", len(results))
	fmt.Fprintf(f, "| 已测试 | %d |\n", ok)
	fmt.Fprintf(f, "| 前置准备 | %d |\n", setupN)
	fmt.Fprintf(f, "| 清理操作 | %d |\n", cleanupN)
	fmt.Fprintf(f, "| 跳过（不安全） | %d |\n", skipped)
	fmt.Fprintf(f, "| 错误 | %d |\n\n", errored)

	for _, cat := range cats {
		group := grouped[cat]
		fmt.Fprintf(f, "## %s（%s）\n\n", zhCat(cat), cat)
		fmt.Fprintf(f, "| 方法 | 路径 | 说明 | 请求大小 | 平均 | P50 | P95 | 首字节(均) | 最小 | 最大 | 状态码 | 当前超时 | 备注 |\n")
		fmt.Fprintf(f, "|------|------|------|----------|------|-----|-----|------------|------|------|--------|----------|------|\n")

		for _, r := range group {
			if r.Note == "skip" || r.Note == "skip-dep" {
				reason := r.Route.SkipReason
				if r.Note == "skip-dep" {
					reason = "前置条件不满足"
				} else if reason == "" {
					reason = "不安全"
				}
			fmt.Fprintf(f, "| %s | `%s` | %s | - | - | - | - | - | - | - | 跳过 | %s | %s |\n",
					r.Route.Method, r.Route.Pattern, zhDesc(r.Route.Description), r.Route.CurrentTimeout, reason)
				continue
			}

			if r.Status <= 0 && r.Note == "" {
				errMsg := "请求失败"
				if len(r.Samples) > 0 && r.Samples[0].Error != "" {
					errMsg = truncate(r.Samples[0].Error, 50)
				}
				fmt.Fprintf(f, "| %s | `%s` | %s | - | - | - | - | - | - | - | 错误 | %s | %s |\n",
					r.Route.Method, r.Route.Pattern, zhDesc(r.Route.Description), r.Route.CurrentTimeout, errMsg)
				continue
			}

			note := zhNote(r.Note, r.Status)
			reqSize := avgReqSize(r.Samples)
			avgTTFB := avgTTFBDuration(r.Samples)

			fmt.Fprintf(f, "| %s | `%s` | %s | %s | %s | %s | %s | %s | %s | %s | %d | %s | %s |\n",
				r.Route.Method, r.Route.Pattern, zhDesc(r.Route.Description),
				fmtBytes(reqSize),
				fmtDuration(r.Avg), fmtDuration(r.P50), fmtDuration(r.P95),
				fmtDuration(avgTTFB),
				fmtDuration(r.Min), fmtDuration(r.Max),
				r.Status, r.Route.CurrentTimeout, note)
		}
		fmt.Fprintln(f)
	}

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

	fmt.Fprintf(f, "## 超时建议\n\n")
	fmt.Fprintf(f, "基于 P95 延迟乘以安全系数（普通接口 3 倍、最低 5 秒，流式接口 5 倍、最低 10 秒），兼顾网络波动与生产环境实际经验：\n\n")
	fmt.Fprintf(f, "| 分类 | 方法 | 路径 | P95 | 当前超时 | 建议超时 | 依据 |\n")
	fmt.Fprintf(f, "|------|------|------|-----|----------|----------|------|\n")
	for _, cat := range cats {
		for _, r := range grouped[cat] {
			if r.Note == "setup" || r.Note == "cleanup" || r.Note == "skip" || r.Note == "skip-dep" {
				continue
			}
			if r.Status <= 0 {
				continue
			}
			suggested := suggestTimeout(r.P95, r.Route.Stream)
			fmt.Fprintf(f, "| %s | %s | `%s` | %s | %s | %s | 实测 |\n",
				zhCat(cat), r.Route.Method, r.Route.Pattern,
				fmtDuration(r.P95), r.Route.CurrentTimeout, fmtDuration(suggested))
		}
	}
	for _, cat := range cats {
		for _, r := range grouped[cat] {
			if r.Note != "skip" && r.Note != "skip-dep" {
				continue
			}
			estimated := estimateSkippedTimeout(r, benchedP95s)
			fmt.Fprintf(f, "| %s | %s | `%s` | - | %s | %s | 估算 |\n",
				zhCat(cat), r.Route.Method, r.Route.Pattern, r.Route.CurrentTimeout, fmtDuration(estimated))
		}
	}
	fmt.Fprintln(f)

	// Skipped routes summary table
	var skippedRoutes []BenchResult
	for _, cat := range cats {
		for _, r := range grouped[cat] {
			if r.Note == "skip" || r.Note == "skip-dep" {
				skippedRoutes = append(skippedRoutes, r)
			}
		}
	}

	if len(skippedRoutes) > 0 {
		fmt.Fprintf(f, "## 跳过的接口一览\n\n")
		fmt.Fprintf(f, "共 %d 个接口被跳过，未执行实际测试。\n\n", len(skippedRoutes))
		fmt.Fprintf(f, "| 分类 | 方法 | 路径 | 说明 | 当前超时 | 建议超时 | 跳过原因 |\n")
		fmt.Fprintf(f, "|------|------|------|------|----------|----------|----------|\n")
		for _, r := range skippedRoutes {
			reason := zhSkipReason(r.Route.SkipReason)
			if r.Note == "skip-dep" {
				reason = "前置条件不满足"
			}
			suggested := fmtDuration(estimateSkippedTimeout(r, benchedP95s))
			fmt.Fprintf(f, "| %s | %s | `%s` | %s | %s | %s | %s |\n",
				zhCat(r.Route.Category), r.Route.Method, r.Route.Pattern,
				zhDesc(r.Route.Description), r.Route.CurrentTimeout, suggested, reason)
		}
		fmt.Fprintln(f)

		// Detailed analysis
		hasAnalysis := false
		for _, r := range skippedRoutes {
			if r.Route.Recommendation != "" {
				hasAnalysis = true
				break
			}
		}
		if hasAnalysis {
			fmt.Fprintf(f, "## 跳过的接口详细分析与建议\n\n")
			fmt.Fprintf(f, "以下接口因安全原因未执行，已基于代码分析给出评估：\n\n")
			for _, r := range skippedRoutes {
				if r.Route.Recommendation == "" {
					continue
				}
				fmt.Fprintf(f, "### %s `%s`（%s）\n\n", r.Route.Method, r.Route.Pattern, zhCat(r.Route.Category))
				fmt.Fprintf(f, "**跳过原因**：%s\n\n", zhSkipReason(r.Route.SkipReason))
				fmt.Fprintf(f, "**当前 Nginx 超时**：%s\n\n", r.Route.CurrentTimeout)
				fmt.Fprintf(f, "**分析建议**：%s\n\n", zhRecommendation(r.Route.Recommendation))
			}
		}
	}
}

func writeCSVZh(results []BenchResult, path string) {
	f, err := os.Create(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: create zh csv file: %v\n", err)
		return
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	_ = w.Write([]string{
		"分类", "方法", "路径", "测试路径", "说明",
		"流式", "阶段", "备注",
		"跳过", "跳过原因", "分析建议",
		"状态码", "采样数",
		"平均(ms)", "P50(ms)", "P95(ms)", "最小(ms)", "最大(ms)",
		"首字节均值(ms)", "请求大小(字节)", "响应大小(字节)",
		"当前超时",
		"错误",
	})

	for _, r := range results {
		stream := "否"
		if r.Route.Stream {
			stream = "是"
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

		skipStr := "否"
		if r.Route.Skip {
			skipStr = "是"
		}

		reqBytes := avgReqSize(r.Samples)
		respBytes := avgRespSize(r.Samples)
		ttfb := avgTTFBDuration(r.Samples)

		noteZh := zhNote(r.Note, r.Status)

		_ = w.Write([]string{
			zhCat(r.Route.Category),
			r.Route.Method,
			r.Route.Pattern,
			testPath,
			zhDesc(r.Route.Description),
			stream,
			strconv.Itoa(r.Route.Phase),
			noteZh,
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
