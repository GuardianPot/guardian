import { existsSync } from 'node:fs';

const generatedDirectories = ['gen', 'apps/web-console/src/generated'];
const present = generatedDirectories.filter((path) => existsSync(path));

if (present.length > 0) {
  console.log(`Generated directories present and committed paths must be checked: ${present.join(', ')}`);
} else {
  console.log('No generated artifact directories exist yet; P0-W10 will activate freshness checks.');
}
