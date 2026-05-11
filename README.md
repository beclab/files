# files

`files` is the Terminus file manager backend. It provides file browsing,
upload/download, previews, search, sharing, copy/move tasks, cloud sync, and
platform integrations for Terminus/Olares deployments.

The service is written in Go and runs as the `filebrowser` binary. The HTTP API
is served by CloudWeGo Hertz, with route and model code generated from Thrift
IDLs in `pkg/hertz/idl`.

## What is in this repository

- `cmd/backend`: main `filebrowser` server.
- `cmd/samba`: Samba share sidecar service.
- `cmd/seahub`: Seahub/Postgres initialization utility.
- `pkg/hertz`: Hertz server, generated routers, handlers, and IDLs.
- `pkg/drivers`: storage drivers for local POSIX storage, cache/external
  mounts, sync storage, and cloud providers backed by rclone.
- `pkg/tasks`: background paste/sync task manager.
- `pkg/media`: media probing, streaming, and transcoding-related code.
- `Dockerfile*`: images for the backend, Samba sidecar, rclone, Seahub init,
  and media-related packaging.

This checkout does not include a frontend package. If you need the web UI,
confirm the matching frontend repository or submodule for your deployment.

## Requirements

- Go 1.25.
- Postgres for share/search metadata when those features are enabled.
- Redis for cache, watcher, preview, and SMB-related state.
- Kubernetes access for the full Terminus deployment path.
- rclone and Seafile/Seahub services for cloud/sync features.

The backend can compile without all runtime services, but many routes expect
the Terminus platform environment to be present.

## Build

```bash
go mod download
go build -o filebrowser ./cmd/backend
```

Run the server:

```bash
./filebrowser --address 127.0.0.1 --port 8080
```

The health endpoints are available at `/healthz`, `/health`, and `/ping`.

## Configuration

Most CLI flags can be supplied through `FB_` environment variables via Viper.
Common runtime variables include:

- `PGHOST`, `PGPORT`, `PGUSER`, `PGPASSWORD`, `PGDB1` for Postgres.
- `REDIS_HOST`, `REDIS_PORT`, `REDIS_PASSWORD`, `REDIS_DB` for Redis.
- `ROOT_PREFIX` for the data root, defaulting to `/data`.
- `FB_SEAFILE_RPC_PATH` for the Seafile RPC pipe.
- `TERMINUSD_HOST`, `EXTERNAL_PREFIX`, `NODE_NAME` for platform integration.

See `cmd/backend/app/root.go`, `pkg/hertz/biz/dal/database/init.go`,
`pkg/redisutils/redis_client.go`, and `pkg/common/constant.go` for the current
source of truth.

## Development

Developer setup, generated code, CI package exclusions, and Docker notes are in
`docs/development.md`.
