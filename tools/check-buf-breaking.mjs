import { execFileSync } from 'node:child_process';

const baseline = execFileSync('git', ['ls-tree', '-r', '--name-only', 'main', 'proto'], {
  encoding: 'utf8',
});

if (!baseline.split(/\r?\n/).some((path) => path.endsWith('.proto'))) {
  console.log('No Protobuf baseline exists on main; breaking signal is deferred until the first contract merge.');
  process.exit(0);
}

try {
  execFileSync('buf', ['breaking', '--against', '.git#branch=main,subdir=proto'], { stdio: 'inherit' });
  console.log('Buf breaking signal passed.');
} catch (error) {
  console.error('Buf reported a breaking change. Development policy permits owner-reviewed breaking changes; all consumers must be updated together.');
  process.exit(error.status ?? 1);
}
