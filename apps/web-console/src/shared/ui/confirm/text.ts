/**
 * Confirmation text (WCX-04 sections 9.3 and 9.10).
 *
 * Every sentence states the effect in the operator's terms and names the
 * object. The confirm control is labelled with the effect, never `OK`.
 */
export const CONFIRM_TEXT = {
  title: (effect: string) => effect,
  /** Level 2: recoverable, so the sentence says what is lost and what is not. */
  recoverable: (effect: string, object: string) =>
    `${effect}: “${object}”. This removes it from Guardian. Anything Guardian already recorded about it is kept.`,
  /** Level 3: the sentence must say, explicitly, that this cannot be undone. */
  irreversible: (effect: string, object: string) =>
    `${effect}: “${object}”. This cannot be undone.`,
  /** Level 3: typing the exact name is what enables the confirm control. */
  typeToConfirm: (object: string) => `Type the exact name to continue: ${object}`,
  typeLabel: 'Object name',
  cancel: 'Cancel',
  stepUpRequired: 'Step-up reauthentication is required and is not available yet, so this action cannot be completed.',
} as const;
