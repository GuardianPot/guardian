import { existsSync, readFileSync } from 'node:fs';

const requiredFiles = [
  'proto/guardian/device/v1/device.proto',
  'proto/guardian/telemetry/v1/telemetry.proto',
  'schemas/device/v1/device-identity.schema.json',
  'schemas/telemetry/v1/telemetry-envelope.schema.json',
  'openapi/guardian.yaml',
  'docs/contracts/README.md',
];

const missing = requiredFiles.filter((path) => !existsSync(path));
if (missing.length > 0) {
  console.error(`Missing canonical contract files: ${missing.join(', ')}`);
  process.exit(1);
}

const deviceSchema = JSON.parse(readFileSync('schemas/device/v1/device-identity.schema.json', 'utf8'));
const telemetrySchema = JSON.parse(readFileSync('schemas/telemetry/v1/telemetry-envelope.schema.json', 'utf8'));
const checks = [
  [deviceSchema.$id === 'https://schemas.guardianpot.internal/device/v1/device-identity.schema.json', 'device schema ID'],
  [telemetrySchema.$id === 'https://schemas.guardianpot.internal/telemetry/v1/telemetry-envelope.schema.json', 'telemetry schema ID'],
  [deviceSchema.properties?.contract_version?.const === 'guardian.device.v1', 'device contract version'],
  [telemetrySchema.properties?.schema_version?.const === 'guardian.telemetry.v1', 'telemetry contract version'],
  [telemetrySchema.required?.includes('event_id'), 'stable telemetry event ID'],
  [telemetrySchema.required?.includes('device_id'), 'telemetry device ID'],
  [readFileSync('proto/guardian/device/v1/device.proto', 'utf8').includes('package guardian.device.v1;'), 'device protobuf package'],
  [readFileSync('proto/guardian/telemetry/v1/telemetry.proto', 'utf8').includes('package guardian.telemetry.v1;'), 'telemetry protobuf package'],
  [readFileSync('openapi/guardian.yaml', 'utf8').includes('  /v1/telemetry:'), 'OpenAPI v1 path'],
  [readFileSync('docs/contracts/README.md', 'utf8').includes('P0-W8'), 'W8 traceability'],
  [readFileSync('docs/contracts/README.md', 'utf8').includes('P0-W9'), 'W9 traceability'],
];

const failed = checks.filter(([passed]) => !passed).map(([, name]) => name);
if (failed.length > 0) {
  console.error(`Canonical contract fixture failed: ${failed.join(', ')}`);
  process.exit(1);
}

console.log('Canonical contract fixture passed.');
