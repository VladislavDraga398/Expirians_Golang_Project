#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
GO_CMD="${GO:-go}"

cd "${ROOT_DIR}"
"${GO_CMD}" test -race ./...
