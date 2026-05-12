#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

HZ_VERSION="${HZ_VERSION:-v0.9.7}"
THRIFTGO_VERSION="${THRIFTGO_VERSION:-v0.4.3}"
APACHE_THRIFT_VERSION="${APACHE_THRIFT_VERSION:-v0.13.0}"

export GOPATH="${GOPATH:-$HOME/go}"
export PATH="$GOPATH/bin:$PATH"

cd "$ROOT_DIR"

go install "github.com/cloudwego/hertz/cmd/hz@$HZ_VERSION"
go install "github.com/cloudwego/thriftgo@$THRIFTGO_VERSION"

go mod tidy
go mod edit -replace "github.com/apache/thrift=github.com/apache/thrift@$APACHE_THRIFT_VERSION"
go mod tidy

cd "$ROOT_DIR/pkg/hertz"
for thrift_file in idl/*.thrift; do
  echo "hz update -idl $thrift_file"
  hz update -idl "$thrift_file"
done

cd "$ROOT_DIR"
go mod tidy
