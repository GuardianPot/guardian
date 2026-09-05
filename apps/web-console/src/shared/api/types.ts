import type { components } from '@generated/openapi';

/**
 * Domain types derived from the approved OpenAPI contract (RE-10, WC-D02).
 *
 * Nothing here re-declares a field, union, or enum. A schema removed or
 * renamed in `openapi/guardian.yaml` fails typecheck here rather than drifting
 * silently. Where the console needs a narrower view, derive it with `Pick`,
 * `Omit`, or a mapping function — never by re-typing.
 */
type Schemas = components['schemas'];

export type Session = Schemas['AuthSession'];
export type SessionCredentials = Schemas['AuthSessionCredentials'];
export type Environment = Schemas['Environment'];
export type Zone = Schemas['Zone'];
export type Device = Schemas['DeviceInventory'];
export type DeviceState = Device['state'];
export type EnrollmentSecret = Schemas['EnrollmentTokenSecret'];
export type HealthView = Schemas['HealthView'];
export type HealthCondition = Schemas['HealthCondition'];
export type HealthConditionType = Schemas['HealthConditionType'];
export type HealthStatus = Schemas['HealthStatus'];
export type StatusResponse = Schemas['StatusResponse'];
