#!/usr/bin/env bash
set -euo pipefail
here=$(cd -- "$(dirname -- "$0")" && pwd)
runtime="${XDG_CACHE_HOME:-$HOME/.cache}/stock-aggregator/android-nginx"
mkdir -p -m 700 "$runtime"
cd "$here/../../frontend"
VITE_ENV=DEVELOPMENT npm run build -- --outDir "$runtime/www" --emptyOutDir
nginx -p "$runtime/" -c "$here/nginx.conf" -t
if [[ -s "$runtime/nginx.pid" ]] && kill -0 "$(cat "$runtime/nginx.pid")" 2>/dev/null; then
    nginx -p "$runtime/" -c "$here/nginx.conf" -s reload
else
    nginx -p "$runtime/" -c "$here/nginx.conf"
fi
