/**
 * The operator text catalogue (WCX-08 section 9.1, decision WC-D22).
 *
 * Every word an operator reads lives here. That is not a tidiness exercise:
 * this product's differentiator is its wording. `SRC-07` requires
 * provenance-aware language and `EV-04` requires evidence to precede
 * inference, and wording spread across forty components cannot be reviewed as
 * a whole. Here it can be read in one sitting.
 *
 * **Keys describe meaning, not wording** (section 9.1.1). A key survives a
 * rewrite of the sentence it names; `devices.enrollment.secretShownOnce` is
 * right, `devices.enrollment.enterThisOnTheHost` is not.
 *
 * **Placeholders are `{named}`** and are typechecked: the accessor derives the
 * required values from the string itself, so a missing one fails the build.
 *
 * **Nothing here is composed at runtime.** No entry is built by concatenating
 * two others, because a sentence assembled from fragments cannot be reviewed
 * and cannot later be translated.
 *
 * **Backend text never appears here.** A device-supplied reason, a condition
 * type, a status slug: those are data, rendered through the untrusted
 * components from `WCX-06`, never a catalogue key and never translated.
 */
export const CATALOGUE = {
  // ─── common ────────────────────────────────────────────────────────────
  'common.product': 'Guardian',
  // The decorative monogram. It is a rendered character, so it lives here
  // like every other rendered character, rather than being the one literal
  // a component is allowed to hold.
  'common.brandMark': 'G',
  'common.productScope': 'Control Plane',
  'common.consoleHome': 'Guardian Console home',
  'common.skipToContent': 'Skip to content',
  'common.primaryNavigation': 'Primary navigation',
  'common.signedInAs': 'Signed in as',
  'common.signOut': 'Sign out',
  'common.signOutFailed': 'Sign-out failed. This session is still active.',
  'common.reauthenticate': 'Re-authenticate',
  // One sentence, not three fragments assembled around a link at a call
  // site. The browser suite matches on the opening clause, which is intact.
  'common.sessionReadOnlyFull':
    'Read-only session restored. {reauthenticate} before changing configuration or signing out.',
  'common.controlPlane': 'The Control Plane',
  'common.checkingSession': 'Checking your session',
  'common.loadingScreen': 'Loading this screen',
  'common.cancel': 'Cancel',
  'common.dismiss': 'Dismiss',
  'common.working': 'Working…',
  'common.inProgress': 'In progress',
  'common.progressDetail': 'Started {age} ago · {reason}',
  'common.confidence': 'Confidence',
  'common.confidenceValue': '{label} — {filled} of {total}',

  // ─── auth ──────────────────────────────────────────────────────────────
  'auth.brand': 'Guardian Control Plane',
  'auth.headlineFirst': 'Know what is protected.',
  'auth.headlineSecond': 'Know what needs attention.',
  'auth.intro': 'Device inventory and health remain separate, explicit signals—never an inferred green light.',
  'auth.restoreEyebrow': 'Restore mutation access',
  'auth.ownerEyebrow': 'Owner access',
  'auth.reauthenticateHeading': 'Re-authenticate',
  'auth.signInHeading': 'Sign in',
  'auth.reauthenticateIntro': 'Your cookie session is active, but its CSRF proof was intentionally not persisted.',
  'auth.signInIntro': 'Use your local owner credentials and one MFA method.',
  'auth.methodGroup': 'MFA method',
  'auth.methodAuthenticator': 'Authenticator',
  'auth.methodRecovery': 'Recovery code',
  'auth.username': 'Username',
  'auth.password': 'Password',
  'auth.totpLabel': '6-digit authenticator code',
  'auth.recoveryLabel': 'Recovery code',
  'auth.submit': 'Continue securely',
  'auth.rateLimited': 'Too many attempts. Wait before trying again.',
  'auth.denied': 'Sign-in was denied. Check your credentials and MFA proof.',

  // ─── environments ──────────────────────────────────────────────────────
  'environments.eyebrow': 'Organization workspace',
  'environments.heading': 'Environments',
  'environments.truthNote': 'Configuration is not health',
  'environments.listHeading': 'Configured environments',
  'environments.collection': 'environments',
  'environments.total': '{count} total',
  'environments.zoneCount.one': '{count} zone',
  'environments.zoneCount.other': '{count} zones',
  'environments.createEyebrow': 'New boundary',
  'environments.createHeading': 'Create environment',
  'environments.createIntro': 'An environment groups zones and Edge devices. Creation does not scan or alter a network.',
  'environments.displayName': 'Display name',
  'environments.create': 'Create environment',
  'environments.created': 'Environment created.',
  'environments.createFailed': 'The environment could not be created. Check the name and try again.',
  'environments.nameFirst': 'Name the first environment',
  'environments.reauthenticateToCreate': 'Re-authenticate before creating an environment.',
  'environments.stillWorks': 'sign-in and navigation',
  'environments.doesNotWork': 'the environment list',
  'environments.staleReason': 'the last refresh of the environment list did not return',

  'environment.back': '← All environments',
  'environment.eyebrow': 'Environment',
  'environment.loading': 'this environment',
  'environment.stillWorks': 'navigation and sign-out',
  'environment.doesNotWork': 'this environment record',
  'environment.staleReason': 'the last refresh of this environment did not return',
  'environment.inventoryEyebrow': 'Inventory truth',
  'environment.inventoryHeading': 'Edge devices',
  'environment.deviceCollection': 'Edge devices',
  'environment.deviceStillWorks': 'the rest of this environment',
  'environment.deviceDoesNotWork': 'the device inventory',
  'environment.deviceStaleReason': 'the last refresh of the device inventory did not return',
  'environment.enrollEyebrow': 'One-time handoff',
  'environment.enrollHeading': 'Enroll an Edge',
  'environment.enrollIntro': 'Create a 15-minute secret for one named Edge. Device health remains unknown until the real Edge reports all conditions.',
  'environment.deviceName': 'Device name',
  'environment.createSecret': 'Create one-time secret',
  // Deliberately distinct from `createSecret`: two controls sharing one
  // accessible name are ambiguous to a screen reader and to every name-based
  // selector, the browser suite's included.
  'environment.enrollFirst': 'Enroll the first Edge',
  'environment.secretDenied': 'Enrollment secret creation was denied.',
  'environment.reauthenticateToEnroll': 'Re-authenticate before creating an enrollment secret.',
  'environment.zonesHeading': 'Private network zones',
  'environment.zoneCollection': 'private network zones',
  'environment.zoneStillWorks': 'the rest of this environment',
  'environment.zoneDoesNotWork': 'the zone list',
  'environment.zoneStaleReason': 'the last refresh of the zone list did not return',
  'environment.zoneName': 'Zone name',
  'environment.zoneCidr': 'Private CIDR',
  'environment.addZone': 'Add zone',
  'environment.zoneAdded': 'Private network zone added.',
  'environment.zoneFailed': 'Zone creation failed. Use a canonical, non-overlapping RFC1918 CIDR.',
  'environment.reauthenticateToAddZone': 'Re-authenticate before adding a zone.',
  'environment.nameFirstZone': 'Name the first zone',
  'environment.zoneUpdated': 'Updated {time}',
  'environment.settingsEyebrow': 'Configuration',
  'environment.settingsHeading': 'Environment settings',
  'environment.displayNameLabel': 'Display name',
  'environment.saveName': 'Save name',
  'environment.renamed': 'Environment name updated.',
  'environment.renameFailed': 'Rename failed. Refresh if another change was made first.',
  'environment.reauthenticateToRename': 'Re-authenticate before changing this environment.',

  'environment.secret.title': 'Enrollment secret — shown once',
  'environment.secret.description': 'Enter this value directly on the intended Edge host. It will leave this page when you dismiss this dialog.',
  'environment.secret.label': 'Enrollment token',
  'environment.secret.expires': 'Expires {time}',
  'environment.secret.dismiss': 'I have stored it securely',

  // ─── devices ───────────────────────────────────────────────────────────
  'devices.back': '← Environment overview',
  'devices.eyebrow': 'Edge device',
  'devices.inventoryDimension': 'Inventory',
  'devices.subject': 'this device record',
  'devices.stillWorks': 'navigation and sign-out',
  'devices.doesNotWork': 'this device record',
  'devices.staleReason': 'the last refresh of this device record did not return',
  'devices.factsLabel': 'Device inventory facts',
  'devices.inventoryState': 'Inventory state',
  // `SRC-07`: the record was last written at this time; Guardian does not
  // claim the device was observed then.
  'devices.recordUpdated': 'Record last updated',
  'devices.certificateExpiry': 'Active certificate expiry',
  'devices.noCertificate': 'No active certificate',

  // ─── health ────────────────────────────────────────────────────────────
  'health.eyebrow': 'Backend health projection',
  'health.heading': 'Eight-condition health',
  'health.conditionsLabel': 'Device health conditions',
  'health.blocking': 'Blocking: ',
  // `SRC-07`: the condition was reported by a device, not established by
  // Guardian.
  'health.reportedBy': 'Reported by device:',
  // SRC-07: the device is the observation source, not an established fact
  // about which device is at fault.
  'health.observedSource': 'observed source {device}',
  'health.receivedAt': 'Control Plane received this projection {time}.',
  'health.environmentSubject': 'the health of this environment',
  'health.deviceSubject': 'the health of this device',
  'health.observationSource': 'an enrolled Edge reports its eight health conditions',
  'health.dependency': 'The health projection',
  'health.stillWorks': 'inventory and configuration, which are separate reads',
  'health.doesNotWork': 'every health condition for this scope',
  'health.staleReason': 'the last health refresh did not return',

  'health.condition.edge_connected': 'Edge connection',
  'health.condition.device_certificate_ready': 'Device certificate',
  'health.condition.config_converged': 'Configuration convergence',
  'health.condition.local_database_healthy': 'Local database',
  'health.condition.spool_healthy': 'Event spool',
  'health.condition.clock_quality': 'Clock quality',
  'health.condition.container_runtime_reachable': 'Container runtime',
  'health.condition.privileged_helper_reachable': 'Privileged helper',

  // ─── states ────────────────────────────────────────────────────────────
  // No state may read as healthy, complete, or successful. The word "healthy"
  // appears exactly once below, inside the sentence that denies it.
  'states.loading.activity': 'Loading {subject}',
  'states.loading.announce': '{activity}…',
  'states.empty.heading': 'No {collection} recorded',
  'states.empty.body': 'Guardian read this scope and found zero {collection}. That is a confirmed count, not a failed read, and it reports nothing about whether anything is working.',
  'states.unknown.heading': 'No observation exists',
  'states.unknown.body': 'Guardian holds no observation of {subject}. Absence of an observation is not a healthy signal and it is not a failure signal.',
  'states.unknown.source': 'An observation would appear once {source}.',
  'states.stale.heading': 'Showing the last data Guardian received',
  'states.stale.age': 'Observed {age} ago. It may no longer match the environment.',
  'states.stale.reason': 'Refresh is not current: {reason}',
  'states.partial.heading': 'Some sources could not be read',
  'states.partial.body': 'What is shown below came from the sources that answered. The rest is missing, not empty.',
  'states.partial.listLabel': 'Sources that could not be read',
  'states.partial.retry': 'Retry the missing sources',
  'states.degraded.heading': '{dependency} is impaired',
  'states.degraded.works': 'Still answering: {detail}',
  'states.degraded.broken': 'Not answering: {detail}',
  'states.degraded.retry': 'Try again',
  'states.denied.heading': 'Access was refused',
  // Deliberately says nothing about what exists. A refusal that leaked "no
  // devices" or "3 devices" would answer the question the refusal withheld.
  'states.denied.body': 'Guardian refused access to this data. A refusal is not an absence: it reports nothing about what is or is not here.',
  'states.denied.reauthenticate': 'Re-authenticate and try again.',
  'states.error.heading': 'This data could not be loaded',
  'states.error.body': 'Guardian could not finish this read. Nothing about the underlying failure is shown here.',
  'states.error.retry': 'Try again',

  'states.rootFailure.heading': 'The console stopped unexpectedly',
  'states.rootFailure.body': 'This is a fault in the console itself, not a report about your environment. Nothing here tells you whether Guardian is protecting anything right now.',
  'states.rootFailure.reload': 'Reload the console',
  'states.screenFailure.heading': 'This screen stopped unexpectedly',
  'states.screenFailure.body': 'The rest of the console still works. Move to another screen, or sign out from the sidebar.',
  'states.screenFailure.retry': 'Try this screen again',

  // ─── confirmations ─────────────────────────────────────────────────────
  'confirm.recoverable': '{effect}: “{object}”. This removes it from Guardian. Anything Guardian already recorded about it is kept.',
  'confirm.irreversible': '{effect}: “{object}”. This cannot be undone.',
  'confirm.typeToConfirm': 'Type the exact name to continue: {object}',
  'confirm.typeLabel': 'Object name',
  'confirm.stepUpRequired': 'Step-up reauthentication is required and is not available yet, so this action cannot be completed.',

  // ─── untrusted content ─────────────────────────────────────────────────
  'untrusted.legend': 'Characters shown as \\xNN or \\uNNNN were control or invisible characters in the captured value. They are displayed, never interpreted.',
  'untrusted.escapeLabel': 'escaped {description}',
  'untrusted.truncatedText': 'Showing the first {shown} of {original} characters.',
  'untrusted.truncatedBlock': 'Showing the first {shown} of {original} characters. Copy to get the whole captured value.',
  'untrusted.copy': 'Copy the original untrusted value',
  'untrusted.copied': 'Copied the original value to the clipboard.',
  'untrusted.copyFailed': 'The clipboard is unavailable, so nothing was copied.',
  'untrusted.blockLabel': 'Captured content',

  // ─── time ──────────────────────────────────────────────────────────────
  'time.unknown': 'No timestamp was recorded',
  'time.utcDescription': 'UTC {iso}',
  'time.relative': '{age} ago',
  'time.degradedClock': 'clock quality degraded',
  'time.degradedClockDescription': 'The source device reported degraded clock quality, so this time may be inaccurate.',
  'time.age.second.one': '{count} second',
  'time.age.second.other': '{count} seconds',
  'time.age.minute.one': '{count} minute',
  'time.age.minute.other': '{count} minutes',
  'time.age.hour.one': '{count} hour',
  'time.age.hour.other': '{count} hours',
  'time.age.day.one': '{count} day',
  'time.age.day.other': '{count} days',
  'time.age.unknown': 'an unknown age',

  // ─── errors ────────────────────────────────────────────────────────────
  // Keyed by the `messageKey` values `WCX-02` reserved, so its temporary map
  // is deleted rather than rewritten (section 9.1.5).
  'errors.unauthenticated': 'Your session is no longer valid. Sign in again.',
  'errors.reauthentication-required': 'Re-authenticate before changing configuration.',
  'errors.forbidden': 'This action was refused.',
  'errors.not-found': 'This record is unavailable or outside this environment.',
  'errors.validation': 'Guardian rejected the submitted values.',
  'errors.conflict': 'Another change was recorded first. Reload the current value.',
  'errors.rate-limited': 'Too many attempts. Wait before trying again.',
  'errors.unavailable': 'Guardian could not complete the request.',
  'errors.timeout': 'The request took too long to complete.',
  'errors.network': 'Guardian could not be reached.',
  'errors.unexpected': 'Guardian could not complete the request.',
} as const;

export type CatalogueKey = keyof typeof CATALOGUE;
