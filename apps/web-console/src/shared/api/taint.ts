import { untrusted } from './untrusted';
import type {
  Device,
  DeviceRaw,
  EnrollmentSecret,
  EnrollmentSecretRaw,
  EnrollmentToken,
  EnrollmentTokenRaw,
  Environment,
  EnvironmentRaw,
  HealthCondition,
  HealthConditionRaw,
  HealthView,
  HealthViewRaw,
  Zone,
  ZoneRaw,
} from './types';

/**
 * The trust boundary, as functions (WCX-06 section 9.8).
 *
 * Every response body crosses exactly one of these on its way from the
 * transport into a feature's query. After that point the free-text fields are
 * `Untrusted` and the compiler will not let a screen render one directly.
 *
 * These deliberately rebuild the object rather than casting the whole thing.
 * A cast would keep working when the contract gains a new free-text field;
 * rebuilding means the new field arrives untyped-as-untrusted and someone has
 * to decide which side of the boundary it belongs on.
 */
export function taintEnvironment(raw: EnvironmentRaw): Environment {
  const { display_name: displayName, ...rest } = raw;
  return { ...rest, display_name: untrusted(displayName) };
}

export function taintZone(raw: ZoneRaw): Zone {
  const { display_name: displayName, ...rest } = raw;
  return { ...rest, display_name: untrusted(displayName) };
}

export function taintDevice(raw: DeviceRaw): Device {
  const { display_name: displayName, ...rest } = raw;
  return { ...rest, display_name: untrusted(displayName) };
}

export function taintEnrollmentSecret(raw: EnrollmentSecretRaw): EnrollmentSecret {
  const { device_name: deviceName, ...rest } = raw;
  return { ...rest, device_name: untrusted(deviceName) };
}

export function taintEnrollmentToken(raw: EnrollmentTokenRaw): EnrollmentToken {
  const { device_name: deviceName, ...rest } = raw;
  return { ...rest, device_name: untrusted(deviceName) };
}

/**
 * The tainted keys are destructured out before the spread rather than
 * overwritten after it. Spreading first leaves the raw `string` type in the
 * result and unions it with the marked one, which under
 * `exactOptionalPropertyTypes` produces a shape that satisfies neither.
 */
function taintCondition(raw: HealthConditionRaw): HealthCondition {
  const { reason, message, source_device_id: sourceDeviceID, ...rest } = raw;
  return {
    ...rest,
    reason: untrusted(reason),
    message: untrusted(message),
    ...(sourceDeviceID === undefined ? {} : { source_device_id: untrusted(sourceDeviceID) }),
  };
}

export function taintHealthView(raw: HealthViewRaw): HealthView {
  const { aggregate, conditions, ...rest } = raw;
  const { reason, blocking_device_id: blockingDeviceID, ...aggregateRest } = aggregate;
  return {
    ...rest,
    aggregate: {
      ...aggregateRest,
      ...(reason === undefined ? {} : { reason: untrusted(reason) }),
      ...(blockingDeviceID === undefined ? {} : { blocking_device_id: untrusted(blockingDeviceID) }),
    },
    conditions: conditions.map(taintCondition),
  };
}
