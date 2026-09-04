import { describe, expect, it } from 'vitest';
import { resolveCapability, type Capability } from './capability';

const CAPABILITIES: Capability[] = [
  'environment.create',
  'environment.update',
  'zone.create',
  'zone.update',
  'zone.delete',
  'device.enroll',
  'device.disable',
  'device.revoke',
  'session.revoke',
  'account.password',
];

describe('capability seam', () => {
  it('allows every capability while the session holds a mutation proof', () => {
    for (const capability of CAPABILITIES) {
      expect(resolveCapability(capability, true)).toEqual({ allowed: true });
    }
  });

  it('denies every capability as reauthentication-required without a proof', () => {
    for (const capability of CAPABILITIES) {
      expect(resolveCapability(capability, false)).toEqual({
        allowed: false,
        denial: 'reauthentication-required',
      });
    }
  });

  it('never denies a capability as not-permitted while one local owner exists', () => {
    // `not-permitted` is declared for a future role model and must stay
    // unreachable in this phase, so callers cannot start depending on it.
    for (const capability of CAPABILITIES) {
      for (const canMutate of [true, false]) {
        const decision = resolveCapability(capability, canMutate);
        expect(decision.allowed || decision.denial).not.toBe('not-permitted');
      }
    }
  });
});
