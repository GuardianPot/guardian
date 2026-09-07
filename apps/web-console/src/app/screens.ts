/**
 * Screen names (WCX-05 sections 9.2.1 and 9.10).
 *
 * These name the *route*, never the record it happens to be showing. An
 * environment's document title is "Environment — Guardian Console", not the
 * environment's display name: section 8.1 keeps backend and attacker-supplied
 * content out of accessible names, announcements, and the title bar, and a
 * title is one of the few places a hostile string would travel outside the
 * document — into a browser tab, a window list, and a bookmark.
 *
 * Catalogue-ready constants so `WCX-08` can move them without renaming.
 */
export const SCREEN = {
  signIn: 'Sign in',
  environments: 'Environments',
  environment: 'Environment',
  device: 'Edge device',
} as const;

export type ScreenName = (typeof SCREEN)[keyof typeof SCREEN];

export const PRODUCT = 'Guardian Console';

/** `<screen name> — Guardian Console`, per section 9.2.1. */
export function documentTitle(screen: string | undefined): string {
  return screen === undefined ? PRODUCT : `${screen} — ${PRODUCT}`;
}

/** What the polite live region says on a completed navigation. */
export function navigationAnnouncement(screen: string | undefined): string {
  return screen === undefined ? '' : `${screen} screen`;
}

/**
 * The shape a route puts in its `handle` so the announcer can read its name
 * from the route definition rather than from rendered content.
 */
export type ScreenHandle = { screen: ScreenName };

export function screenFromHandle(handle: unknown): ScreenName | undefined {
  if (typeof handle !== 'object' || handle === null) return undefined;
  const candidate = (handle as Partial<ScreenHandle>).screen;
  return typeof candidate === 'string' ? candidate : undefined;
}
