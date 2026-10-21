#!/bin/sh
# Builds ddreport from this checkout and serves the designer for manual testing:
#   ./run-local.sh            http://127.0.0.1:8080
#   ./run-local.sh 9000       another port
# Ctrl+C stops it. NO_BROWSER=1 skips opening the browser. Works in Linux, macOS and Git Bash on Windows.
set -e
cd "$(dirname "$0")"

port="${1:-8080}"
exe=""
case "$(uname -s)" in MINGW*|MSYS*|CYGWIN*) exe=".exe" ;; esac
bin="./ddreport-local$exe"

echo "building $bin ..."
go build -o "$bin" ./cmd/ddreport

url="http://127.0.0.1:$port/"
echo
echo "  designer (Indonesian)   ${url}?lang=id"
echo "  designer (English)      $url"
echo "  Ctrl+C to stop"
echo

# open the browser once the server is up; nothing happens where there is no opener
[ -n "$NO_BROWSER" ] || ( sleep 1
  if command -v xdg-open >/dev/null 2>&1; then xdg-open "${url}?lang=id"
  elif command -v open >/dev/null 2>&1; then open "${url}?lang=id"
  elif [ -n "$exe" ]; then cmd //c start "" "${url}?lang=id"
  fi ) >/dev/null 2>&1 &

trap 'rm -f "$bin"' EXIT
"$bin" -serve "127.0.0.1:$port"
