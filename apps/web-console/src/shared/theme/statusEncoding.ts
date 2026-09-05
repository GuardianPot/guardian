/**
 * Status encoding contract (WCX-03, decision WC-D11).
 *
 * Colour is never the only channel. Every health, severity, device-state and
 * configuration indicator resolves to a glyph shape, a text label, and a class
 * name, so the value survives colour blindness, monochrome print, and a
 * failed stylesheet.
 *
 * Each lookup is total over a closed union and falls back to the unknown
 * treatment. An unrecognised value must never resolve to a healthy, complete,
 * or low-severity presentation.
 */
export type GlyphShape =
  | 'circle'
  | 'square'
  | 'diamond'
  | 'triangle'
  | 'pentagon'
  | 'hexagon'
  | 'bar'
  | 'ring'
  | 'cross'
  | 'slash';

export type StatusEncoding = {
  /** Shape rendered as an inline SVG beside the label. */
  glyph: GlyphShape;
  /** Operator-facing text. `WCX-08` moves these into the catalogue. */
  label: string;
  /** CSS Module class suffix, resolved against the module by the caller. */
  tone: string;
};

export type HealthStatus = 'True' | 'False' | 'Unknown';
export type Severity = 'informational' | 'low' | 'medium' | 'high' | 'critical';
export type Confidence = 'Low' | 'Medium' | 'High';
export type DeviceState = 'pending' | 'active' | 'disabled' | 'revoked';
export type ConfigState = 'needs_zones' | 'zones_defined';

const HEALTH: Readonly<Record<HealthStatus, StatusEncoding>> = {
  True: { glyph: 'circle', label: 'Healthy', tone: 'healthTrue' },
  False: { glyph: 'square', label: 'Action required', tone: 'healthFalse' },
  Unknown: { glyph: 'diamond', label: 'Unknown', tone: 'healthUnknown' },
};

const SEVERITY: Readonly<Record<Severity, StatusEncoding>> = {
  informational: { glyph: 'circle', label: 'Informational', tone: 'severityInformational' },
  low: { glyph: 'triangle', label: 'Low', tone: 'severityLow' },
  medium: { glyph: 'square', label: 'Medium', tone: 'severityMedium' },
  high: { glyph: 'pentagon', label: 'High', tone: 'severityHigh' },
  critical: { glyph: 'hexagon', label: 'Critical', tone: 'severityCritical' },
};

const DEVICE: Readonly<Record<DeviceState, StatusEncoding>> = {
  pending: { glyph: 'ring', label: 'pending', tone: 'devicePending' },
  active: { glyph: 'circle', label: 'active', tone: 'deviceActive' },
  disabled: { glyph: 'slash', label: 'disabled', tone: 'deviceDisabled' },
  revoked: { glyph: 'cross', label: 'revoked', tone: 'deviceRevoked' },
};

const CONFIG: Readonly<Record<ConfigState, StatusEncoding>> = {
  zones_defined: { glyph: 'circle', label: 'Configured', tone: 'configComplete' },
  needs_zones: { glyph: 'ring', label: 'Needs zones', tone: 'configPending' },
};

/**
 * The single fallback. An unrecognised value is unknown, never healthy.
 */
export const UNKNOWN_ENCODING: StatusEncoding = HEALTH.Unknown;

const resolve = <Key extends string>(
  table: Readonly<Record<Key, StatusEncoding>>,
  value: string,
): StatusEncoding =>
  Object.prototype.hasOwnProperty.call(table, value)
    ? table[value as Key]
    : UNKNOWN_ENCODING;

export const healthEncoding = (value: string): StatusEncoding => resolve(HEALTH, value);
export const severityEncoding = (value: string): StatusEncoding => resolve(SEVERITY, value);
export const deviceEncoding = (value: string): StatusEncoding => resolve(DEVICE, value);
export const configEncoding = (value: string): StatusEncoding => resolve(CONFIG, value);

/** Severity in ascending order, so a caller never invents an ordering. */
export const SEVERITY_ORDER: readonly Severity[] = [
  'informational',
  'low',
  'medium',
  'high',
  'critical',
];

/**
 * Confidence is a three-step neutral indicator. `WC-D11` forbids expressing it
 * with colour, so this returns a filled-step count and a label, never a tone.
 */
export const CONFIDENCE_ORDER: readonly Confidence[] = ['Low', 'Medium', 'High'];

export type ConfidenceEncoding = { filled: number; total: number; label: string };

export function confidenceEncoding(value: string): ConfidenceEncoding {
  const index = CONFIDENCE_ORDER.indexOf(value as Confidence);
  if (index < 0) return { filled: 0, total: CONFIDENCE_ORDER.length, label: 'Unknown' };
  return { filled: index + 1, total: CONFIDENCE_ORDER.length, label: value };
}

/** Every table, for exhaustiveness tests. */
export const ENCODING_TABLES = { HEALTH, SEVERITY, DEVICE, CONFIG } as const;
