import { Outlet, useLocation, useMatches } from 'react-router-dom';
import { RouteAnnouncer } from './RouteAnnouncer';
import { screenFromHandle, type ScreenName } from './screens';

/**
 * The persistent root of the route tree (WCX-05 section 9.2).
 *
 * It exists so the live region has an owner that outlives every navigation.
 * A region re-created per screen would be a new node each time, and a screen
 * reader has nothing to compare a brand-new region against — the announcement
 * is either missed or duplicated.
 *
 * The screen name comes from the matched route's `handle`, so it is a property
 * of the route definition and can never be a record's display name.
 */
export function AppLayout() {
  const matches = useMatches();
  const location = useLocation();
  // The deepest match that names a screen wins, so a nested route can refine
  // the name its parent set.
  const screen = matches.reduce<ScreenName | undefined>(
    (found, match) => screenFromHandle(match.handle) ?? found,
    undefined,
  );

  return (
    <>
      <RouteAnnouncer screen={screen} locationKey={location.key} />
      <Outlet />
    </>
  );
}
