import { existsSync, readFileSync } from 'node:fs';

if (!existsSync('package-lock.json')) {
  console.error('package-lock.json is required for reproducible npm installs.');
  process.exit(1);
}

const packageJson = JSON.parse(readFileSync('package.json', 'utf8'));
if (packageJson.engines?.node !== '>=24') {
  console.error('Root Node.js engine must remain on the approved current LTS baseline >=24.');
  process.exit(1);
}

console.log('Dependency policy check passed.');
