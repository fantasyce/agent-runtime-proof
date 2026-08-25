import { spawn } from 'node:child_process';

const [dsh, profile, patch, prompt] = process.argv.slice(2);
if (!dsh || !profile || !patch || !prompt) throw new Error('usage: run-dsh.mjs DSH PROFILE PATCH PROMPT');
const child = spawn(dsh, ['--profile', profile, 'headless', '--patch', patch, prompt], { stdio: ['ignore', 'pipe', 'pipe'], env: process.env });
let stdout = '', stderr = '';
child.stdout.setEncoding('utf8'); child.stderr.setEncoding('utf8');
child.stdout.on('data', value => { stdout += value; }); child.stderr.on('data', value => { stderr += value; });
const exitCode = await new Promise(resolve => child.once('exit', resolve));
if (exitCode !== 0) throw new Error('DSH host invocation failed');
process.stdout.write(JSON.stringify({ status: 'completed', stdout_bytes: Buffer.byteLength(stdout), stderr_bytes: Buffer.byteLength(stderr) }) + '\n');
