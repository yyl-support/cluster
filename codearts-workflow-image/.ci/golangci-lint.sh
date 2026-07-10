#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
GOLANGCI_BIN="${GOLANGCI_BIN:-$HOME/.local/bin/golangci-lint}"

EXTRA_ARGS=""

while [[ $# -gt 0 ]]; do
    case $1 in
        --args)
            EXTRA_ARGS="$2"
            shift 2
            ;;
        -h|--help)
            echo "Usage: $0 [--args '...']"
            echo "  --args    Additional golangci-lint arguments"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

cd "$PROJECT_ROOT"

echo "Setting up git netrc for private modules..."
export GOPRIVATE="github.com/opensourceways"

if [ -n "$TOKEN" ] && [ -n "$USER" ]; then
    touch ~/.netrc
    chmod 600 ~/.netrc
    echo "machine github.com login $USER password $TOKEN" > ~/.netrc
    echo "Git netrc configured."
fi

echo "Checking golangci-lint..."
if ! command -v "$GOLANGCI_BIN" &> /dev/null; then
    echo "Installing golangci-lint..."
    GOBIN="$HOME/.local/bin" go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
fi

cd "$PROJECT_ROOT/go/cmd/converter"

echo "Running golangci-lint..."
"$GOLANGCI_BIN" run -v --config="$PROJECT_ROOT/.golangci.yml" --max-same-issues=0 $EXTRA_ARGS

echo "Lint completed."
