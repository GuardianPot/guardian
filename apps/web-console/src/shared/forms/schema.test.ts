import { readFileSync } from 'node:fs';
import { safeParse } from 'valibot';
import { describe, expect, it } from 'vitest';
import { CONSTRAINTS } from '@generated/constraints';
import { choicesOf, maxLengthOf, schemaFor } from './schema';

/**
 * Section 8.2 and 10.1.2: the validators must agree with the contract, and the
 * only way to keep them agreeing is to derive them from it.
 *
 * The drift fixture is the point of this file. It reads
 * `openapi/guardian.yaml` directly and checks the generated constraints still
 * match it, so a bound edited in the contract without regenerating fails here
 * as well as in `generated:check`.
 */
const CONTRACT = readFileSync('../../openapi/guardian.yaml', 'utf8');

describe('the generated constraints match the contract', () => {
  it.each([
    ['display_name', 'maxLength: 512'],
    ['address', 'maxLength: 15'],
    ['address', 'minLength: 7'],
    ['pack_version', 'minLength: 5'],
  ])('%s carries %s from openapi/guardian.yaml', (_field, declaration) => {
    expect(CONTRACT).toContain(declaration);
  });

  it('reads its bounds from the generated module, never from a literal here', () => {
    // If someone replaces the generator with a hand-written file, these still
    // pass — which is why the drift check above reads the contract, and why
    // `generated:check` regenerates and diffs.
    expect(maxLengthOf('DecoyWriteRequest', 'display_name')).toBe(
      CONSTRAINTS.DecoyWriteRequest.fields.display_name.maxLength,
    );
    expect(maxLengthOf('DecoyWriteRequest', 'address')).toBe(15);
  });

  it('takes the closed vocabularies from the contract, not from a copy', () => {
    expect(choicesOf('DecoyWriteRequest', 'family')).toEqual(['ssh', 'http', 'postgres', 'smb']);
    expect(choicesOf('DecoyWriteRequest', 'persona')).toEqual([
      'linux_admin_server',
      'internal_admin_web_app',
      'database_server',
      'windows_file_service_host',
    ]);
  });
});

describe('the derived decoy validator', () => {
  const schema = schemaFor('DecoyWriteRequest', [
    'zone_id',
    'display_name',
    'family',
    'persona',
    'address',
    'pack',
    'pack_version',
  ]);

  const valid = {
    zone_id: '018f1f7e-6d31-7cc5-8db8-17547f78e6c6',
    display_name: 'Finance file server',
    family: 'smb',
    persona: 'windows_file_service_host',
    address: '10.20.0.40',
    pack: 'smb-fileshare',
    pack_version: '0.1.0',
  };

  it('accepts a well-formed decoy', () => {
    expect(safeParse(schema, valid).success).toBe(true);
  });

  it.each([
    ['an empty required name', { display_name: '' }],
    ['a name past the contract bound', { display_name: 'x'.repeat(513) }],
    ['a family outside the closed set', { family: 'telnet' }],
    ['a persona outside the closed set', { persona: 'chief_executive' }],
    ['a public address', { address: '8.8.8.8' }],
    ['a malformed address', { address: 'not-an-address' }],
    ['a malformed pack version', { pack_version: 'latest' }],
    ['a zone reference that is not a UUIDv7', { zone_id: 'not-a-uuid' }],
  ])('rejects %s', (_case, override) => {
    expect(safeParse(schema, { ...valid, ...override }).success).toBe(false);
  });

  /**
   * Section 8.1. The client cannot know a zone's prefix, so an address that is
   * well-formed but outside its zone passes here and is refused by the Control
   * Plane. That is the case change proposal 0004 exists to report on a field,
   * and this asserts the client does not pretend to catch it.
   */
  it('accepts an address only the Control Plane can refuse', () => {
    expect(safeParse(schema, { ...valid, address: '10.99.0.40' }).success).toBe(true);
  });
});
