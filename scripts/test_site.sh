#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
repo_dir="$(cd "$script_dir/.." && pwd -P)"
index="$repo_dir/site/index.html"

[[ -f "$index" && -f "$repo_dir/site/styles.css" && -f "$repo_dir/site/arp-runtime-proof.svg" && -f "$repo_dir/site/404.html" ]]
[[ "$(grep -Eoc '<h1([ >])' "$index")" -eq 1 ]]
for landmark in header main footer nav; do grep -Eq "<$landmark([ >])" "$index"; done
grep -Fq 'Prove the agent runtime you launched is the artifact you approved.' "$index"
grep -Fq 'name="viewport"' "$index"
grep -Fq 'docs/demo.md' "$index"
grep -Fq 'releases/latest' "$index"
grep -Fq 'SECURITY.md' "$index"
grep -Fq 'github.com/fantasyce/agent-runtime-proof' "$index"
grep -Fq '<title' "$repo_dir/site/arp-runtime-proof.svg"
grep -Fq '<desc' "$repo_dir/site/arp-runtime-proof.svg"
grep -Fq 'actions/configure-pages@983d7736d9b0ae728b81ab479565c72886d7745b' "$repo_dir/.github/workflows/pages.yml"
grep -Fq 'actions/upload-pages-artifact@7b1f4a764d45c48632c6b24a0339c27f5614fb0b' "$repo_dir/.github/workflows/pages.yml"
grep -Fq 'actions/deploy-pages@d6db90164ac5ed86f2b6aed7e0febac5b3c0c03e' "$repo_dir/.github/workflows/pages.yml"

if rg -n 'https?://[^" ]+\.(js|css|woff2?|ttf)|<script|googletag|segment\.com|plausible|analytics|href="#"|TODO|PLACEHOLDER' "$repo_dir/site"; then
  echo 'site contains an external dependency, tracker, script, or placeholder' >&2
  exit 1
fi

tmp_root="$(cd "${TMPDIR:-/tmp}" && pwd -P)"
test_root="$(mktemp -d "$tmp_root/arp-site-test.XXXXXX")"
server_pid=""
cleanup() {
  if [[ -n "$server_pid" ]]; then kill "$server_pid" 2>/dev/null || true; wait "$server_pid" 2>/dev/null || true; fi
  case "$test_root" in
    "$tmp_root"/arp-site-test.*) find "$test_root" -depth -delete 2>/dev/null || true ;;
    *) echo 'refusing unexpected site test cleanup path' >&2; return 1 ;;
  esac
}
trap cleanup EXIT INT TERM

port_file="$test_root/port"
python3 - "$repo_dir/site" "$port_file" <<'PY' &
import http.server
import pathlib
import sys

root = pathlib.Path(sys.argv[1])
port_file = pathlib.Path(sys.argv[2])
class QuietHandler(http.server.SimpleHTTPRequestHandler):
    def log_message(self, format, *args):
        pass

handler = lambda *args, **kwargs: QuietHandler(*args, directory=root, **kwargs)
server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), handler)
port_file.write_text(str(server.server_port), encoding="ascii")
server.serve_forever()
PY
server_pid=$!
for _ in 1 2 3 4 5 6 7 8 9 10; do [[ -s "$port_file" ]] && break; sleep 0.1; done
[[ -s "$port_file" ]]
port="$(cat "$port_file")"
curl --fail --silent --show-error "http://127.0.0.1:$port/" > "$test_root/index.html"
cmp "$index" "$test_root/index.html"
curl --fail --silent --show-error "http://127.0.0.1:$port/styles.css" > /dev/null

echo 'site tests passed'
