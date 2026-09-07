import type { GlyphShape, StatusEncoding } from '@shared/theme/statusEncoding';
import styles from '@shared/styles/app.module.css';

/**
 * The `WCX-03` status encoding contract, rendered (decision WC-D11).
 *
 * Three channels always travel together: a glyph shape, the operator-facing
 * label as real text, and the tone class. Colour alone never carries the
 * meaning, so the value survives colour blindness, monochrome print, and a
 * stylesheet that failed to load.
 *
 * The badge takes an encoding, never a raw string and never a class name. A
 * screen therefore cannot restyle a status indicator or invent a tone: an
 * unrecognised backend value has already resolved to the unknown treatment in
 * `statusEncoding`, and there is no prop that could override it.
 */
const GLYPH_PATHS: Readonly<Record<GlyphShape, string>> = {
  circle: 'M6 1a5 5 0 1 0 0 10A5 5 0 0 0 6 1Z',
  square: 'M1.4 1.4h9.2v9.2H1.4Z',
  diamond: 'M6 .7 11.3 6 6 11.3.7 6Z',
  triangle: 'M6 1.1 11.5 10.9H.5Z',
  pentagon: 'M6 .9 11.2 4.7 9.2 10.9H2.8L.8 4.7Z',
  hexagon: 'M3.4 1.1h5.2L11.2 6l-2.6 4.9H3.4L.8 6Z',
  bar: 'M.8 4.4h10.4v3.2H.8Z',
  ring: 'M6 1a5 5 0 1 0 0 10A5 5 0 0 0 6 1Zm0 2.3A2.7 2.7 0 1 1 6 8.7 2.7 2.7 0 0 1 6 3.3Z',
  cross: 'M2.3.9 6 4.6 9.7.9 11.1 2.3 7.4 6l3.7 3.7-1.4 1.4L6 7.4l-3.7 3.7-1.4-1.4L4.6 6 .9 2.3Z',
  slash: 'M9.1.8 11.2 2.9 2.9 11.2.8 9.1Z',
};

export function StatusGlyph({ shape }: { shape: GlyphShape }) {
  return (
    <svg className={styles.statusGlyph} viewBox="0 0 12 12" aria-hidden="true" focusable="false">
      <path d={GLYPH_PATHS[shape]} fillRule="evenodd" />
    </svg>
  );
}

export type StatusBadgeProps = {
  encoding: StatusEncoding;
  /**
   * Names the dimension the badge reports, so an operator cannot mistake an
   * inventory badge for a health badge. Rendered before the label.
   */
  dimension?: string;
};

export function StatusBadge({ encoding, dimension }: StatusBadgeProps) {
  return (
    <span className={`${styles.statusBadge} ${styles[encoding.tone] ?? ''}`}>
      <StatusGlyph shape={encoding.glyph} />
      {dimension ? `${dimension}: ${encoding.label}` : encoding.label}
    </span>
  );
}
