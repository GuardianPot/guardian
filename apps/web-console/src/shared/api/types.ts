import type { components } from '@generated/openapi';
import type { Untrusted } from './untrusted';

/**
 * Domain types derived from the approved OpenAPI contract (RE-10, WC-D02).
 *
 * Nothing here re-declares a field, union, or enum. A schema removed or
 * renamed in `openapi/guardian.yaml` fails typecheck here rather than drifting
 * silently. Where the console needs a narrower view, derive it with `Pick`,
 * `Omit`, or a mapping function — never by re-typing.
 *
 * Free-text fields are re-typed as `Untrusted` (WCX-06 section 9.8). That is
 * the one deliberate departure from "never re-type": the generated contract
 * says these are strings, and they are, but the console must not be able to
 * render one without routing it through the untrusted contract. The mapping
 * happens once, in each feature's API module, so a screen receives values that
 * are already marked.
 *
 * Which fields, and why:
 *
 * - health `reason` and `message` come from the device, and a compromised or
 *   emulated Edge writes them directly;
 * - device and source identifiers are device-supplied strings, not validated
 *   identifiers, wherever they appear in a projection;
 * - display names round-trip through the API. The console cannot tell an
 *   operator's typing from an attacker with a stolen session, so it does not
 *   try.
 *
 * Values the backend validates into a known shape — UUIDs it issues, CIDRs,
 * timestamps, enum states — stay plain strings. Marking them would make the
 * brand mean "string" rather than "untrusted".
 */
type Schemas = components['schemas'];

/**
 * The tainted views are written out rather than produced by a generic helper.
 * A mapped type over a subset of keys does not reliably carry the optional
 * modifier across, and under `exactOptionalPropertyTypes` a lost `?` turns an
 * absent `source_device_id` into a type error at every construction site.
 * Six explicit declarations are shorter than the helper that would keep them
 * honest, and they read as what they are: a list of what Guardian does not
 * trust.
 */
export type Session = Schemas['AuthSession'];
export type SessionCredentials = Schemas['AuthSessionCredentials'];

export type EnvironmentRaw = Schemas['Environment'];
export type Environment = Omit<EnvironmentRaw, 'display_name'> & { display_name: Untrusted };

export type ZoneRaw = Schemas['Zone'];
export type Zone = Omit<ZoneRaw, 'display_name'> & { display_name: Untrusted };

export type DeviceRaw = Schemas['DeviceInventory'];
export type Device = Omit<DeviceRaw, 'display_name'> & { display_name: Untrusted };
export type DeviceState = DeviceRaw['state'];

export type EnrollmentSecretRaw = Schemas['EnrollmentTokenSecret'];
export type EnrollmentSecret = Omit<EnrollmentSecretRaw, 'device_name'> & { device_name: Untrusted };

/**
 * A token summary carries a device name and never the token value. The
 * contract has no field for one here, which is the guarantee itself: the
 * console cannot list a secret it is never sent.
 */
export type EnrollmentTokenRaw = Schemas['EnrollmentTokenSummary'];
export type EnrollmentToken = Omit<EnrollmentTokenRaw, 'device_name'> & { device_name: Untrusted };

export type HealthConditionRaw = Schemas['HealthCondition'];
export type HealthCondition = Omit<HealthConditionRaw, 'reason' | 'message' | 'source_device_id'> & {
  reason: Untrusted;
  message: Untrusted;
  source_device_id?: Untrusted;
};

export type HealthViewRaw = Schemas['HealthView'];
export type HealthAggregateRaw = HealthViewRaw['aggregate'];
export type HealthAggregate = Omit<HealthAggregateRaw, 'reason' | 'blocking_device_id'> & {
  reason?: Untrusted;
  blocking_device_id?: Untrusted;
};
export type HealthView = Omit<HealthViewRaw, 'aggregate' | 'conditions'> & {
  readonly aggregate: HealthAggregate;
  readonly conditions: readonly HealthCondition[];
};

export type HealthConditionType = Schemas['HealthConditionType'];
export type HealthStatus = Schemas['HealthStatus'];
export type StatusResponse = Schemas['StatusResponse'];

/**
 * The decoy contract (P2-W15), consumed and never defined here.
 *
 * Two fields are free text and both are marked. `display_name` is
 * operator-supplied and round-trips through the API; a condition `message` is
 * written by an Edge, which is the component closest to an attacker. Everything
 * else in a decoy — family, persona, interaction level, desired and observed
 * state — is a closed token the console may treat as a category.
 *
 * `reason` is bounded to `^[a-z][a-z0-9_]{0,63}$` by the contract, but it is
 * still a device-supplied string chosen by the Edge, so it is marked too. The
 * pattern is what makes it safe to display; the marking is what makes that a
 * decision rather than an assumption.
 */
export type DecoyRaw = Schemas['Decoy'];
export type Decoy = Omit<DecoyRaw, 'display_name'> & { display_name: Untrusted };

export type DecoyConditionRaw = Schemas['DecoyCondition'];
export type DecoyCondition = Omit<DecoyConditionRaw, 'reason' | 'message'> & {
  reason: Untrusted;
  message: Untrusted;
};

export type DecoyObservationRaw = Schemas['DecoyObservation'];
export type DecoyObservation = Omit<DecoyObservationRaw, 'conditions'> & {
  readonly conditions: readonly DecoyCondition[];
};

export type DecoyViewRaw = Schemas['DecoyView'];
export type DecoyView = { decoy: Decoy; observed: DecoyObservation };

export type DecoyFamily = Schemas['DecoyFamily'];
export type DecoyPersona = Schemas['DecoyPersona'];
export type DecoyDesiredState = Schemas['DecoyDesiredState'];
export type DecoyObservedState = Schemas['DecoyObservedState'];
export type DecoyConditionType = Schemas['DecoyConditionType'];
export type DecoyWriteRequest = Schemas['DecoyWriteRequest'];
