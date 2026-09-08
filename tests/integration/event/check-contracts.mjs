import { readFileSync } from 'node:fs';

/**
 * The canonical event exists in three places and must mean the same thing in
 * all of them: the schema, the Control Plane's ingest type, and the Edge's
 * producer type. Two Go modules cannot share a package, so the vocabularies are
 * declared twice — which is exactly the arrangement that drifts.
 *
 * Declared here rather than derived from one of the three, so agreeing with
 * each other is not enough: all three must agree with this file, and changing a
 * vocabulary means changing the contract deliberately.
 */
const VOCABULARIES = {
  protocol: {
    values: ['ssh', 'http', 'postgres', 'smb', 'tcp'],
    controlPlane: 'protocols',
    edge: 'protocols',
  },
  action: {
    values: [
      'connection_opened',
      'connection_closed',
      'auth_attempt',
      'auth_succeeded',
      'auth_failed',
      'command_executed',
      'request_received',
      'file_accessed',
      'query_executed',
      'session_transcript',
    ],
    controlPlane: 'actionValues',
    edge: 'actions',
  },
  sensitivity: {
    values: ['routine', 'sensitive', 'restricted'],
    controlPlane: 'sensitivity',
    edge: 'sensitivity',
  },
};

const SCHEMA_VERSION = 'guardian.event.v1';

const schema = JSON.parse(readFileSync('schemas/event/v1/canonical-event.schema.json', 'utf8'));
const controlPlane = readFileSync('apps/control-plane/internal/event/event.go', 'utf8');
const edge = readFileSync('apps/edge-agent/internal/normalize/event.go', 'utf8');

let failures = 0;
const check = (name, condition) => {
  if (!condition) {
    console.error(`FAIL ${name}`);
    failures++;
  }
};

check('schema pins the contract version', schema.properties?.schema?.const === SCHEMA_VERSION);
check('Control Plane pins the contract version', controlPlane.includes(`Schema = "${SCHEMA_VERSION}"`));
check('Edge pins the contract version', edge.includes(`Schema = "${SCHEMA_VERSION}"`));

/**
 * Reads a Go `name = []string{...}` literal by scanning rather than by regex.
 * The declarations are formatted by gofmt, so the shape is stable and a scanner
 * has no escaping to get wrong.
 */
function goSlice(source, name) {
  const marker = `${name} `;
  let from = 0;
  for (;;) {
    const at = source.indexOf(marker, from);
    if (at < 0) return null;
    const open = source.indexOf('[]string{', at);
    const equals = source.indexOf('=', at);
    if (open < 0 || equals < 0 || equals > open || open - at > 40) {
      from = at + marker.length;
      continue;
    }
    const close = source.indexOf('}', open);
    if (close < 0) return null;
    const body = source.slice(open + '[]string{'.length, close);
    return [...body.matchAll(/"([^"]*)"/g)].map((entry) => entry[1]);
  }
}

/** Go source with its comments removed, so prose about a field is not a field. */
function withoutComments(source) {
  return source
    .replace(/\/\*[\s\S]*?\*\//g, '')
    .split('\n')
    .map((line) => {
      const at = line.indexOf('//');
      return at < 0 ? line : line.slice(0, at);
    })
    .join('\n');
}

for (const [field, declaration] of Object.entries(VOCABULARIES)) {
  const expected = JSON.stringify(declaration.values);
  check(`schema ${field} vocabulary`, JSON.stringify(schema.properties?.[field]?.enum) === expected);
  check(
    `Control Plane ${field} vocabulary`,
    JSON.stringify(goSlice(controlPlane, declaration.controlPlane)) === expected,
  );
  check(`Edge ${field} vocabulary`, JSON.stringify(goSlice(edge, declaration.edge)) === expected);
}

/**
 * EV-03: credential material is protected or redacted. The guarantee is that no
 * side has a field a secret could live in. Checked against code with comments
 * stripped, because the comments explaining the rule necessarily use the words.
 */
const FORBIDDEN = /\b(password|passphrase|secret|plaintext)\b/i;
for (const [name, source] of [['Control Plane', controlPlane], ['Edge', edge]]) {
  check(`${name} event type has no credential field`, !FORBIDDEN.test(withoutComments(source)));
}
check(
  'schema auth object has no credential field',
  !FORBIDDEN.test(Object.keys(schema.properties?.auth?.properties ?? {}).join(' ')),
);

/**
 * `ingested_time` is the Control Plane's. The Edge's type must not carry it: a
 * struct with nowhere to put it cannot send one.
 */
check('Control Plane owns ingested_time', controlPlane.includes('IngestedTime'));
check('Edge cannot express ingested_time', !withoutComments(edge).includes('IngestedTime'));

if (failures > 0) {
  console.error(`\ncanonical event contract parity failed: ${failures} mismatch(es)`);
  process.exit(1);
}
console.log('Canonical event contract parity holds across the schema, the Control Plane, and the Edge.');
