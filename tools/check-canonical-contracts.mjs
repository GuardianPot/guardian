import { existsSync, readFileSync } from 'node:fs';

const requiredFiles = [
  'proto/guardian/device/v1/device.proto',
  'proto/guardian/telemetry/v1/telemetry.proto',
  'schemas/device/v1/device-identity.schema.json',
  'schemas/telemetry/v1/telemetry-envelope.schema.json',
  'schemas/decoy/v1/decoy-manifest.schema.json',
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
const manifestSchema = JSON.parse(readFileSync('schemas/decoy/v1/decoy-manifest.schema.json', 'utf8'));
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
  // P2-W4. The manifest is the only place a decoy runtime detail can enter,
  // so the shape of that door is a canonical contract like any other.
  [manifestSchema.$id === 'https://guardianpot.dev/schemas/decoy/v1/decoy-manifest.schema.json', 'decoy manifest schema ID'],
  [manifestSchema.properties?.schema?.const === 'guardian.decoy.manifest.v1', 'decoy manifest contract version'],
  [manifestSchema.additionalProperties === false, 'decoy manifest rejects unknown properties'],
  // The privilege vocabulary is the security surface of this contract. A
  // capability added to it is a decision, and this makes adding one fail here
  // rather than pass quietly.
  [
    JSON.stringify(manifestSchema.properties?.privileges?.properties?.capabilities?.items?.enum) === '["NET_BIND_SERVICE"]',
    'decoy manifest grants only NET_BIND_SERVICE',
  ],
  [manifestSchema.properties?.egress?.properties?.policy?.const === 'deny', 'decoy egress is deny-by-default'],
];

const failed = checks.filter(([passed]) => !passed).map(([, name]) => name);
if (failed.length > 0) {
  console.error(`Canonical contract fixture failed: ${failed.join(', ')}`);
  process.exit(1);
}

console.log('Canonical contract fixture passed.');
