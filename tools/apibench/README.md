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

### 通过端口转发运行

```bash
kubectl port-forward svc/files-svc 8080:8080 -n <namespace>
./apibench --base-url http://localhost:8080 --owner <user> --samples 10
```

### 参数说明

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `--base-url` | `http://localhost:8080` | 目标服务地址 |
| `--owner` | (必填) | 测试用户名，设置 `X-Bfl-User` 请求头 |
| `--samples` | `5` | 每个接口的采样次数 |
| `--timeout` | `5m` | 单次请求的最大超时时间 |
| `--output` | `./results` | 报告输出目录 |
| `--category` | (空=全部) | 只运行指定分类的接口 |
| `--verbose` | `false` | 打印每次采样的详细结果 |

### 可用分类

`health`, `resources`, `tree`, `raw`, `preview`, `upload`, `paste`,
`search`, `share`, `users`, `repos`, `permission`, `md5`, `external`,
`callback`, `media`

## 执行流程

所有接口都会被执行（无 Skip），按以下阶段运行：

1. **Setup (phase < 0)** — 创建测试目录、上传测试文件等前置操作，单次执行，不纳入 benchmark 统计
2. **Benchmark (phase 0)** — 核心测试，每个接口预热 1 次 + N 次采样
3. **Late cleanup (phase 1-98)** — 需要延迟清理的资源（如 share token、SMB 用户）
4. **Final cleanup (phase 99)** — 删除测试目录、分享路径、测试 repo 等

写操作接口会实际执行，但最终会通过 cleanup 阶段清理掉。

## 输出

运行后在 `--output` 目录下生成：

- `api_benchmark_YYYYMMDD_HHMMSS.md` — Markdown 格式报告
- `api_benchmark_YYYYMMDD_HHMMSS.csv` — CSV 原始数据

### 报告内容

- 按分类分组的响应时间统计（Avg / P50 / P95 / Min / Max）
- 流式接口标记（stream）
- Setup / Cleanup 阶段标注
- 基于 P95 的超时建议值（普通接口 3x，流式接口 5x）

## 动态 ID 捕获

部分接口间有依赖关系（如创建 repo → 获取 download-info → 删除 repo），工具会自动从创建响应中提取 ID 并传递给后续接口：

- `createdRepoID` — 从 POST /api/repos 响应中提取
- `createdSharePath` — 从 POST /api/share/share_path 响应中提取
- `createdTokenID` — 从 POST /api/share/share_token 响应中提取

如果提取失败，后续依赖接口会使用占位值发起请求（可能返回 4xx，仍记录响应时间）。
