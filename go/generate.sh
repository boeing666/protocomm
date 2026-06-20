#!/bin/bash
# Builds the protoc-gen-protocomm-go plugin and generates Go code
# for all .proto files under proto/.
#
# Prerequisites:
#   - Go 1.21+
#   - protoc (Protocol Buffer compiler)
#   - protoc-gen-go: go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
#
# Usage:
#   cd go/
#   bash generate.sh

set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

echo "==> Building protoc-gen-protocomm-go..."
cd protoc-gen-protocomm-go
go build -o protoc-gen-protocomm-go .
cd "$SCRIPT_DIR"

PLUGIN="$SCRIPT_DIR/protoc-gen-protocomm-go/protoc-gen-protocomm-go"
# Windows adds .exe automatically; detect it
if [[ -f "${PLUGIN}.exe" && ! -f "$PLUGIN" ]]; then
    PLUGIN="${PLUGIN}.exe"
fi

PROTO_DIR="$SCRIPT_DIR/../proto"

PROTOS=(
    "$PROTO_DIR/example.proto"
    "$PROTO_DIR/calculator.proto"
    "$PROTO_DIR/chat.proto"
)

for proto in "${PROTOS[@]}"; do
    echo "==> Generating $proto"
    protoc \
        --proto_path="$PROTO_DIR" \
        --go_out=. --go_opt=module=github.com/boeing666/protocomm/go \
        --plugin=protoc-gen-protocomm-go="$PLUGIN" \
        --protocomm-go_out=. --protocomm-go_opt=module=github.com/boeing666/protocomm/go \
        "$proto"
done

echo "==> Done. Generated files:"
find proto -name "*.pb.go" | sort
