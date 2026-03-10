#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

need() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "Required tool not found: $1"
    exit 1
  }
}

need go

GO_CMD="${GO:-go}"
GOSEC_VERSION="${GOSEC_VERSION:-v2.22.2}"
GOBIN_DIR="${GOBIN:-$("$GO_CMD" env GOPATH)/bin}"
GOSEC_BIN="$GOBIN_DIR/gosec"

if [[ ! -x "$GOSEC_BIN" ]]; then
  echo "Installing gosec ${GOSEC_VERSION}..."
  GOBIN="$GOBIN_DIR" "$GO_CMD" install "github.com/securego/gosec/v2/cmd/gosec@${GOSEC_VERSION}"
fi

echo "🔐 Running gosec scan..."
"$GOSEC_BIN" -fmt text ./...
echo "✅ Security gate passed"
