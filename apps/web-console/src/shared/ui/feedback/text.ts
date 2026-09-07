/**
 * Feedback surface text (WCX-04 sections 9.4 and 9.10).
 */
export const TOAST_TEXT = {
  dismiss: 'Dismiss',
} as const;

export const PENDING_TEXT = {
  /** Long-running work is shown on the object it affects, never in a modal. */
  label: 'In progress',
  detail: (age: string, reason: string) => `Started ${age} ago · ${reason}`,
} as const;
