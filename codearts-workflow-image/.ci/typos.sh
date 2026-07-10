#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

WRITE_CHANGES=false

while [[ $# -gt 0 ]]; do
    case $1 in
        --write)
            WRITE_CHANGES=true
            shift
            ;;
        -h|--help)
            echo "Usage: $0 [--write]"
            echo "  --write    Auto-fix typos"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

echo "Installing typos..."
if ! command -v typos &> /dev/null; then
    cargo install typos-cli 2>/dev/null || brew install typos-cli 2>/dev/null || {
        echo "Failed to install typos. Please install manually:"
        echo "  cargo install typos-cli"
        echo "  or: brew install typos-cli"
        exit 1
    }
fi

cd "$PROJECT_ROOT"

echo "Running spell check..."
if [ "$WRITE_CHANGES" = true ]; then
    echo "Auto-fixing typos..."
    typos --format json --write-changes
else
    typos --format json
fi

echo "Spell check completed."
