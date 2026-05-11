# Development

This guide covers the backend checkout in this repository. It is intended for
local development and CI maintenance.

## Prerequisites

- Go 1.25.
- Git with submodule support.
- Optional local Postgres and Redis if you are exercising routes that require
  share metadata, search metadata, preview state, watcher state, or SMB state.
- Kubernetes credentials for the full Terminus/Olares runtime path. Some
  startup integrations expect in-cluster resources and service accounts.
- Docker Buildx when building release images.

The repository does not currently contain a frontend package. Treat frontend
setup instructions from older docs as stale unless a matching frontend checkout
has been added separately.

## Runtime Environment

The backend reads configuration from CLI flags, `FB_` environment variables,
and selected platform variables.

Common local variables:

```bash
export ROOT_PREFIX=/tmp/files-data
export PGHOST=127.0.0.1
export PGPORT=5432
export PGUSER=files
export PGPASSWORD=files
export PGDB1=files
export REDIS_HOST=127.0.0.1
export REDIS_PORT=6379
export REDIS_PASSWORD=
export REDIS_DB=0
```

Notes:

- If the `PG*` variables are not set, database initialization is skipped and
  routes that require `database.DB` will not be usable.
- Redis defaults to `localhost:6379` with the package default password when
  variables are omitted. Set the Redis variables explicitly for local work.
- `ROOT_PREFIX` defaults to `/data`. Use a writable local directory when
  running outside the deployment container.
- `FB_SEAFILE_RPC_PATH` configures the Seafile RPC pipe for sync integrations.

## Build And Run

```bash
go mod download
go build -o filebrowser ./cmd/backend
./filebrowser --address 127.0.0.1 --port 8080 --root "$ROOT_PREFIX"
```

Health checks:

```bash
curl -f http://127.0.0.1:8080/healthz
curl -f http://127.0.0.1:8080/ping
```

## Generated Hertz Code

The HTTP API is generated from Thrift IDLs in `pkg/hertz/idl`. CI currently
pins the generator toolchain to avoid non-reproducible diffs:

- `github.com/cloudwego/hertz/cmd/hz@v0.9.7`
- `github.com/cloudwego/thriftgo@v0.4.3`
- `github.com/apache/thrift` replaced with `v0.13.0`

When changing an IDL, regenerate the Hertz code before building. The current CI
workflow performs the equivalent of:

```bash
export GOPATH="${GOPATH:-$HOME/go}"
export PATH="$GOPATH/bin:$PATH"
go install github.com/cloudwego/hertz/cmd/hz@v0.9.7
go install github.com/cloudwego/thriftgo@v0.4.3
go mod tidy
go mod edit -replace github.com/apache/thrift=github.com/apache/thrift@v0.13.0
go mod tidy
cd pkg/hertz
for f in idl/*.thrift; do
  hz update -idl "$f"
done
cd ../..
go mod tidy
```

## Test And Lint

CI builds and tests the packages returned by `go list ./...`, excluding legacy
media packages that currently do not compile and are not imported by
`cmd/backend`.

```bash
go list ./... 2>/dev/null \
  | rg -v '^files/pkg/media/mediabrowser/common/net$|^files/pkg/media/api$' \
  > .ci-pkgs.txt

xargs go build < .ci-pkgs.txt
xargs go test -race -vet=off -count=1 -timeout=180s < .ci-pkgs.txt
xargs go vet < .ci-pkgs.txt
```

`go vet` and Staticcheck are advisory in CI until the existing baseline is
clean. `.golangci.yml` documents the intended high-signal linter set for future
cleanup.

## Docker Images

The repository builds several images:

- `Dockerfile`: combined backend image with the `filebrowser` binary and media
  dependencies.
- `Dockerfile.samba`: Samba sidecar image.
- `Dockerfile.rclone`: rclone remote-control image.
- `Dockerfile.seahub`: Seahub initialization image.
- `Dockerfile.media`: media-focused image.

Release workflows under `.github/workflows` publish these images to Docker Hub.
Keep local build scripts, Dockerfiles, and workflows aligned when changing the
Hertz generator or Go build steps.
