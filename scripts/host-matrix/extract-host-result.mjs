import fs from 'node:fs';
import os from 'node:os';

const [host, hostVersion, commit, listPath, eventsPath, messagePath] = process.argv.slice(2);
if (!messagePath) throw new Error('missing result inputs');
const list = fs.readFileSync(listPath, 'utf8');
const events = fs.readFileSync(eventsPath, 'utf8');
const message = fs.readFileSync(messagePath, 'utf8');
if (!list.includes('agent-runtime-proof')) throw new Error('host did not list the MCP server');
if (!events.includes('verify_local_runtime')) throw new Error('host did not call verify_local_runtime');
const match = message.match(/ARP_HOST_PASS verdict=([A-Z_]+) proof_id=(sha256:[0-9a-f]{64}) binding_id=([a-z0-9.-]+)/);
if (!match) throw new Error('host did not return a bounded Proof result');
if (match[1] !== 'MATCHED') throw new Error(`unexpected host verdict: ${match[1]}`);
if (match[3] !== `${host}.agent-runtime-proof`) throw new Error('unexpected binding attribution');
process.stdout.write(JSON.stringify({
  schema_version: 'agent-runtime-proof-host-evidence/1.0',
  host_id: host,
  host_version: hostVersion,
  platform: `darwin-${os.arch()}`,
  arp_commit: commit,
  tool_names: ['inspect_local_runtimes', 'list_local_runtime_candidates', 'verify_local_runtime'],
  verdict: match[1],
  proof_id: match[2],
  binding_id: match[3],
  cleanup: 'task-owned',
}, null, 2) + '\n');
