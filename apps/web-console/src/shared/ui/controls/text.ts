/**
 * Operator-facing text for the base control set (WCX-04 section 9.10).
 *
 * One constant object per component so `WCX-08` can move each object into the
 * catalogue without renaming a key or touching a component. No component in
 * this directory writes a literal string.
 */
export const BUTTON_TEXT = {
  /** Marks a pending control. The label itself never changes, so the
   *  accessible name an operator or a script targets stays stable. */
  pending: 'Working…',
} as const;

export const CONFIDENCE_METER_TEXT = {
  label: 'Confidence',
  /** Confidence is never colour-coded, so the value is also plain text. */
  value: (label: string, filled: number, total: number) =>
    `${label} — ${filled} of ${total}`,
} as const;

export const SKELETON_TEXT = {
  /** A skeleton is a placeholder shape only. `LoadingState` announces the
   *  activity, so the skeleton itself is hidden from assistive technology
   *  rather than announced twice. */
  role: 'presentation',
} as const;

export const DIALOG_TEXT = {
  cancel: 'Cancel',
} as const;
