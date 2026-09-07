/**
 * Environment screen text (WCX-04 section 9.10).
 *
 * Catalogue-ready constants, one object per screen, so `WCX-08` can move them
 * without renaming. Strings the browser suite asserts are unchanged.
 */
export const ENVIRONMENTS_TEXT = {
  eyebrow: 'Organization workspace',
  heading: 'Environments',
  truthNote: 'Configuration is not health',
  listHeading: 'Configured environments',
  collection: 'environments',
  total: (count: number) => `${count} total`,
  zones: (count: number) => `${count} ${count === 1 ? 'zone' : 'zones'}`,
  createEyebrow: 'New boundary',
  createHeading: 'Create environment',
  createIntro: 'An environment groups zones and Edge devices. Creation does not scan or alter a network.',
  displayName: 'Display name',
  create: 'Create environment',
  created: 'Environment created.',
  createFailed: 'The environment could not be created. Check the name and try again.',
  nameFirst: 'Name the first environment',
  reauthenticateToCreate: 'Re-authenticate before creating an environment.',
  loading: 'the environment list',
  dependency: 'The Control Plane',
  stillWorks: 'sign-in and navigation',
  doesNotWork: 'the environment list',
  staleReason: 'the last refresh of the environment list did not return',
} as const;

export const ENVIRONMENT_TEXT = {
  back: '← All environments',
  eyebrow: 'Environment',
  loading: 'this environment',
  dependency: 'The Control Plane',
  stillWorks: 'navigation and sign-out',
  doesNotWork: 'this environment record',
  staleReason: 'the last refresh of this environment did not return',
  inventoryEyebrow: 'Inventory truth',
  inventoryHeading: 'Edge devices',
  deviceCollection: 'Edge devices',
  deviceLoading: 'the device inventory',
  deviceDependency: 'The Control Plane',
  deviceStillWorks: 'the rest of this environment',
  deviceDoesNotWork: 'the device inventory',
  deviceStaleReason: 'the last refresh of the device inventory did not return',
  enrollEyebrow: 'One-time handoff',
  enrollHeading: 'Enroll an Edge',
  enrollIntro: 'Create a 15-minute secret for one named Edge. Device health remains unknown until the real Edge reports all conditions.',
  deviceName: 'Device name',
  createSecret: 'Create one-time secret',
  // Distinct from `createSecret` on purpose: two controls with the same
  // accessible name on one screen are ambiguous to a screen reader and to
  // every name-based selector, including the browser suite's.
  enrollFirst: 'Enroll the first Edge',
  secretDenied: 'Enrollment secret creation was denied.',
  reauthenticateToEnroll: 'Re-authenticate before creating an enrollment secret.',
  zonesHeading: 'Private network zones',
  zoneCollection: 'private network zones',
  zoneLoading: 'the zone list',
  zoneDependency: 'The Control Plane',
  zoneStillWorks: 'the rest of this environment',
  zoneDoesNotWork: 'the zone list',
  zoneStaleReason: 'the last refresh of the zone list did not return',
  zoneName: 'Zone name',
  zoneCidr: 'Private CIDR',
  addZone: 'Add zone',
  zoneAdded: 'Private network zone added.',
  zoneFailed: 'Zone creation failed. Use a canonical, non-overlapping RFC1918 CIDR.',
  reauthenticateToAddZone: 'Re-authenticate before adding a zone.',
  nameFirstZone: 'Name the first zone',
  updated: (time: string) => `Updated ${time}`,
  settingsEyebrow: 'Configuration',
  settingsHeading: 'Environment settings',
  displayNameLabel: 'Display name',
  saveName: 'Save name',
  renamed: 'Environment name updated.',
  renameFailed: 'Rename failed. Refresh if another change was made first.',
  reauthenticateToRename: 'Re-authenticate before changing this environment.',
} as const;

export const SECRET_DIALOG_TEXT = {
  title: 'Enrollment secret — shown once',
  description: 'Enter this value directly on the intended Edge host. It will leave this page when you dismiss this dialog.',
  label: 'Enrollment token',
  expires: (time: string) => `Expires ${time}`,
  dismiss: 'I have stored it securely',
} as const;
