import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

const conditionContract = [
  {
    go: 'TypeEdgeConnected',
    schema: 'edgeConnected',
    type: 'edge_connected',
    reasons: {
      connected: 'True',
      channel_disconnected: 'False',
      heartbeat_stale: 'Unknown',
      not_observed: 'Unknown',
    },
  },
  {
    go: 'TypeDeviceCertificateReady',
    schema: 'deviceCertificateReady',
    type: 'device_certificate_ready',
    reasons: {
      valid: 'True',
      rotation_window: 'False',
      expired: 'False',
      revoked: 'False',
      rotation_failed: 'False',
      clock_unreliable: 'Unknown',
      not_observed: 'Unknown',
    },
  },
  {
    go: 'TypeConfigConverged',
    schema: 'configConverged',
    type: 'config_converged',
    reasons: { converged: 'True', revision_drift: 'False', not_observed: 'Unknown' },
  },
  {
    go: 'TypeLocalDatabaseHealthy',
    schema: 'localDatabaseHealthy',
    type: 'local_database_healthy',
    reasons: {
      ready: 'True',
      read_failed: 'False',
      write_failed: 'False',
      integrity_failed: 'False',
      not_observed: 'Unknown',
    },
  },
  {
    go: 'TypeSpoolHealthy',
    schema: 'spoolHealthy',
    type: 'spool_healthy',
    reasons: {
      ready: 'True',
      capacity_warning: 'False',
      capacity_critical: 'False',
      measurement_unavailable: 'Unknown',
      not_observed: 'Unknown',
    },
  },
  {
    go: 'TypeClockQuality',
    schema: 'clockQuality',
    type: 'clock_quality',
    reasons: {
      synchronized: 'True',
      offset_exceeded: 'False',
      unsynchronized: 'False',
      measurement_unavailable: 'Unknown',
      not_observed: 'Unknown',
    },
  },
  {
    go: 'TypeContainerRuntimeReachable',
    schema: 'containerRuntimeReachable',
    type: 'container_runtime_reachable',
    reasons: {
      reachable: 'True',
      probe_failed: 'False',
      probe_timeout: 'False',
      not_observed: 'Unknown',
    },
  },
  {
    go: 'TypePrivilegedHelperReachable',
    schema: 'privilegedHelperReachable',
    type: 'privileged_helper_reachable',
    reasons: {
      reachable: 'True',
      socket_missing: 'False',
      socket_verification_failed: 'False',
      connection_create_failed: 'False',
      rpc_timeout: 'False',
      peer_authentication_failed: 'False',
      rpc_unavailable: 'False',
      api_version_mismatch: 'False',
      not_observed: 'Unknown',
    },
  },
];

const statuses = ['True', 'False', 'Unknown'];
const controlPlaneModel = readFileSync('apps/control-plane/internal/health/model.go', 'utf8');
const edgeModel = readFileSync('apps/edge-agent/internal/health/model.go', 'utf8');
const edgeThresholds = readFileSync('apps/edge-agent/internal/health/thresholds.go', 'utf8');
const proto = readFileSync('proto/guardian/device/v1/device.proto', 'utf8');
const openapi = readFileSync('openapi/guardian.yaml', 'utf8');
const schema = JSON.parse(readFileSync('schemas/device/v1/health-report.schema.json', 'utf8'));

function sortedEntries(value) {
  return Object.entries(value).sort(([left], [right]) => left.localeCompare(right));
}

function parseGoReasons(source, goType) {
  const mapStart = source.indexOf('var reasonStatuses');
  const mapEnd = source.indexOf('var forbiddenMessageFragments', mapStart);
  assert.notEqual(mapStart, -1, 'Go reason map start');
  assert.notEqual(mapEnd, -1, 'Go reason map end');
  const mapSource = source.slice(mapStart, mapEnd);
  const escapedType = goType.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  const match = mapSource.match(new RegExp(`${escapedType}: \\{([\\s\\S]*?)\\n\\t\\},`));
  assert.ok(match, `${goType} reason block`);
  const result = {};
  for (const reasonMatch of match[1].matchAll(/"([a-z][a-z0-9_]*)":\s+Status(True|False|Unknown)/g)) {
    result[reasonMatch[1]] = reasonMatch[2];
  }
  return result;
}

function parseSchemaReasons(definitionName) {
  const contract = schema.$defs[definitionName].allOf[1];
  const result = {};
  for (const branch of contract.oneOf) {
    const status = branch.properties.status.const;
    const reasonSchema = branch.properties.reason;
    const reasons = reasonSchema.enum ?? [reasonSchema.const];
    for (const reason of reasons) result[reason] = status;
  }
  return result;
}

assert.equal(schema.$id, 'https://schemas.guardianpot.internal/device/v1/health-report.schema.json');
assert.equal(schema.properties.schema_version.const, 'guardian.health.report.v1');
assert.equal(schema.properties.conditions.minItems, 8);
assert.equal(schema.properties.conditions.maxItems, 8);
assert.equal(schema.properties.conditions.items, false);
assert.equal(schema.$defs.conditionBase.properties.reason.maxLength, 64);
assert.equal(schema.$defs.conditionBase.properties.message.maxLength, 512);
assert.deepEqual(schema.$defs.conditionBase.properties.status.enum, statuses);

const orderedSchemaDefinitions = schema.properties.conditions.prefixItems.map(({ $ref }) => $ref.split('/').at(-1));
assert.deepEqual(
  orderedSchemaDefinitions,
  conditionContract.map(({ schema: definitionName }) => definitionName),
  'JSON Schema canonical condition order',
);

for (const contract of conditionContract) {
  for (const [sourceName, source] of [
    ['Control Plane', controlPlaneModel],
    ['Edge', edgeModel],
  ]) {
    assert.ok(source.includes(`${contract.go}`), `${sourceName} ${contract.go}`);
    assert.ok(source.includes(`"${contract.type}"`), `${sourceName} ${contract.type}`);
    assert.deepEqual(
      sortedEntries(parseGoReasons(source, contract.go)),
      sortedEntries(contract.reasons),
      `${sourceName} ${contract.type} reasons`,
    );
  }
  const schemaContract = schema.$defs[contract.schema].allOf[1];
  assert.equal(schemaContract.properties.type.const, contract.type, `schema ${contract.type}`);
  assert.deepEqual(
    sortedEntries(parseSchemaReasons(contract.schema)),
    sortedEntries(contract.reasons),
    `schema ${contract.type} reasons`,
  );

  const enumName = contract.type.toUpperCase();
  assert.ok(proto.includes(`HEALTH_CONDITION_TYPE_${enumName}`), `proto ${enumName}`);
  assert.ok(openapi.includes(`        - ${contract.type}`), `OpenAPI ${contract.type}`);
}

for (const status of statuses) {
  assert.ok(proto.includes(`HEALTH_CONDITION_STATUS_${status.toUpperCase()}`), `proto ${status}`);
  assert.ok(openapi.includes(`        - '${status}'`), `OpenAPI ${status}`);
}

for (const source of [controlPlaneModel, edgeModel]) {
  assert.ok(source.includes('SchemaVersion') && source.includes('"guardian.health.report.v1"'));
  assert.ok(source.includes('MaxIdentifierBytes') && source.includes('= 64'));
  assert.ok(source.includes('MaxMessageBytes') && source.includes('= 512'));
  assert.ok(source.includes('MaxReportBytes') && source.includes('16 * 1024'));
  assert.ok(source.includes('HeartbeatInterval') && source.includes('30 * time.Second'));
  assert.ok(source.includes('StaleAfter') && source.includes('90 * time.Second'));
}

for (const expected of [
  'CertificateRotationWindow = 10 * 24 * time.Hour',
  'MaximumHealthyClockOffset = 5 * time.Second',
  'ProbeTimeout              = 2 * time.Second',
  'SpoolWarningRatio         = 0.80',
  'SpoolCriticalRatio        = 0.95',
  'DiskWarningFreePercent    = 10.0',
  'DiskCriticalFreePercent   = 5.0',
]) {
  assert.ok(edgeThresholds.includes(expected), `Edge threshold ${expected}`);
}

for (const healthPath of [
  '/v1/environments/{environmentId}/health',
  '/v1/devices/{deviceId}/health',
]) {
  const pathStart = openapi.indexOf(`  ${healthPath}:`);
  assert.notEqual(pathStart, -1, `OpenAPI contract ${healthPath}`);
  const nextPath = openapi.indexOf('\n  /v1/', pathStart + 1);
  const pathContract = openapi.slice(pathStart, nextPath === -1 ? undefined : nextPath);
  assert.ok(pathContract.includes('\n    get:'), `${healthPath} GET`);
  for (const method of ['post', 'put', 'patch', 'delete']) {
    assert.ok(!pathContract.includes(`\n    ${method}:`), `${healthPath} has no ${method}`);
  }
}

for (const expected of [
  '        - guardianSession: []',
  '            const: no-store',
]) {
  assert.ok(openapi.includes(expected), `OpenAPI contract ${expected.trim()}`);
}

console.log('P1-W9 health contracts are canonical and in parity.');
