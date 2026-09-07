/**
 * The untrusted-content transformation (WCX-06 section 9.1, decision WC-D24).
 *
 * Turns an attacker-influenced string into a list of segments that can only
 * render as text. Everything that could forge what an operator sees becomes
 * visible instead of being obeyed.
 *
 * The order matters and is the order section 9.1 sets: control characters,
 * then directional formatting, then invisible characters, then the length
 * bound. Nothing is normalised — normalising could change evidence, and an
 * operator reading a transcript needs the bytes that arrived, not a tidied
 * version of them.
 */
export type Segment =
  /** Renders as-is. Contains no character this module neutralises. */
  | { kind: 'text'; value: string }
  /** Renders as a visible escape with an accessible description. */
  | { kind: 'escape'; value: string; description: string };

export type Transformed = {
  segments: readonly Segment[];
  /** True when the value was longer than the bound. Never silent. */
  truncated: boolean;
  /** Code points in the original, so the indicator can state the real size. */
  originalLength: number;
  /** True when any character was escaped, so a legend is worth showing. */
  escaped: boolean;
};

/**
 * Bounds, in code points.
 *
 * Section 9.1.5 states them as 512 and 64 KiB. They are applied in code points
 * rather than bytes so a truncation can never split a character in half and
 * invent a replacement glyph that was not in the evidence.
 */
export const TEXT_LIMIT = 512;
export const BLOCK_LIMIT = 65_536;

/**
 * Directional formatting. A right-to-left override can reverse a filename, so
 * `report‮gnp.exe` displays as `reportexe.png` — the classic filename
 * forgery.
 *
 * Keyed by code point rather than by character: these are invisible, and a
 * source file containing them is a source file nobody can review.
 */
const DIRECTIONAL: ReadonlyMap<number, string> = new Map([
  [0x202a, 'left-to-right embedding'],
  [0x202b, 'right-to-left embedding'],
  [0x202c, 'pop directional formatting'],
  [0x202d, 'left-to-right override'],
  [0x202e, 'right-to-left override'],
  [0x2066, 'left-to-right isolate'],
  [0x2067, 'right-to-left isolate'],
  [0x2068, 'first strong isolate'],
  [0x2069, 'pop directional isolate'],
  [0x200e, 'left-to-right mark'],
  [0x200f, 'right-to-left mark'],
]);

/** Invisible characters. They can split a keyword so a reader misses it. */
const INVISIBLE: ReadonlyMap<number, string> = new Map([
  [0x200b, 'zero-width space'],
  [0x200c, 'zero-width non-joiner'],
  [0x200d, 'zero-width joiner'],
  [0x2060, 'word joiner'],
  [0xfeff, 'zero-width no-break space'],
]);

const isControl = (code: number): boolean =>
  (code >= 0x00 && code <= 0x1f) || (code >= 0x7f && code <= 0x9f);

/** `\x1b`, the escaped source an operator can read and search for. */
const escapeSource = (code: number): string =>
  code <= 0xff
    ? `\\x${code.toString(16).padStart(2, '0')}`
    : `\\u${code.toString(16).padStart(4, '0')}`;

const controlName = (code: number): string => {
  if (code === 0x00) return 'null byte';
  if (code === 0x1b) return 'escape character, the start of an ANSI sequence';
  if (code === 0x07) return 'bell character';
  if (code === 0x08) return 'backspace character';
  if (code === 0x0d) return 'carriage return';
  return `control character U+${code.toString(16).toUpperCase().padStart(4, '0')}`;
};

export type TransformOptions = {
  limit: number;
  /** `UntrustedBlock` keeps newline and tab; `UntrustedText` escapes them. */
  allowLineBreaks: boolean;
};

/**
 * Applies the contract.
 *
 * Iterates code points rather than UTF-16 units, so an astral character is one
 * unit of length and an unpaired surrogate is escaped rather than thrown on
 * (section 9.9.1).
 */
export function transformUntrusted(value: string, options: TransformOptions): Transformed {
  const points = [...value];
  const truncated = points.length > options.limit;
  const visible = truncated ? points.slice(0, options.limit) : points;

  const segments: Segment[] = [];
  let buffer = '';
  let escaped = false;

  const flush = (): void => {
    if (buffer === '') return;
    segments.push({ kind: 'text', value: buffer });
    buffer = '';
  };

  const escape = (source: string, description: string): void => {
    flush();
    escaped = true;
    segments.push({ kind: 'escape', value: source, description });
  };

  for (const point of visible) {
    const code = point.codePointAt(0) ?? 0;

    if (options.allowLineBreaks && (point === '\n' || point === '\t')) {
      buffer += point;
      continue;
    }

    if (isControl(code)) {
      escape(escapeSource(code), controlName(code));
      continue;
    }

    const directional = DIRECTIONAL.get(code);
    if (directional !== undefined) {
      escape(escapeSource(code), directional);
      continue;
    }

    const invisible = INVISIBLE.get(code);
    if (invisible !== undefined) {
      escape(escapeSource(code), invisible);
      continue;
    }

    // An unpaired surrogate cannot be rendered and would show as a
    // replacement glyph, which is indistinguishable from one that was
    // actually in the evidence. Escape it instead.
    if (code >= 0xd800 && code <= 0xdfff) {
      escape(escapeSource(code), 'unpaired surrogate');
      continue;
    }

    buffer += point;
  }

  flush();
  return { segments, truncated, originalLength: points.length, escaped };
}
