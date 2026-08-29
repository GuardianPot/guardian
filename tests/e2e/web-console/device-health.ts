import * as grpc from '@grpc/grpc-js';
import * as protoLoader from '@grpc/proto-loader';
import { randomBytes } from 'node:crypto';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import process from 'node:process';

type Connection = { close(): void };

const health = [
  ['HEALTH_CONDITION_TYPE_EDGE_CONNECTED', 'connected'],
  ['HEALTH_CONDITION_TYPE_DEVICE_CERTIFICATE_READY', 'valid'],
  ['HEALTH_CONDITION_TYPE_CONFIG_CONVERGED', 'converged'],
  ['HEALTH_CONDITION_TYPE_LOCAL_DATABASE_HEALTHY', 'ready'],
  ['HEALTH_CONDITION_TYPE_SPOOL_HEALTHY', 'ready'],
  ['HEALTH_CONDITION_TYPE_CLOCK_QUALITY', 'synchronized'],
  ['HEALTH_CONDITION_TYPE_CONTAINER_RUNTIME_REACHABLE', 'reachable'],
  ['HEALTH_CONDITION_TYPE_PRIVILEGED_HELPER_REACHABLE', 'reachable'],
];

export async function publishHealthy(identityDirectory: string, sequence: number): Promise<Connection> {
  const repoRoot = required('GUARDIAN_E2E_REPO_ROOT');
  const protoRoot = join(repoRoot, 'proto');
  const packageDefinition = protoLoader.loadSync([
    join(protoRoot, 'guardian/device/v1/channel.proto'),
    join(protoRoot, 'guardian/device/v1/device.proto'),
  ], { includeDirs: [protoRoot], keepCase: false, longs: String, enums: String, defaults: true, oneofs: true });
  const loaded = grpc.loadPackageDefinition(packageDefinition) as any;
  const Service = loaded.guardian.device.v1.DeviceChannelService;
  const client = new Service(required('GUARDIAN_E2E_DEVICE_CHANNEL'), grpc.credentials.createSsl(
    readFileSync(required('GUARDIAN_E2E_TLS_CA')),
    readFileSync(join(identityDirectory, 'device.key')),
    readFileSync(join(identityDirectory, 'device.crt')),
  ));
  const stream = client.Connect();
  const reportID = uuidV7();
  const now = { seconds: String(Math.floor(Date.now() / 1000)), nanos: 0 };

  return new Promise<Connection>((resolve, reject) => {
    const timer = setTimeout(() => {
      stream.cancel(); client.close(); reject(new Error('health acknowledgement timed out'));
    }, 15_000);
    stream.on('error', (error: Error) => { clearTimeout(timer); reject(new Error(`device health stream failed: ${error.message}`)); });
    stream.on('data', (message: any) => {
      if (message.protocolSelection) {
        if (!message.protocolSelection.accepted) {
          clearTimeout(timer); reject(new Error('device protocol was rejected')); return;
        }
        stream.write({ healthReport: {
          schemaVersion: 'guardian.health.report.v1', reportId: reportID, sequence: String(sequence), observedAt: now,
          conditions: health.map(([type, reason], index) => ({
            type,
            status: 'HEALTH_CONDITION_STATUS_TRUE',
            reason,
            message: '',
            ...(index === 2 ? { observedRevision: '1' } : {}),
            lastTransitionTime: now,
          })),
        } });
      }
      if (message.desiredState) {
        stream.write({ acknowledgement: {
          messageId: message.desiredState.messageId,
          kind: 'ACKNOWLEDGEMENT_KIND_DESIRED_STATE',
          revision: message.desiredState.revision,
        } });
      }
      if (message.acknowledgement?.messageId === reportID) {
        clearTimeout(timer);
        resolve({ close() { stream.cancel(); client.close(); } });
      }
    });
    stream.write({ hello: { protocol: { major: 1, minor: 0 }, agentVersion: 'guardian-e2e/phase-1' } });
  });
}

function uuidV7() {
  const bytes = randomBytes(16);
  const milliseconds = BigInt(Date.now());
  for (let index = 5; index >= 0; index -= 1) bytes[5 - index] = Number((milliseconds >> BigInt(index * 8)) & 0xffn);
  bytes[6] = (bytes[6] & 0x0f) | 0x70;
  bytes[8] = (bytes[8] & 0x3f) | 0x80;
  const value = bytes.toString('hex');
  return `${value.slice(0, 8)}-${value.slice(8, 12)}-${value.slice(12, 16)}-${value.slice(16, 20)}-${value.slice(20)}`;
}

function required(name: string) {
  const value = process.env[name];
  if (!value) throw new Error(`${name} is required`);
  return value;
}
