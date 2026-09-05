/**
 * Token values readable from tests (WCX-03).
 *
 * `semantic.css` is the source of truth for the browser. This module mirrors
 * the resolved values so the contrast and disjointness tests can assert on
 * them without a running browser; `tokens.test.ts` proves the two agree by
 * parsing the CSS.
 */
export const PRIMITIVE = {
  neutral0: '#020e11',
  neutral50: '#071a1f',
  neutral100: '#0b2429',
  neutral200: '#102f34',
  neutral300: '#183c41',
  neutral400: '#23454a',
  neutral500: '#31575c',
  neutral600: '#3b6469',
  neutral700: '#91aaa6',
  neutral800: '#bed0cc',
  neutral900: '#c5d9d5',
  neutral1000: '#eaf5f1',
  azure300: '#8ecdf5',
  azure500: '#4aa8e8',
  green300: '#7cf2bd',
  green500: '#32d38b',
  red400: '#ff837c',
  slate400: '#a8b5c2',
  sev1: '#9cabbb',
  sev2: '#cfc478',
  sev3: '#f0a44f',
  sev4: '#f4855f',
  sev5: '#f48098',
} as const;

/** Groups whose colours must never collide. */
export const BRAND_COLOURS = [PRIMITIVE.azure300, PRIMITIVE.azure500] as const;

export const HEALTH_COLOURS = {
  True: PRIMITIVE.green300,
  False: PRIMITIVE.red400,
  Unknown: PRIMITIVE.slate400,
} as const;

export const SEVERITY_COLOURS = {
  informational: PRIMITIVE.sev1,
  low: PRIMITIVE.sev2,
  medium: PRIMITIVE.sev3,
  high: PRIMITIVE.sev4,
  critical: PRIMITIVE.sev5,
} as const;

/** Alpha of the tone tints in `semantic.css`. */
export const TINT_ALPHA = 0.12;

/**
 * Composite a translucent colour over an opaque one, the way the browser
 * paints it.
 *
 * A status badge paints `--<tone>-surface` — the tone at 12% — over the panel
 * it sits on. Measuring the tone against the bare panel token, as this table
 * first did, measures a background that is never painted and reports a
 * contrast the operator never sees. Measured against the bare panel, severity
 * high and critical reported 4.96 and 5.05; composited over their own 12%
 * tint they were 4.30 and 4.34, both below the 4.5 threshold. Their primitives
 * were lightened until the composited figure cleared it.
 */
export function composite(colour: string, alpha: number, over: string): string {
  const parse = (hex: string) => [1, 3, 5].map((index) => Number.parseInt(hex.slice(index, index + 2), 16));
  const [red, green, blue] = parse(colour);
  const [baseRed, baseGreen, baseBlue] = parse(over);
  const mix = (top: number, bottom: number) => Math.round(alpha * top + (1 - alpha) * bottom);
  return `#${[mix(red!, baseRed!), mix(green!, baseGreen!), mix(blue!, baseBlue!)]
    .map((value) => value.toString(16).padStart(2, '0'))
    .join('')}`;
}

/** The painted background of a tone badge sitting on `surface`. */
const badge = (tone: string, surface: string): string => composite(tone, TINT_ALPHA, surface);

/**
 * Foreground and background pairs that can actually occur, with the WCAG 2.2
 * threshold each must meet. Text is 4.5; a non-text indicator or a large
 * heading is 3.
 *
 * Every background here is what the browser paints, not what the token
 * nominally holds. `WCX-03` shipped the difference as a real axe failure.
 */
export const CONTRAST_PAIRS: readonly {
  name: string;
  foreground: string;
  background: string;
  minimum: number;
}[] = [
  { name: 'primary text on page', foreground: PRIMITIVE.neutral1000, background: PRIMITIVE.neutral50, minimum: 4.5 },
  { name: 'primary text on raised surface', foreground: PRIMITIVE.neutral1000, background: PRIMITIVE.neutral200, minimum: 4.5 },
  { name: 'secondary text on raised surface', foreground: PRIMITIVE.neutral900, background: PRIMITIVE.neutral200, minimum: 4.5 },
  { name: 'muted text on page', foreground: PRIMITIVE.neutral700, background: PRIMITIVE.neutral50, minimum: 4.5 },
  { name: 'muted text on raised surface', foreground: PRIMITIVE.neutral700, background: PRIMITIVE.neutral200, minimum: 4.5 },
  { name: 'muted text on sunken surface', foreground: PRIMITIVE.neutral700, background: PRIMITIVE.neutral100, minimum: 4.5 },
  { name: 'brand accent on page', foreground: PRIMITIVE.azure300, background: PRIMITIVE.neutral50, minimum: 4.5 },
  { name: 'brand accent on raised surface', foreground: PRIMITIVE.azure300, background: PRIMITIVE.neutral200, minimum: 4.5 },
  { name: 'brand mark text on brand mark', foreground: PRIMITIVE.neutral0, background: PRIMITIVE.azure300, minimum: 4.5 },
  { name: 'primary action label', foreground: PRIMITIVE.neutral0, background: PRIMITIVE.azure300, minimum: 4.5 },
  { name: 'secondary action label', foreground: PRIMITIVE.neutral1000, background: PRIMITIVE.neutral300, minimum: 4.5 },
  { name: 'focus ring on page', foreground: PRIMITIVE.azure300, background: PRIMITIVE.neutral50, minimum: 3 },
  { name: 'focus ring on raised surface', foreground: PRIMITIVE.azure300, background: PRIMITIVE.neutral200, minimum: 3 },
  { name: 'health true badge on raised surface', foreground: PRIMITIVE.green300, background: badge(PRIMITIVE.green300, PRIMITIVE.neutral200), minimum: 4.5 },
  { name: 'health false badge on raised surface', foreground: PRIMITIVE.red400, background: badge(PRIMITIVE.red400, PRIMITIVE.neutral200), minimum: 4.5 },
  { name: 'health unknown badge on raised surface', foreground: PRIMITIVE.slate400, background: badge(PRIMITIVE.slate400, PRIMITIVE.neutral200), minimum: 4.5 },
  { name: 'severity informational badge', foreground: PRIMITIVE.sev1, background: badge(PRIMITIVE.sev1, PRIMITIVE.neutral200), minimum: 4.5 },
  { name: 'severity low badge', foreground: PRIMITIVE.sev2, background: badge(PRIMITIVE.sev2, PRIMITIVE.neutral200), minimum: 4.5 },
  { name: 'severity medium badge', foreground: PRIMITIVE.sev3, background: badge(PRIMITIVE.sev3, PRIMITIVE.neutral200), minimum: 4.5 },
  { name: 'severity high badge', foreground: PRIMITIVE.sev4, background: badge(PRIMITIVE.sev4, PRIMITIVE.neutral200), minimum: 4.5 },
  { name: 'severity critical badge', foreground: PRIMITIVE.sev5, background: badge(PRIMITIVE.sev5, PRIMITIVE.neutral200), minimum: 4.5 },
  { name: 'health true badge on sunken surface', foreground: PRIMITIVE.green300, background: badge(PRIMITIVE.green300, PRIMITIVE.neutral100), minimum: 4.5 },
  { name: 'health false badge on sunken surface', foreground: PRIMITIVE.red400, background: badge(PRIMITIVE.red400, PRIMITIVE.neutral100), minimum: 4.5 },
  { name: 'health unknown badge on sunken surface', foreground: PRIMITIVE.slate400, background: badge(PRIMITIVE.slate400, PRIMITIVE.neutral100), minimum: 4.5 },
  // Error text and the blocking banner paint the health-false tone as plain
  // text, with no tint behind it, so they need their own pairs.
  { name: 'error text on raised surface', foreground: PRIMITIVE.red400, background: PRIMITIVE.neutral200, minimum: 4.5 },
  { name: 'error text on page', foreground: PRIMITIVE.red400, background: PRIMITIVE.neutral50, minimum: 4.5 },
  { name: 'notice accent on notice surface', foreground: PRIMITIVE.azure300, background: composite(PRIMITIVE.azure500, 0.1, PRIMITIVE.neutral200), minimum: 4.5 },
  { name: 'secret label on page', foreground: PRIMITIVE.sev3, background: PRIMITIVE.neutral50, minimum: 4.5 },
  { name: 'config complete label', foreground: PRIMITIVE.neutral1000, background: PRIMITIVE.neutral300, minimum: 4.5 },
  { name: 'config pending label', foreground: PRIMITIVE.neutral900, background: PRIMITIVE.neutral500, minimum: 4.5 },
  { name: 'device pending label', foreground: PRIMITIVE.neutral900, background: PRIMITIVE.neutral400, minimum: 4.5 },
  { name: 'device active label', foreground: PRIMITIVE.neutral1000, background: PRIMITIVE.neutral500, minimum: 4.5 },
  { name: 'device disabled label', foreground: PRIMITIVE.neutral800, background: PRIMITIVE.neutral300, minimum: 4.5 },
  { name: 'device revoked label', foreground: PRIMITIVE.neutral1000, background: PRIMITIVE.neutral600, minimum: 4.5 },
  { name: 'confidence step', foreground: PRIMITIVE.neutral1000, background: PRIMITIVE.neutral200, minimum: 3 },
  { name: 'reauthentication banner text', foreground: PRIMITIVE.neutral0, background: PRIMITIVE.sev3, minimum: 4.5 },
];

const channel = (value: number): number => {
  const scaled = value / 255;
  return scaled <= 0.03928 ? scaled / 12.92 : ((scaled + 0.055) / 1.055) ** 2.4;
};

/** Relative luminance per WCAG 2.2. */
export function luminance(hex: string): number {
  const value = hex.replace('#', '');
  const red = Number.parseInt(value.slice(0, 2), 16);
  const green = Number.parseInt(value.slice(2, 4), 16);
  const blue = Number.parseInt(value.slice(4, 6), 16);
  return 0.2126 * channel(red) + 0.7152 * channel(green) + 0.0722 * channel(blue);
}

/** Contrast ratio between two opaque colours. */
export function contrastRatio(foreground: string, background: string): number {
  const first = luminance(foreground);
  const second = luminance(background);
  const lighter = Math.max(first, second);
  const darker = Math.min(first, second);
  return (lighter + 0.05) / (darker + 0.05);
}
