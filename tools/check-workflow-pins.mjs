import { readdirSync, readFileSync } from 'node:fs';
import { join } from 'node:path';

const workflowDir = '.github/workflows';
const shaPattern = /^[0-9a-f]{40}$/;
const failures = [];

for (const file of readdirSync(workflowDir)) {
  if (!file.endsWith(('.yml', '.yaml'))) continue;
  const path = join(workflowDir, file);
  const lines = readFileSync(path, 'utf8').split(/\r?\n/);
  lines.forEach((line, index) => {
    const match = line.match(/^\s*-?\s*uses:\s*[^@]+@([^\s#]+)/);
    if (match && !shaPattern.test(match[1])) {
      failures.push(`${path}:${index + 1} uses a non-SHA action ref`);
    }
  });
}

if (failures.length > 0) {
  console.error(failures.join('\n'));
  process.exit(1);
}

console.log('Workflow action SHA pin check passed.');
