import { spawn } from 'node:child_process';
import readline from 'node:readline';

const [candidate, expectation, pidText, expectedVerdict = 'MATCHED'] = process.argv.slice(2);
if (!candidate || !expectation || !/^[1-9][0-9]*$/.test(pidText ?? '')) throw new Error('usage: verify-proof.mjs CANDIDATE EXPECTATION PID [VERDICT]');
const child = spawn(candidate, ['mcp'], { stdio: ['pipe', 'pipe', 'pipe'], env: process.env });
let stderr = '';
child.stderr.setEncoding('utf8');
child.stderr.on('data', value => { stderr += value; });
const lines = readline.createInterface({ input: child.stdout });
const pending = new Map();
lines.on('line', line => {
  const value = JSON.parse(line);
  if (value.id !== undefined && pending.has(value.id)) { pending.get(value.id)(value); pending.delete(value.id); }
});
const call = (id, method, params = {}) => new Promise(resolve => { pending.set(id, resolve); child.stdin.write(`${JSON.stringify({ jsonrpc: '2.0', id, method, params })}\n`); });
const initialized = await call(1, 'initialize', { protocolVersion: '2025-06-18', capabilities: {}, clientInfo: { name: 'phase4-host-runner', version: '1' } });
if (initialized.error) throw new Error('initialize failed');
child.stdin.write(`${JSON.stringify({ jsonrpc: '2.0', method: 'notifications/initialized' })}\n`);
const listed = await call(2, 'tools/list');
const names = listed.result?.tools?.map(tool => tool.name).sort();
const expectedNames = ['inspect_local_runtimes', 'list_local_runtime_candidates', 'verify_local_runtime'];
if (JSON.stringify(names) !== JSON.stringify(expectedNames)) throw new Error('tool list mismatch');
const verified = await call(3, 'tools/call', { name: 'verify_local_runtime', arguments: { pid: Number(pidText), expectation_path: expectation } });
const proof = verified.result?.structuredContent?.proof;
if (!proof || proof.verdict !== expectedVerdict || !/^sha256:[0-9a-f]{64}$/.test(proof.proof_id)) throw new Error('proof validation failed');
child.stdin.end();
const exitCode = await new Promise(resolve => child.once('exit', resolve));
if (exitCode !== 0 || stderr !== '') throw new Error('MCP child did not exit cleanly');
process.stdout.write(`${JSON.stringify({ tools: names, verdict: proof.verdict, proof_id: proof.proof_id })}\n`);
