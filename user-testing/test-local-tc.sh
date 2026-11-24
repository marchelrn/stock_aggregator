#!/usr/bin/env bash
set -euo pipefail

if [[ $# != 1 || ! -r $1 ]]; then
  echo "Usage: $0 /absolute/path/to/Stockbit-TC.pdf" >&2
  exit 2
fi
pdf=$(realpath "$1")
project=$(cd "$(dirname "$0")/.." && pwd)
cache_dir="${XDG_CACHE_HOME:-$HOME/.cache}/stock-tc-model-test"
port=${TEST_OLLAMA_PORT:-11445}
[[ $port =~ ^[0-9]{1,5}$ ]] && (( port >= 1 && port <= 65535 )) || exit 2
export OLLAMA_HOST="127.0.0.1:$port" OLLAMA_NO_CLOUD=1

if ! curl --noproxy '*' -fsS "http://$OLLAMA_HOST/api/tags" >/dev/null 2>&1; then
  if [[ -x "$cache_dir/usr/bin/ollama" ]]; then
    runtime="$cache_dir/usr/bin/ollama"
    export OLLAMA_MODELS="$cache_dir/models"
  else
    runtime=$(command -v ollama) || { echo "Install Ollama locally first." >&2; exit 1; }
  fi
  "$runtime" serve >/dev/null 2>&1 &
  server_pid=$!
  trap 'kill "$server_pid" 2>/dev/null || true; wait "$server_pid" 2>/dev/null || true' EXIT
  for ((i=0;i<30;i++)); do
    if curl --noproxy '*' -fsS "http://$OLLAMA_HOST/api/tags" >/dev/null 2>&1; then break; fi
    sleep 1
  done
fi

cd "$project/api"
env TEST_OLLAMA_LOCAL=1 TEST_OLLAMA_PORT="$port" STOCKBIT_TEST_PDF="$pdf" \
  go test -count=1 -v ./service -run '^TestStockbitLayoutFallbackLocalModel$'
