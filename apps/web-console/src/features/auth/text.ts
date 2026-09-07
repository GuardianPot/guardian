/**
 * Sign-in text (WCX-04 section 9.10).
 *
 * The browser suite drives this screen by label and button name, so every
 * string here is unchanged from `P1-W11`.
 */
export const LOGIN_TEXT = {
  checkingSession: 'Checking your session',
  brandEyebrow: 'Guardian Control Plane',
  headlineFirst: 'Know what is protected.',
  headlineSecond: 'Know what needs attention.',
  intro: 'Device inventory and health remain separate, explicit signals—never an inferred green light.',
  restoreEyebrow: 'Restore mutation access',
  ownerEyebrow: 'Owner access',
  reauthenticateHeading: 'Re-authenticate',
  signInHeading: 'Sign in',
  reauthenticateIntro: 'Your cookie session is active, but its CSRF proof was intentionally not persisted.',
  signInIntro: 'Use your local owner credentials and one MFA method.',
  methodGroup: 'MFA method',
  authenticator: 'Authenticator',
  recovery: 'Recovery code',
  username: 'Username',
  password: 'Password',
  totpLabel: '6-digit authenticator code',
  recoveryLabel: 'Recovery code',
  submit: 'Continue securely',
  rateLimited: 'Too many attempts. Wait before trying again.',
  denied: 'Sign-in was denied. Check your credentials and MFA proof.',
} as const;
