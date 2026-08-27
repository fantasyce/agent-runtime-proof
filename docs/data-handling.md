# Data handling

Agent Runtime Proof observes local current-user process identity and declared
local artifact state. It runs as a CLI, a local `stdio` MCP child, or an
optional launch Witness.

## What stays local

ARP does not contain a network client, open a listener, create an account, or
send telemetry. Ordinary inspection and MCP calls are not persisted. Launch
receipts are created only when the caller explicitly uses Witness and are
stored under the caller-selected ARP home.

## What public Proofs omit

Projected Proofs omit raw argv, environment values, command lines, file
contents, credentials, headers, transcripts, and raw host configuration. Paths
are omitted or represented through bounded identifiers and hashes according to
the interface and privacy registry.

## What ARP may read

Depending on the requested operation and operating-system permissions, ARP may
read current-user process metadata, explicitly supplied expectation files,
declared artifact roots, and supported host configuration files as inert data.
Configuration discovery does not execute commands, interpolation, YAML tags,
plugins, or imports.

Permission denial and incomplete observation become evidence limitations. ARP
does not request elevation or weaken host controls to turn an uncertain result
into a match.

## User responsibility

Do not publish raw terminal history, surrounding logs, expectation files, or
screenshots without reviewing them. The privacy projection applies to ARP's
structured output, not to unrelated shell output captured alongside it.

For the normative field-level contract, see [privacy-model.md](privacy-model.md)
and `contracts/privacy-registry.json`. Report suspected disclosure privately
through [the security policy](../SECURITY.md).
