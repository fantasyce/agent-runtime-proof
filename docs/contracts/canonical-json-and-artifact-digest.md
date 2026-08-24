# Canonical JSON and Artifact Digest Contract

Status: frozen for `agent-runtime-proof/1.0` and
`agent-runtime-expectation/1.0`.

## Canonical JSON profile

ARP uses RFC 8785 JSON Canonicalization Scheme semantics for any bytes covered
by a Proof digest, with these contract restrictions:

- output is UTF-8 with no byte-order mark, trailing newline, or insignificant
  whitespace;
- object properties use RFC 8785 UTF-16 code-unit ordering;
- strings use the RFC 8785 escaping rules and preserve their Unicode scalar
  sequence; ARP does not silently apply NFC or any other Unicode normalization
  to general strings;
- JSON numbers are finite integers in the I-JSON safe range
  `[-9007199254740991, 9007199254740991]`;
- values requiring greater integer precision, including
  `created_at_unix_nano`, are canonical decimal strings with no sign, exponent,
  fraction, or leading zero except the value `0`;
- `NaN`, positive or negative infinity, negative zero, floating-point values,
  duplicate object keys, and trailing JSON values are invalid contract input;
- timestamps are RFC 3339 UTC values ending in `Z`.

These restrictions ensure Go, JavaScript, Python, Swift, and other Agent host
implementations can reproduce the same bytes without losing integer precision.

## Proof digest

To calculate `proof_id`:

1. Validate the complete Proof against its schema and semantic rules.
2. Remove the top-level `proof_id` member; do not replace it with an empty or
   null value.
3. Canonicalize the remaining object with the profile above.
4. Calculate SHA-256 over those exact UTF-8 bytes.
5. Store lowercase hexadecimal as `sha256:<64 lowercase hex digits>`.

The digest makes later byte changes visible. It is not a signature, device
identity, hardware attestation, or statement that every loaded memory page was
measured.

## Path normalization before JSON

Path normalization is a separate artifact operation and happens before a path
enters canonical JSON:

1. Make the path relative to the already resolved artifact root.
2. Reject absolute paths, empty segments, `.` segments, `..` segments, NUL,
   and any resolved target outside the root.
3. Convert platform separators to `/`.
4. Normalize each relative-path string to Unicode NFC.
5. On Windows, derive a case-folded collision key in addition to preserving the
   normalized display path.
6. Reject two inputs that share a normalized or platform collision key.

General Proof strings are not normalized; only artifact relative paths follow
this path profile.

## Artifact tree digest

Each accepted regular file produces exactly this object:

```json
{"path":"bin/agent-runtime-proof","sha256":"<64 lowercase hex digits>","size":12}
```

The artifact digester then:

1. sorts entries by normalized `path` using UTF-8 byte order after collision
   checks;
2. canonicalizes the complete JSON array;
3. hashes those exact bytes with SHA-256;
4. records file count, total byte count, duration, and active scan limits in
   the surrounding observation.

Empty directories are not represented. Symlinks, junctions, reparse points,
devices, sockets, and other non-regular types are rejected unless a later
version introduces an explicit, separately hashed representation. A limit or
read race yields `UNKNOWN`, never a digest of a partial tree.

## Golden vectors

`testdata/canonical/proof-vectors.json` and
`testdata/canonical/artifact-tree-vectors.json` contain literal canonical byte
strings and independently calculated SHA-256 values. Implementations must pass
those vectors before producing Proof IDs or artifact digests.
