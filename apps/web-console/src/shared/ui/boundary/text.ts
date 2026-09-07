/**
 * Error boundary text (WCX-04 sections 9.2 and 9.10).
 *
 * Every string is fixed. Section 8.8 forbids a fallback from rendering an
 * exception message, a stack, a component stack, or any request or response
 * content, so no value below is derived from the caught error, and the
 * boundary never passes one in.
 */
export const ROOT_BOUNDARY_TEXT = {
  product: 'Guardian',
  heading: 'The console stopped unexpectedly',
  body: 'This is a fault in the console itself, not a report about your environment. Nothing here tells you whether Guardian is protecting anything right now.',
  reload: 'Reload the console',
} as const;

export const ROUTE_BOUNDARY_TEXT = {
  heading: 'This screen stopped unexpectedly',
  body: 'The rest of the console still works. Move to another screen, or sign out from the sidebar.',
  retry: 'Try this screen again',
} as const;
