/**
 * The permanent hostile-content corpus (WCX-06 section 9.2, `RE-12`).
 *
 * These are the strings Guardian expects to be handed by whoever attacked the
 * network: they arrive as decoy display names, SSH transcripts, HTTP bodies,
 * captured filenames, and credential attempts. Every one is inert — no working
 * exploit, no real credential, no third party's hostname — because their job
 * is to prove the renderer neutralises them, not to attack anything.
 *
 * **This file only grows.** Every rendering defect found from here on adds a
 * fixture, so the same defect can never return unnoticed. Removing an entry
 * means deciding that a class of hostile content no longer needs proving,
 * which is an owner decision rather than a cleanup.
 *
 * Every control, directional, and invisible character is built with `cp()`
 * rather than typed literally. A corpus containing literal invisible bytes is
 * a corpus nobody can review, and reviewing it is the entire point —
 * `corpus.test.ts` asserts this file's own source contains none of them.
 */
export type HostileFixture = {
  /** Stable id. Test failures name it, so it must not be renamed casually. */
  id: string;
  /** What an attacker is trying to achieve. */
  intent: string;
  value: string;
};

/** Builds a character from its code point, so the source stays readable. */
const cp = (code: number): string => String.fromCodePoint(code);

const ESC = cp(0x1b); // starts every ANSI sequence
const BEL = cp(0x07);
const NUL = cp(0x00);
const RLO = cp(0x202e); // right-to-left override
const RLI = cp(0x2067); // right-to-left isolate
const PDI = cp(0x2069); // pop directional isolate
const ZWSP = cp(0x200b); // zero-width space

export const HOSTILE_CORPUS: readonly HostileFixture[] = [
  {
    id: 'script-element',
    intent: 'execute script by closing out of a text context',
    value: '<script>alert(document.cookie)</script>',
  },
  {
    id: 'event-handler',
    intent: 'execute script through an element event handler',
    value: '<img src=x onerror=alert(1)>',
  },
  {
    id: 'attribute-break',
    intent: 'escape an attribute value and add a handler',
    value: '"><svg onload=alert(1)>',
  },
  {
    id: 'nested-markup',
    intent: 'survive a naive single-pass tag stripper',
    value: '<scr<script>ipt>alert(1)</scr</script>ipt>',
  },
  {
    id: 'javascript-url',
    intent: 'become a clickable link that executes script',
    value: "javascript:alert('xss')",
  },
  {
    id: 'data-url',
    intent: 'become a link to an attacker-authored document',
    value: 'data:text/html;base64,PHNjcmlwdD5hbGVydCgxKTwvc2NyaXB0Pg==',
  },
  {
    id: 'ansi-colour',
    intent: 'repaint a transcript so a failure reads as a success',
    value: `${ESC}[32mALL CHECKS PASSED${ESC}[0m`,
  },
  {
    id: 'ansi-cursor',
    intent: 'move the cursor to overwrite text already on screen',
    value: `denied${ESC}[2K${ESC}[1Agranted`,
  },
  {
    id: 'ansi-title',
    intent: 'set the terminal window title from captured content',
    value: `${ESC}]0;guardian-admin@control-plane${BEL}`,
  },
  {
    // `report<RLO>gnp.exe` displays as `reportexe.png` if the override is obeyed.
    id: 'rtl-override-filename',
    intent: 'reverse a filename so an executable reads as an image',
    value: `report${RLO}gnp.exe`,
  },
  {
    id: 'bidi-isolate',
    intent: 'reorder a line using isolates rather than an override',
    value: `admin ${RLI}denied${PDI} access`,
  },
  {
    id: 'zero-width-keyword',
    intent: 'split a keyword so a reader or a search misses it',
    value: `pass${ZWSP}word`,
  },
  {
    id: 'long-token',
    intent: 'break a layout with a single unbreakable token',
    value: `A${'a'.repeat(4_000)}`,
  },
  {
    id: 'null-byte',
    intent: 'truncate a value early in a consumer that treats it as a C string',
    value: `safe${NUL} /etc/shadow`,
  },
  {
    id: 'crlf-injection',
    intent: 'inject a header or a log line through a carriage return',
    value: 'edge-one\r\nX-Injected: yes',
  },
  {
    id: 'markdown-link',
    intent: 'become a link if the value is ever rendered as Markdown',
    value: '[click here](javascript:alert(1))',
  },
  {
    id: 'markdown-image',
    intent: 'trigger a request if the value is ever rendered as Markdown',
    value: '![](https://attacker.invalid/pixel.png)',
  },
  {
    id: 'path-traversal-filename',
    intent: 'escape a directory if the filename is ever used as a path',
    value: '../../../../etc/passwd',
  },
  {
    id: 'trailing-space-filename',
    intent: 'hide the real extension behind trailing whitespace',
    value: 'invoice.pdf                    .exe ',
  },
  {
    id: 'double-extension-filename',
    intent: 'read as a document while being an executable',
    value: 'quarterly-report.pdf.scr',
  },
  {
    // Deliberately not shaped like any real provider's key. The repository
    // secret scan would flag one that was, and a fixture that trips the
    // scanner teaches people to ignore the scanner.
    id: 'credential-looking',
    intent: 'be mistaken for a real secret and handled as one',
    value: 'Authorization: Bearer not-a-real-token-0000000000000000',
  },
  {
    id: 'unpaired-surrogate',
    intent: 'crash a transformer that assumes well-formed UTF-16',
    value: `edge${cp(0xd800)}one`,
  },
  {
    id: 'template-expression',
    intent: 'be evaluated by a template engine that interpolates data',
    value: '{{constructor.constructor("alert(1)")()}}',
  },
  {
    id: 'style-injection',
    intent: 'hide page content by injecting a stylesheet',
    value: '</strong><style>body{display:none}</style>',
  },
];

/** Every fixture value, for tests that only need the strings. */
export const HOSTILE_VALUES: readonly string[] = HOSTILE_CORPUS.map((fixture) => fixture.value);

/** Elements no Guardian surface ever creates from data. */
export const FORBIDDEN_ELEMENTS =
  'script, iframe, object, embed, style, link, img, svg, form, a, video, audio, source, base, meta';

/** Attributes that would turn a captured value into a request or an action. */
export const FORBIDDEN_ATTRIBUTES = [
  'href',
  'src',
  'srcset',
  'action',
  'formaction',
  'poster',
  'data',
  'download',
  'ping',
  'background',
];
