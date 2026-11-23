#!/usr/bin/env bash
set -euo pipefail

action=${1:-}
if [[ $# != 2 || ( $action != connect && $action != disconnect ) ]]; then
  echo "Pemakaian: bash user-testing/android.sh connect|disconnect SERIAL" >&2
  exit 2
fi
device=(adb -s "$2")
"${device[@]}" get-state >/dev/null
if [[ $action == disconnect ]]; then
  "${device[@]}" reverse --remove tcp:43000
  "${device[@]}" reverse --remove tcp:43030
  exit
fi

curl --noproxy '*' -fsS -o /dev/null http://localhost:43000/login
curl --noproxy '*' -fsS http://localhost:43030/health
"${device[@]}" reverse tcp:43030 tcp:43030
"${device[@]}" reverse tcp:43000 tcp:43000
"${device[@]}" reverse --list
echo "Buka http://localhost:43000/login di browser HP."
