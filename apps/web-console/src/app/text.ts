/**
 * Application shell text (WCX-04 section 9.10).
 *
 * Declared as catalogue-ready constants so `WCX-08` can move them without
 * renaming. The strings themselves are unchanged from `P1-W11`, because the
 * browser suite asserts several of them and the migration must not change what
 * an operator reads.
 */
export const SHELL_TEXT = {
  product: 'Guardian',
  productScope: 'Control Plane',
  homeLabel: 'Guardian Console home',
  skipToContent: 'Skip to content',
  primaryNavigation: 'Primary navigation',
  environments: 'Environments',
  signedInAs: 'Signed in as',
  signOut: 'Sign out',
  signOutFailed: 'Sign-out failed. This session is still active.',
  reauthenticate: 'Re-authenticate',
  readOnlySession: 'Read-only session restored.',
  beforeChanging: 'before changing configuration or signing out.',
  checkingSession: 'Checking your session',
  /** Shown while a route's chunk is still arriving (WCX-07 section 9.2.1). */
  loadingScreen: 'Loading this screen',
} as const;
