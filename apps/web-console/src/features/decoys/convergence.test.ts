import { describe, expect, it } from 'vitest';
import { untrusted } from '@shared/api/untrusted';
import type { DecoyView } from '@shared/api/types';
import { CONVERGENCE_WINDOW_MS, convergenceOf, hasBeenObserved, observationAgeMs } from './convergence';

const CHANGED_AT = '2026-09-08T12:00:00.000Z';
const changedAtMs = Date.parse(CHANGED_AT);

function view(overrides: {
  revision?: number;
  desiredRevision?: number | null;
  observedState?: DecoyView['observed']['observed_state'];
  reportedAt?: string | null;
}): DecoyView {
  return {
    decoy: {
      decoy_id: '0198dc8c-c600-7000-8000-000000000005',
      environment_id: '0198dc8c-c600-7000-8000-000000000003',
      zone_id: '0198dc8c-c600-7000-8000-000000000004',
      display_name: untrusted('Finance file server'),
      family: 'smb',
      persona: 'windows_file_service_host',
      interaction_level: 'low',
      address: '10.20.0.40',
      pack: 'smb-fileshare',
      pack_version: '0.1.0',
      pack_digest: null,
      desired_state: 'deployed',
      revision: overrides.revision ?? 3,
      created_at: CHANGED_AT,
      updated_at: CHANGED_AT,
    },
    observed: {
      observed_state: overrides.observedState ?? 'unknown',
      reporting_device_id: null,
      reported_at: overrides.reportedAt === undefined ? null : overrides.reportedAt,
      last_interaction_at: null,
      desired_revision:
        overrides.desiredRevision === undefined ? null : overrides.desiredRevision,
      conditions: [],
    },
  };
}

describe('decoy convergence', () => {
  it('is converged only when an Edge confirmed the revision on screen', () => {
    const result = convergenceOf(view({ revision: 3, desiredRevision: 3 }), changedAtMs);
    expect(result).toEqual({ state: 'converged', observedRevision: 3 });
  });

  it('is pending inside the window and never claims a failure', () => {
    const result = convergenceOf(
      view({ revision: 4, desiredRevision: 3 }),
      changedAtMs + CONVERGENCE_WINDOW_MS - 1,
    );
    expect(result.state).toBe('pending');
  });

  /**
   * The important one. Past the window the console says it is still waiting; it
   * does not say the decoy failed. Section 9.5 forbids inventing a timeout, and
   * an overdue convergence is an absence of news, not news of a failure.
   */
  it('reports an overdue wait rather than inventing a failure', () => {
    const result = convergenceOf(
      view({ revision: 4, desiredRevision: 3 }),
      changedAtMs + CONVERGENCE_WINDOW_MS + 1,
    );
    expect(result.state).toBe('overdue');
    expect(result).not.toHaveProperty('reason');
    if (result.state === 'overdue') {
      expect(result.observedRevision).toBe(3);
      expect(result.elapsedMs).toBeGreaterThan(CONVERGENCE_WINDOW_MS);
    }
  });

  it('treats a decoy nothing has reported on as pending, never as converged', () => {
    const result = convergenceOf(view({ revision: 1, desiredRevision: null }), changedAtMs);
    expect(result.state).toBe('pending');
    if (result.state === 'pending') expect(result.observedRevision).toBeNull();
  });

  /** SEC-06: convergence is not a question that can be asked of a decoy the
   * Control Plane can no longer drive. */
  it('does not ask about convergence for an unmanaged decoy', () => {
    const result = convergenceOf(view({ observedState: 'unmanaged', desiredRevision: 1 }), changedAtMs);
    expect(result).toEqual({ state: 'unmanaged' });
  });

  it('separates never-observed from observed-and-old', () => {
    expect(hasBeenObserved(view({}))).toBe(false);
    expect(observationAgeMs(view({}))).toBeNull();

    const reported = view({ reportedAt: CHANGED_AT });
    expect(hasBeenObserved(reported)).toBe(true);
    expect(observationAgeMs(reported, changedAtMs + 5_000)).toBe(5_000);
  });
});
