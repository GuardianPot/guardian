import { useSearchParams } from 'react-router';

/**
 * The environment scope parameter (WCX-10 sections 8.2 and 9.2, WC-D14).
 *
 * `WC-D14` puts the scope in the URL rather than in memory or storage, so a
 * link an operator pastes into an incident channel resolves to the same view
 * for whoever opens it. That is the whole reason it is a query parameter.
 *
 * It is also **untrusted input**. Anyone can type it, and it arrives from a
 * shared link written by someone the console has never authenticated. Three
 * rules follow, and all three are about refusing to guess:
 *
 * 1. It is validated against the UUID pattern *before* it is used, so a
 *    malformed value never reaches a request path.
 * 2. An unknown or denied environment renders `not-found` or `denied` as the
 *    Control Plane reports it.
 * 3. **There is no fallback.** Not to the first environment, not to the only
 *    environment, not to the last one viewed. An operator who believes they
 *    are looking at environment A while seeing environment B will act on the
 *    wrong network, and nothing about the screen would tell them.
 */
export const SCOPE_PARAM = 'env';

/**
 * The identifier shape the Control Plane issues.
 *
 * Deliberately the general UUID pattern rather than the contract's UUIDv7
 * pattern: this is an input filter, not a schema check. Its job is to keep
 * anything that is not an identifier out of a URL path, and the backend
 * decides whether the identifier names something.
 */
const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

export function isEnvironmentId(value: string | null | undefined): value is string {
  return typeof value === 'string' && UUID.test(value);
}

export type Scope =
  /** No `?env=` at all. Scope-aware surfaces ask for a selection. */
  | { state: 'absent' }
  /** Present but not an identifier. Rendered `not-found`, never ignored. */
  | { state: 'malformed'; raw: string }
  | { state: 'selected'; environmentId: string };

export function readScope(params: URLSearchParams): Scope {
  const raw = params.get(SCOPE_PARAM);
  if (raw === null || raw === '') return { state: 'absent' };
  if (!isEnvironmentId(raw)) return { state: 'malformed', raw };
  return { state: 'selected', environmentId: raw };
}

export type ScopeControl = {
  scope: Scope;
  /**
   * Writes the scope onto the current route, replacing rather than pushing.
   *
   * Replacing keeps the back button meaning "the screen before this one"
   * rather than "the same screen with a different environment", which is what
   * an operator pressing it expects. `null` clears the parameter.
   */
  select: (environmentId: string | null) => void;
};

export function useScope(): ScopeControl {
  const [params, setParams] = useSearchParams();
  return {
    scope: readScope(params),
    select(environmentId) {
      const next = new URLSearchParams(params);
      // Everything else on the URL is preserved: the parameter is one part of
      // a link an operator may have built deliberately.
      if (environmentId === null) next.delete(SCOPE_PARAM);
      else next.set(SCOPE_PARAM, environmentId);
      setParams(next, { replace: true });
    },
  };
}
