/**
 * Untrusted-content text (WCX-06 sections 9.6 and 9.10).
 *
 * Deliberately calm. Escaped content is not an incident — it is ordinary in a
 * deception product — so the wording explains rather than alarms, and section
 * 9.6 forbids using a severity colour for it.
 */
export const UNTRUSTED_TEXT = {
  legend: 'Characters shown as \\xNN or \\uNNNN were control or invisible characters in the captured value. They are displayed, never interpreted.',
  escapeLabel: (description: string) => `escaped ${description}`,
  truncatedText: (shown: number, original: number) =>
    `Showing the first ${shown} of ${original} characters.`,
  truncatedBlock: (shown: number, original: number) =>
    `Showing the first ${shown} of ${original} characters. Copy to get the whole captured value.`,
  copy: 'Copy the original untrusted value',
  copied: 'Copied the original value to the clipboard.',
  copyFailed: 'The clipboard is unavailable, so nothing was copied.',
  blockLabel: 'Captured content',
} as const;
