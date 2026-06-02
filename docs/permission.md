# 权限判断

统一入口 `access.CheckAccess(ctx, owner, rawURL)`：把前端 URL 映射到具体存储类型，做输入校验后分发到各存储的 `CheckPermission`，返回与 action 无关的统一 `Level`。调用方用 `level.Allow(action)` 判断是否放行；HTTP 处理器一般通过 `Gate` 辅助（见「Gate 加闸辅助」）调用，无需直接接触 `CheckAccess`。

非 share 的 posix 读写面均经此加闸：写面 paste、resources（POST/PATCH/PUT/DELETE）、permission PUT(chown)、archive（compress/extract）、upload（会话创建 `UploadLinkMethod`）；读面 resources GET(List)、preview、raw、md5、archive（entries/entry）。各 driver 的 `CheckPermission`（drive/cache/external/cloud/sync）即经此生效。

**share URL 不走 `CheckAccess`**：share 权限还需 resolved 的分享记录/成员供中间件做反代重写，故由 share 中间件复用本包的 `ShareResolvePath` + `ShareAuthorize`（paste 走 `ShareCheckPaste`，底层矩阵为 `SharePermitted`）决定。

## Level 与 Action

权限等级 `Level`（由低到高）：

- `LevelNone` 无权限
- `LevelRead` 可读（列目录、读取、预览、下载）
- `LevelWrite` 可写（创建、上传、改名、删除、移动/复制目标）
- `LevelAdmin` 管理（含写，并可管理共享）

操作 `Action`：`ActionList` / `ActionRead` / `ActionPreview` / `ActionDownload` / `ActionWrite` / `ActionUpload` / `ActionDelete` / `ActionShareManage`。

`Level.Allow(action)` 矩阵：

- `ActionList/Read/Preview/Download`：需 `>= LevelRead`
- `ActionWrite/Upload/Delete`：需 `>= LevelWrite`
- `ActionShareManage`：需 `LevelAdmin`

## URL 规范化

`rawURL` 为前端格式 `/{fileType}/{extend}/{path...}`，如 `/drive/Home/a.txt`、`/sync/{repoId}/dir/`，由 `CreateFileParam` 解析；解析失败即报错、不放行。（调用方传入的都是前端 URL，故入口不再做后端物理路径的逆向解析。）

## 输入校验

任一不通过即拒绝：

- `owner` 非空，且用户存在（`GlobalData.GetPvcUser(owner) != ""` 或在用户列表中）。
- `fileType` 受支持；`drive` 的 `extend` 必须为 `Home`/`Data`/`Common`；`cache`/`external` 节点存在。
- 角色：见下方 `drive/Common`。

## 各存储类型权限

- **drive（Home/Data）/ cache / cloud（google/dropbox/awss3）/ external**：均为 owner 自己的资源，owner 恒 `LevelAdmin`。
- **drive/Common**（集群共享卷）：按用户的平台角色（platform role，注解 `bytetrade.io/owner-role`，经 `integration.IsPlatformAdmin` 判定）：
  - `owner` / `admin` → `LevelAdmin`
  - 普通用户 → `LevelRead`
- **sync**（Seafile 库）：调用 `seahub.CheckFolderPermission(owner@auth.local, repoId, path)`，字符串映射：
  - `""` → `LevelNone`
  - `"r"` / `"preview"` / `"cloud-edit"` → `LevelRead`
  - `"rw"` → `LevelWrite`
  - `"admin"` → `LevelAdmin`
  - 库只读状态强制返回 `LevelRead`。
- **share**：不经 `CheckAccess`，由 share 中间件调用本包的 `ShareResolvePath` + `ShareAuthorize`（paste 走 `ShareCheckPaste`）处理（这些函数还会返回中间件做反代重写所需的 `SharePath`/`ShareMember`）：
  - `ShareResolvePath` 校验分享是否存在、是否过期（所有者本人且非 `fromShare` 时短路放行，跳过过期）。
  - `ShareAuthorize` 做内部/外部分发：**内部分享**所有者本人放行，否则经 `ShareCheckInternal` 按成员权限（`share_members.permission`，int32 0-4）；**外部分享**经 `ShareCheckExternal` 校验 `token`，按 path 级权限（int32 0-4）。未知 share 类型 **fail-closed**（返回 `ErrShareDenied`）。
  - int32 权限（`0`→拒绝、`1`→只读、`2`→external 仅上传、`3`→读写、`4`→Admin）由 `SharePermitted` 结合 HTTP `Method` 与 `ShareAccess`（resource/preview/raw/download/upload/paste）直接判定该操作是否允许，不经 `Level` 模型。

## Gate 加闸辅助

`handler.Gate(ctx, c, fileParam, action, skipShare, tag)` 是读写处理器统一的加闸入口：

- `skipShare && share=1`：这是 share 反代回环请求，跳过 `CheckAccess`，改为校验内部令牌（见「内部令牌信任边界」），权限已由 share 中间件按成员权限鉴过。
- 其余：调用 `access.CheckAccessParam` 解析 `Level`，按 `Level.Allow(action)` 判定；不通过记日志并返回 `403` + 通用拒绝消息（不回显原始错误）。

## 各接入点加闸

各处理器以何种 action 加闸：

- **paste 处理器**（`pkg/hertz/biz/handler/api/paste/paste_service.go`）：分配任务前先过 `CheckAccess` 授权闸口——src 需读权限、dst 需写权限；`move` 会删源，故 src 改为需写/删权限。`share=1` 时以授权者（`SrcOwner`/`DstOwner`）身份判定；接收方的分享权限仍由 share 代理/中间件把关。src 存在性走 `srcHandler.CheckPathExists`。
- **resources 处理器**（`pkg/hertz/biz/handler/api/resources/resources_service.go`）：`GET`(List) 需读、`POST`(create)/`PATCH`(rename)/`PUT`(edit) 需写、`DELETE` 需删。share 反代请求（`share=1`）跳过 `CheckAccess`——但前提是携带有效内部令牌（见「内部令牌信任边界」），其权限已由 share 中间件按成员权限鉴权。
- **读取处理器**（`preview`/`raw`/`md5`）：分别按 `ActionPreview`/`ActionDownload`/`ActionRead` 加闸，`skipShare=true`（share 反代经内部令牌放行）。
- **permission 处理器**（`pkg/hertz/biz/handler/api/permission/permission_service.go`）：`PUT`(chown/ChownRecursive) 改属主属写类，需写权限；`GET` 见下方「查询接口」。
- **archive 处理器**（`pkg/hertz/biz/handler/api/archive/archive_service.go`）：`entries`/`entry` 需读，`Compress`/`Extract` 对各 source 需读、对 destination 需写（均 posix-only，`skipShare=false`，不在 share 反代路径）。
- **upload 处理器**（`pkg/hertz/biz/handler/upload/upload_service.go`）：`UploadLinkMethod` 是每次上传的会话授权点（chunk POST 依赖此处签发的 uid），仅在此查一次 `ActionUpload`，避免按 chunk 反复查（sync 会变成每块一次 RPC）；`req.Share=="1"` 时不查 `CheckAccess`，改为校验 share 反代附带的内部令牌（失败即 403）。`UploadedBytesMethod` 在 `share=1` 时同样校验内部令牌。sync driver 的写权限拒绝以 `seahub.ErrSyncPermissionDenied` 哨兵返回，`UploadLink`/`UploadedBytes` 经 `errors.Is` 映射为 `403` + 通用拒绝消息（而非裸 500），与 `Gate` 一致。
- **share 中间件**（`pkg/hertz/biz/router/middleware.go`）：分享路径解析、内部/外部成员校验、权限矩阵判定全部委托给 `pkg/access`（读路径 `ShareResolvePath` + `ShareAuthorize`，paste `ShareCheckPaste`，底层矩阵 `SharePermitted`）：
  - **paste**：源/目标分享的查找 + 过期 + 成员阈值由 `access.ShareCheckPaste` 判定（源需成员权限 `>=1`、目标 `>=2`，该阈值刻意区别于 `SharePermitted` 的方法矩阵——只读成员仍可作 paste 源）。
  - **chunk 上传**（`ShareUpload`）：先经 `access.ShareResolvePath` 强制 `share_paths` 过期校验，再经 `access.ShareAuthorize`（`ShareAccess{Upload:true}`）按矩阵加闸，挡住只读成员（或仅持 share id 者）推送分片；链接 / `token` 过期返回 `RespErrorExpired`，其余拒绝 `403`，`CreateFileParam` 解析失败 `400`。
  - **未知 share 类型**：`ShareAuthorize` fail-closed（返回 `ErrShareDenied`）；smb 分享走 samba 挂载，不经此 HTTP 反代。
  - **外部分享读被拒**：返回 `403` + 通用拒绝消息。
- **sync 内部检查**（`pkg/drivers/sync/seahub/*`）：seahub 位于 driver 之下，无法反向调用 `access.CheckAccess`（会成环），故复用同一套语义 `models.LevelFromSyncPermission(perm).Allow(action)`，而非各处手写字符串比较。要点：
  - **统一拒绝哨兵**：纯闸口用 `EnsureSyncPermission`；仍需原始权限串的站点（dir/file/repos/upload 的响应或自定义 URL 逻辑需要 `permission`/`user_perm`/`file_perm`/`IsRepoSyncable`）保留单次 `CheckFolderPermission`，但拒绝统一返 `ErrSyncPermissionDenied` 哨兵，便于边界 `errors.Is` 映射 403。
  - **父目录解析统一**：`seahub.SyncParentDir` 单一实现（`sync.go` 委托之），file→parent 的转换在调用点（`Preview`/`thumbnail`）完成，不在 `CheckFolderPermission` 内做（多数调用点已传目录）。
  - **fail-closed**：库枚举失败、目录共享权限更新失败均上抛 error，不静默成功；`SyncPermToMode` 经 `LevelFromSyncPermission` 推导（None→0、Read→0555、Write/Admin→0755）。
- **driver 接口**：`base.Execute` 暴露 `CheckPermission(p, owner)` 与 `CheckPathExists(p) (exists, isDir, err)`。前者纯逻辑权限判定，后者用于 paste 源存在性 / share 目标 isDir 校验，失败由 err 上抛。Sync 的 `CheckPermission` 在算 Level 前先 `GetRepo`，repo 不存在直接给出明确错误。

### sync 行为变化

按阈值等价迁移（`!= "rw"` → `Allow(ActionWrite)`，`== ""` → `!Allow(ActionRead)`，`Contains(_,"r")` → `Allow(ActionRead)`），仅以下两点与历史行为不同，且均为更安全/更正确方向：

- `admin` 现在在所有写类检查处均视为可写（此前部分站点用 `== "rw"` / `Contains("r")` 把 `admin` 误排除）。`admin` 是 Seafile 中权限更高的角色，不构成越权。
- 未识别的权限字符串现在 fail-closed（按 `LevelNone` 拒绝），此前 `!= ""` 会放行任意非空字符串。
- 对 download/preview/raw 等历史上要求 `rw` 的读类操作，迁移按阈值映射为 `ActionWrite`，仍要求 `rw`/`admin`，**未放宽**给只读用户。

### 内部令牌信任边界

share 反代会回环到本进程（`SERVER_HOST=127.0.0.1:8080`）。`share=1` 这个 query 仅由 share 中间件的反代产生、由下游 `Gate`/upload 会话处理器消费，故用每进程随机密钥（`pkg/common/internal_auth.go` 的 `HeaderInternalShareToken`）把守这道回环，避免伪造的 `share=1` 跳过授权：

- 反代构建请求时附带 `common.InternalShareToken()`（resources/preview/raw/upload-link/upload-bytes 反代与 paste 转发）。
- `Gate`（`skipShare && share=1`）、`UploadLinkMethod`/`UploadedBytesMethod`（`share=1`）、paste 处理器（信任 `SrcOwner`/`DstOwner`）均要求令牌匹配，否则记日志并 `403`，不再仅凭 `share=1` 放行。三处共用 `handler.RequireInternalShareToken(c, tag)` 一个校验函数（统一日志与拒绝消息）。
- RNG 失败时令牌为空，`EqualInternalShareToken` 对空值恒判失败（fail-closed），相关内部转发在进程重启前一律拒绝。

## 查询接口（HTTP）

`GET /api/permission/<dir>?user=<name>`：返回某用户对某目录的统一权限 `Level`，即「给用户 + 目录返回权限」的对外入口。

- `<dir>` 为通配 path，形如 `/drive/Home/docs`；`user` 缺省取 `X-Bfl-User` 头，可用 `?user=` 覆盖。**代查他人**（`user != X-Bfl-User`）仅平台管理员（`integration.IsPlatformAdmin`）允许，否则 `403`。
- 鉴权失败统一返回 `403` + 通用拒绝消息（不回显原始错误）。
- 内部调用 `access.CheckAccessParam`，对全部存储类型（drive/cache/external/cloud/sync）生效，**不要求目标存在**。
- 响应（扁平 JSON）：

  ```json
  { "level": "admin", "can_read": true, "can_write": true, "can_delete": true, "can_upload": true, "uid": 1000 }
  ```

  `level` 为 `none`/`read`/`write`/`admin`；`can_*` 由 `Level.Allow(action)` 得出。`uid`（POSIX 属主）为 best-effort：仅 posix 存储且文件存在时返回，cloud/sync 或文件不存在时省略。
- share URL 不支持（`CheckAccess` 不处理 share），会返回错误。

## 备注

- cloud 当前按「owner 恒 Admin」处理；若存在受限凭据，需补写探测。
- **external 非按用户隔离**：`external`/`internal`/`smb`/`usb`/`hdd` 均解析到共享的 `/data/External`（不带 owner key），`extend` 只校验节点存在。故 `CheckPermission` 对其恒返回 `LevelAdmin`——任一有效平台用户经闸口即可写，读路径也未设闸口。这是现有设计（按节点共享，依赖 OS 文件权限与网关/节点路由兜底），**非本次回归**。若要按用户/设备隔离，属独立的产品决策，需在 `GetResourceUri`/挂载模型层面改造。
