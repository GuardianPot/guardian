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
  sev1: '#93a4b5',
  sev2: '#cfc478',
  sev3: '#f0a44f',
  sev4: '#f2734a',
  sev5: '#f2708a',
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

/**
 * Foreground and background pairs that can actually occur, with the WCAG 2.2
 * threshold each must meet. Text is 4.5; a non-text indicator or a large
 * heading is 3.
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
  { name: 'health true on raised surface', foreground: PRIMITIVE.green300, background: PRIMITIVE.neutral200, minimum: 4.5 },
  { name: 'health false on raised surface', foreground: PRIMITIVE.red400, background: PRIMITIVE.neutral200, minimum: 4.5 },
  { name: 'health unknown on raised surface', foreground: PRIMITIVE.slate400, background: PRIMITIVE.neutral200, minimum: 4.5 },
  { name: 'severity informational on raised surface', foreground: PRIMITIVE.sev1, background: PRIMITIVE.neutral200, minimum: 4.5 },
  { name: 'severity low on raised surface', foreground: PRIMITIVE.sev2, background: PRIMITIVE.neutral200, minimum: 4.5 },
  { name: 'severity medium on raised surface', foreground: PRIMITIVE.sev3, background: PRIMITIVE.neutral200, minimum: 4.5 },
  { name: 'severity high on raised surface', foreground: PRIMITIVE.sev4, background: PRIMITIVE.neutral200, minimum: 4.5 },
  { name: 'severity critical on raised surface', foreground: PRIMITIVE.sev5, background: PRIMITIVE.neutral200, minimum: 4.5 },
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
