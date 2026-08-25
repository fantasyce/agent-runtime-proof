#!/usr/bin/env bash

sha256_file() {
  if command -v sha256sum >/dev/null 2>&1; then sha256sum "$1" | awk '{print $1}'; else shasum -a 256 "$1" | awk '{print $1}'; fi
}

sha256_text() {
  if command -v sha256sum >/dev/null 2>&1; then sha256sum | awk '{print $1}'; else shasum -a 256 | awk '{print $1}'; fi
}

file_size() {
  if stat -c '%s' "$1" >/dev/null 2>&1; then stat -c '%s' "$1"; else stat -f '%z' "$1"; fi
}

write_single_file_expectation() {
  local destination="$1" subject_id="$2" subject_name="$3" version="$4" root="$5" entrypoint="$6" file="$7" expected_digest="${8:-}"
  local digest size tree_digest
  digest="$(sha256_file "$file")"
  size="$(file_size "$file")"
  tree_digest="$(printf '[{"path":"%s","sha256":"%s","size":%s}]' "$entrypoint" "$digest" "$size" | sha256_text)"
  if [[ -n "$expected_digest" ]]; then tree_digest="$expected_digest"; fi
  printf '%s\n' "{\"schema_version\":\"agent-runtime-expectation/1.0\",\"subject\":{\"id\":\"$subject_id\",\"display_name\":\"$subject_name\",\"version\":\"$version\"},\"launch\":{\"kind\":\"native\",\"entrypoint\":\"$entrypoint\",\"argument_fingerprints\":[]},\"artifact\":{\"root\":\"$root\",\"include\":[\"$entrypoint\"],\"exclude\":[],\"sha256\":\"$tree_digest\",\"max_files\":1,\"max_bytes\":$size,\"max_duration_ms\":5000},\"policy\":{\"allowed_roots\":[\"$root\"],\"allow_symlinks\":false},\"source\":{\"kind\":\"user-file\",\"locator_hash\":\"0000000000000000000000000000000000000000000000000000000000000000\",\"trust\":\"declared\"}}" > "$destination"
}

assert_safe_proof() {
  local path="$1" verdict="$2"
  grep -q "\"verdict\":\"$verdict\"" "$path"
  grep -Eq '"proof_id":"sha256:[0-9a-f]{64}"' "$path"
  if grep -Eq '/Users/|[A-Za-z]:\\\\Users\\\\|token-super-secret|password-super-secret' "$path"; then
    printf 'unsafe value found in Proof output\n' >&2
    return 1
  fi
}
