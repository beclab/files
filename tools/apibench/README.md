# API Response Time Benchmark

对 files 服务的全部 HTTP 接口发起真实请求，采集响应时间并输出统计报告，用于指导超时值设定。

## 构建

本地构建：

```bash
cd tools/apibench
go build -o apibench .
```

CI 构建会自动将 `apibench` 编译并打入 Docker 镜像，部署后即可直接使用。

## 使用

### 在已部署的 Pod 内直接运行

apibench 已随镜像部署到 `/usr/local/bin/apibench`，直接 exec 进 Pod 运行即可：

```bash
kubectl exec -it <pod> -n <namespace> -- \
  apibench --base-url http://localhost:8080 --owner <user> --samples 10
```

### 通过 Ingress/外部链路运行（推荐用于生产超时评估）

走真实的外部网络路径（经过 Ingress / TLS / 反向代理），使用浏览器 Cookie 鉴权：

```bash
./apibench \
  --base-url https://files.example.com \
  --owner <user> \
  --cookie "auth_token=eyJ..." \
  --samples 10 \
  --concurrency 3 \
  --upload-sizes 1,4,8
```

### 通过端口转发运行

```bash
kubectl port-forward svc/files-svc 8080:8080 -n <namespace>
./apibench --base-url http://localhost:8080 --owner <user> --samples 10
```

### 参数说明

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `--base-url` | `http://localhost:8080` | 目标服务地址（支持 https） |
| `--owner` | (必填) | 测试用户名，设置 `X-Bfl-User` 请求头 |
| `--samples` | `5` | 每个接口的采样次数 |
| `--timeout` | `5m` | 单次请求的最大超时时间 |
| `--output` | `./results` | 报告输出目录 |
| `--category` | (空=全部) | 只运行指定分类的接口 |
| `--verbose` | `false` | 打印每次采样的详细结果 |
| `--cookie` | (空) | 浏览器 Cookie 字符串，用于经 Ingress 鉴权 |
| `--concurrency` | `1` | 并发 worker 数，模拟多用户同时请求 |
| `--upload-sizes` | `1,8` | 上传分片大小列表（MB），逗号分隔 |
| `--big-dir` | `false` | 在 setup 阶段创建 200 个文件，测试大目录列表性能 |

### 可用分类

`health`, `resources`, `tree`, `raw`, `preview`, `upload`, `paste`,
`search`, `share`, `users`, `repos`, `permission`, `md5`, `external`,
`callback`, `media`

## 执行流程

所有接口都会被执行（标记 Skip 的除外），按以下阶段运行：

1. **Setup (phase < 0)** — 创建测试目录、上传测试文件等前置操作，单次执行，不纳入 benchmark 统计。如果开启 `--big-dir`，会在 phase -2 创建 200 个小文件。
2. **Benchmark (phase 0)** — 核心测试，每个接口预热 1 次 + N 次采样。如果 `--concurrency > 1`，采样会并发执行。
3. **Late cleanup (phase 1-98)** — 需要延迟清理的资源（如 share token、SMB 用户、上传的测试文件）
4. **Final cleanup (phase 99)** — 删除测试目录、分享路径、测试 repo 等

### 上传测试

工具会为 `--upload-sizes` 中的每个大小生成真实的 multipart/form-data 上传请求体，分别测试 Posix 和 Sync 两种上传通道：

- `POST /upload/upload-link/:node/:uid` — Posix 上传
- `POST /seafhttp/:upload/:uid` — Sync 上传
- `PUT /api/resources/*path` — HTTP PUT 上传

每个大小会使用随机数据填充，模拟真实的分片上传场景。

### 幂等性与清理

- PUT 上传使用固定路径名（如 `bench_upload_8mb.bin`），重复执行会覆盖而非创建新文件
- 所有上传的文件在 cleanup 阶段会被删除
- 分享、Repo、SMB 用户等资源在各自的 cleanup phase 中清理

## 输出

运行后在 `--output` 目录下一次性生成 5 个文件：

- `api_benchmark_YYYYMMDD_HHMMSS.md` — Markdown 格式报告（英文）
- `api_benchmark_YYYYMMDD_HHMMSS.csv` — CSV 原始数据（英文）
- `api_benchmark_YYYYMMDD_HHMMSS_zh.md` — Markdown 格式报告（中文）
- `api_benchmark_YYYYMMDD_HHMMSS_zh.csv` — CSV 原始数据（中文）
- `api_benchmark_YYYYMMDD_HHMMSS_zh.docx` — Word 报告（中文），可直接用于提交

### 报告内容

- **测试条件表**：记录 base-url、并发数、上传大小、鉴权模式、协议等完整测试参数
- **分类统计表**：按分类分组的响应时间统计（Avg / P50 / P95 / TTFB / Min / Max），附带请求大小列
- **超时建议**：基于 P95 的动态超时建议值（普通接口 3x，流式接口 5x，最低 1s），区分实测和估算
- **危险接口估算**：对标记 Skip 的接口，从分析文本中提取建议超时值，或基于已测接口 P95 分位数估算
- **代码分析**：对跳过的接口提供完整的调用链分析和超时建议

## 动态 ID 捕获

部分接口间有依赖关系（如创建 repo → 获取 download-info → 删除 repo），工具会自动从创建响应中提取 ID 并传递给后续接口：

- `createdRepoID` — 从 POST /api/repos 响应中提取
- `createdSharePath` — 从 POST /api/share/share_path 响应中提取
- `createdTokenID` — 从 POST /api/share/share_token 响应中提取

如果提取失败，后续依赖接口会使用占位值发起请求（可能返回 4xx，仍记录响应时间）。
