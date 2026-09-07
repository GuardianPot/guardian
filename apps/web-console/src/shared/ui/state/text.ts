/**
 * Operator-facing text for the eight data states (WCX-04 sections 9.1, 9.10).
 *
 * One constant object per state component so `WCX-08` can move each object
 * into the catalogue without renaming a key.
 *
 * The single rule that governs every string here: **no state may read as
 * healthy, complete, or successful.** Missing data, refused access, an
 * impaired dependency, and a stale read are each stated as what they are. The
 * word "healthy" appears exactly once, inside the phrase that denies it, and
 * `states.test.tsx` asserts that negatively for all eight.
 */
export const LOADING_TEXT = {
  /** `activity` names what is loading, so the announcement is not a bare spinner. */
  announce: (activity: string) => `${activity}…`,
} as const;

export const EMPTY_TEXT = {
  heading: (collection: string) => `No ${collection} recorded`,
  body: (collection: string) =>
    `Guardian read this scope and found zero ${collection}. That is a confirmed count, not a failed read, and it reports nothing about whether anything is working.`,
} as const;

export const UNKNOWN_TEXT = {
  heading: 'No observation exists',
  body: (subject: string) =>
    `Guardian holds no observation of ${subject}. Absence of an observation is not a healthy signal and it is not a failure signal.`,
  source: (source: string) => `An observation would appear once ${source}.`,
} as const;

export const STALE_TEXT = {
  heading: 'Showing the last data Guardian received',
  age: (age: string) => `Observed ${age} ago. It may no longer match the environment.`,
  reason: (reason: string) => `Refresh is not current: ${reason}`,
} as const;

export const PARTIAL_TEXT = {
  heading: 'Some sources could not be read',
  body: 'What is shown below came from the sources that answered. The rest is missing, not empty.',
  listLabel: 'Sources that could not be read',
  retry: 'Retry the missing sources',
} as const;

export const DEGRADED_TEXT = {
  heading: (dependency: string) => `${dependency} is impaired`,
  works: (detail: string) => `Still answering: ${detail}`,
  broken: (detail: string) => `Not answering: ${detail}`,
  retry: 'Try again',
} as const;

export const DENIED_TEXT = {
  heading: 'Access was refused',
  /**
   * Deliberately says nothing about what exists. A refusal that leaked "no
   * devices" or "3 devices" would answer the question the refusal withheld.
   */
  body: 'Guardian refused access to this data. A refusal is not an absence: it reports nothing about what is or is not here.',
  reauthenticate: 'Re-authenticate and try again.',
} as const;

export const ERROR_TEXT = {
  heading: 'This data could not be loaded',
  /** Fixed. No exception text, no status code, no request or response detail. */
  body: 'Guardian could not finish this read. Nothing about the underlying failure is shown here.',
  retry: 'Try again',
} as const;
