import { createHmac } from 'node:crypto';

let input = '';
for await (const chunk of process.stdin) input += chunk;
const bootstrap = JSON.parse(input);
if (!Array.isArray(bootstrap.recovery_codes) || bootstrap.recovery_codes.length !== 10) process.exit(1);
const provisioning = new URL(bootstrap.provisioning_uri);
const seed = decodeBase32(provisioning.searchParams.get('secret') ?? '');
if (seed.length !== 32) process.exit(1);
const counter = Buffer.alloc(8);
counter.writeBigUInt64BE(BigInt(Math.floor(Date.now() / 30_000)));
const digest = createHmac('sha256', seed).update(counter).digest();
const offset = digest[digest.length - 1] & 0x0f;
const code = String((digest.readUInt32BE(offset) & 0x7fffffff) % 1_000_000).padStart(6, '0');
seed.fill(0);
process.stdout.write(`${JSON.stringify(bootstrap.recovery_codes)}\n${code}\n`);

function decodeBase32(value) {
  const alphabet = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567';
  let bits = 0;
  let accumulator = 0;
  const bytes = [];
  for (const character of value) {
    const index = alphabet.indexOf(character);
    if (index < 0) return Buffer.alloc(0);
    accumulator = (accumulator << 5) | index;
    bits += 5;
    if (bits >= 8) {
      bits -= 8;
      bytes.push((accumulator >> bits) & 0xff);
    }
  }
  return Buffer.from(bytes);
}
