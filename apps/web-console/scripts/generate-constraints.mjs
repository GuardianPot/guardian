import { execFileSync } from 'node:child_process';
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { URL } from 'node:url';
import process from 'node:process';

/**
 * Emits the runtime half of the contract (WCX-11 sections 8.2 and 9.1.3).
 *
 * `openapi-typescript` gives the console the contract's *types*, which is
 * everything the compiler needs and nothing a validator can use: `maxLength:
 * 128` is not a type, so it does not survive into `openapi.ts`. Hand-copying it
 * into a schema is exactly what section 8.2 forbids, because a copy drifts
 * silently and a drifted copy accepts input the Control Plane will reject.
 *
 * So the numbers are generated too. Redocly bundles the contract to JSON — the
 * same pinned tool the lint lane already uses — and the constraints for the
 * write-request schemas are written out as a typed module. A constraint changed
 * in `openapi/guardian.yaml` without regenerating fails `generated:check`.
 *
 * Only the write requests are emitted. A response schema has no validator, and
 * emitting the whole contract would ship bytes no form reads.
 */
const SCHEMAS = ['EnvironmentWriteRequest', 'ZoneWriteRequest', 'DecoyWriteRequest'];

/** The constraint keywords a validator can act on. Everything else is prose. */
const KEYWORDS = ['type', 'minLength', 'maxLength', 'pattern', 'enum', 'format'];

const scratch = mkdtempSync(join(tmpdir(), 'guardian-constraints-'));
const bundled = join(scratch, 'contract.json');
let schemas;
try {
  execFileSync(
    'npx',
    ['--yes', '@redocly/cli@2.49.0', 'bundle', '../../openapi/guardian.yaml', '--ext', 'json', '-o', bundled],
    { stdio: 'pipe', shell: process.platform === 'win32' },
  );
  schemas = JSON.parse(readFileSync(bundled, 'utf8')).components.schemas;
} finally {
  rmSync(scratch, { recursive: true, force: true });
}

/** Bundling keeps internal `$ref`s, so a referenced enum is resolved here. */
const resolve = (node) => {
  if (node === null || typeof node !== 'object') return node;
  const reference = node.$ref;
  if (typeof reference !== 'string') return node;
  const name = reference.replace('#/components/schemas/', '');
  if (!Object.prototype.hasOwnProperty.call(schemas, name)) {
    throw new Error(`constraint generation cannot resolve ${reference}`);
  }
  return resolve(schemas[name]);
};

const emitted = {};
for (const name of SCHEMAS) {
  const schema = schemas[name];
  if (schema === undefined) throw new Error(`contract has no schema ${name}`);
  const fields = {};
  for (const [field, rawProperty] of Object.entries(schema.properties ?? {})) {
    const property = resolve(rawProperty);
    const constraint = {};
    for (const keyword of KEYWORDS) {
      if (property[keyword] !== undefined) constraint[keyword] = property[keyword];
    }
    fields[field] = constraint;
  }
  emitted[name] = { required: schema.required ?? [], fields };
}

const banner = `/**
 * Generated from openapi/guardian.yaml. Do not edit.
 *
 * Run \`npm run generate:constraints -w @guardianpot/web-console\` after a
 * contract change. \`generated:check\` fails when this file is stale, so a
 * constraint cannot drift away from the contract it came from (WCX-11 8.2).
 */`;

writeFileSync(
  new URL('../src/generated/constraints.ts', import.meta.url),
  `${banner}\nexport const CONSTRAINTS = ${JSON.stringify(emitted, null, 2)} as const;\n`,
  'utf8',
);
process.stdout.write(`Wrote constraints for ${SCHEMAS.join(', ')}.\n`);
