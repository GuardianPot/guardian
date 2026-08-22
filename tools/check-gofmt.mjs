import { execFileSync } from 'node:child_process';

const output = execFileSync('gofmt', ['-l', 'apps/control-plane', 'apps/edge-agent'], {
  encoding: 'utf8',
});

if (output.trim()) {
  console.error(`Go files need formatting:\n${output}`);
  process.exit(1);
}

console.log('Go format check passed.');
