import { Suspense, useEffect, useId, useRef, useState } from 'react';
import { Link, NavLink, Outlet, useLocation } from 'react-router';
import { toConsoleError } from '@shared/api/error';
import { useAuth, useCapability } from '@features/auth';
import { Banner, Button, InlineMessage, LoadingState, RouteErrorBoundary } from '@shared/ui';
import styles from '@shared/styles/app.module.css';
import { t, tx } from '@shared/text';
import { ScopeSelector } from './ScopeSelector';
import { useNarrowViewport } from './useNarrowViewport';

/**
 * The application shell (WCX-10 section 9.3).
 *
 * One layout, one breakpoint at 900 pixels, and one rule that outranks
 * everything else in this file: **no operator control is removed at any
 * viewport width.**
 *
 * That rule exists because it was broken. `P1-W11` gave `.operator` a
 * `display: none` below 900 pixels, which took sign-out and the
 * re-authentication link off every phone-sized screen — an operator who
 * suspected a stolen session could not end it from the device in their hand.
 * `WCX-05` closed the defect by making the block wrap instead of vanish;
 * `WCX-10` replaces that with the design its section 9.3 defines, and the
 * invariant is now asserted at four widths in `shell.test.tsx` and again in
 * the browser suite at 375 pixels.
 *
 * Above the breakpoint: a persistent sidebar with navigation and the operator
 * block. Below it: both move into a disclosure, which is a real button with
 * real state rather than a CSS trick, because it has to close on escape and on
 * navigation and return focus to its trigger. The disclosure only exists
 * below the breakpoint — above it, there is nothing to disclose and the panel
 * is unconditionally present.
 */
export function Shell() {
  const auth = useAuth();
  const location = useLocation();
  const narrow = useNarrowViewport();
  const [open, setOpen] = useState(false);
  const [signOutError, setSignOutError] = useState('');
  const triggerRef = useRef<HTMLButtonElement>(null);
  const panelId = useId();
  // Signing out revokes the current session, so it shares that capability.
  const signOut = useCapability('session.revoke');

  /*
   * The disclosure closes for two reasons, both handled during render rather
   * than in an effect so there is no frame where it is still open.
   *
   * Navigating (section 9.3.4) is the operator saying they are done with the
   * menu, and leaving it open covers the screen they just asked for. Widening
   * past the breakpoint dissolves the disclosure entirely, so its state must
   * not survive to the next time the viewport narrows.
   */
  const [context, setContext] = useState({ path: location.pathname, narrow });
  if (context.path !== location.pathname || context.narrow !== narrow) {
    setContext({ path: location.pathname, narrow });
    setOpen(false);
  }

  useEffect(() => {
    if (!narrow || !open) return;
    const onKey = (event: KeyboardEvent) => {
      if (event.key !== 'Escape') return;
      setOpen(false);
      // Section 9.3.4: focus returns to the trigger, not to the top of the
      // document, so a keyboard operator stays where they were.
      triggerRef.current?.focus();
    };
    window.addEventListener('keydown', onKey);
    return () => { window.removeEventListener('keydown', onKey); };
  }, [narrow, open]);

  const panelVisible = !narrow || open;

  return (
    <div className={styles.shell}>
      {/* Section 9.5.2: still the first focusable element, still targets main. */}
      <a className={styles.skipLink} href="#main-content">{t('common.skipToContent')}</a>
      <aside className={styles.sidebar}>
        <div className={styles.sidebarBar}>
          <Link className={styles.brand} to="/" aria-label={t('common.consoleHome')}>
            <span className={styles.brandMark} aria-hidden="true">{t('common.brandMark')}</span>
            <span><strong>{t('common.product')}</strong><small>{t('common.productScope')}</small></span>
          </Link>
          {narrow && (
            <Button
              variant="quiet"
              buttonRef={triggerRef}
              expanded={open}
              controls={panelId}
              onClick={() => { setOpen((current) => !current); }}
            >
              {t(open ? 'common.closeNavigation' : 'common.openNavigation')}
            </Button>
          )}
        </div>

        {/*
          One panel holding navigation, scope, and the operator block. They
          travel together because they are all things an operator reaches for,
          and splitting them is how one of them gets left behind at a width
          nobody tested.
        */}
        <div id={panelId} className={styles.sidebarPanel} hidden={!panelVisible}>
          <nav aria-label={t('common.primaryNavigation')}>
            {/*
              Section 9.1: an entry exists only when its screen does. Phase 3
              adds incident content to Home; Phase 2 decoy work adds its own.
            */}
            <ShellLink to="/" end>{t('home.heading')}</ShellLink>
            <ShellLink to="/environments">{t('environments.heading')}</ShellLink>
            <ShellLink to="/account">{t('account.heading')}</ShellLink>
          </nav>
          <ScopeSelector />
          <div className={styles.operator}>
            <span>{t('common.signedInAs')}</span>
            <strong>{auth.session?.username}</strong>
            {signOut.allowed ? (
              <Button
                variant="quiet"
                onClick={() => {
                  setSignOutError('');
                  // A refused sign-out must stay visible instead of silently
                  // leaving the operator on an apparently ended session.
                  auth.logout().catch(() => setSignOutError(t('common.signOutFailed')));
                }}
              >
                {t('common.signOut')}
              </Button>
            ) : (
              <Link to="/login">{t('common.reauthenticate')}</Link>
            )}
            {signOutError && <InlineMessage tone="error">{signOutError}</InlineMessage>}
          </div>
        </div>
      </aside>
      <div className={styles.workspace}>
        {/* Section 9.3.5: present at every width, disclosure or not. */}
        {!signOut.allowed && <ReadOnlyBanner />}
        <main id="main-content" className={styles.main} tabIndex={-1}>
          {/*
            One route boundary around the outlet rather than one per element:
            it survives navigation between screens, so its reset actually has
            something to reset, and a screen that throws leaves the sidebar —
            and therefore sign-out — mounted.
          */}
          {/*
            `Suspense` sits inside the boundary, so a chunk that fails to load
            throws into the route fallback rather than blanking the console
            (WCX-07 section 9.9.4). Its fallback is the same `loading` state
            every other pending read uses, announced once through `status`.
          */}
          <RouteErrorBoundary>
            <Suspense fallback={<LoadingState activity={t('common.loadingScreen')} />}>
              <Outlet />
            </Suspense>
          </RouteErrorBoundary>
        </main>
      </div>
    </div>
  );
}

/**
 * The reload-restored session, and the way out of it.
 *
 * `W11-C3-A` keeps the synchronizer proof in browser memory, so a reload
 * leaves a valid session that cannot change anything. Until change proposal
 * `0003` the only way forward was a full sign-in — password and MFA — every
 * time. That cost almost nothing in Phase 1 and a great deal from Phase 2 on,
 * and an operator who finds reloading expensive stops reloading, which in a
 * detection product works directly against the product.
 *
 * `Restore write access` exchanges the surviving cookie for a new proof. It
 * restores level 1 and level 2 controls and nothing else: level 3 is gated by
 * step-up whatever the proof's age, because a proof says the browser has a
 * session and not that the operator is still the one holding it.
 *
 * Full sign-in stays on the banner beside it. A refused re-issue must leave a
 * way forward rather than a dead end.
 */
function ReadOnlyBanner() {
  const auth = useAuth();
  const [restoring, setRestoring] = useState(false);
  const [failed, setFailed] = useState('');

  async function restore() {
    setFailed('');
    setRestoring(true);
    try {
      await auth.restoreWriteAccess();
    } catch (caught) {
      // Rate limiting is the one refusal worth naming: it says to wait rather
      // than to try something else, and the console never retries on its own.
      setFailed(t(toConsoleError(caught).kind === 'rate-limited'
        ? 'common.restoreRateLimited'
        : 'common.restoreFailed'));
    } finally {
      setRestoring(false);
    }
  }

  return (
    <Banner tone="restricted">
      {/*
        One catalogue entry, not three fragments around a link. A sentence
        assembled at the call site cannot be reviewed as a sentence
        (WCX-08 section 9.1.6).
      */}
      {tx('common.sessionReadOnlyFull', {
        reauthenticate: <Link to="/login">{t('common.reauthenticate')}</Link>,
      })}
      <span className={styles.bannerActions}>
        <Button variant="secondary" pending={restoring} onClick={() => { void restore(); }}>
          {t('common.restoreWriteAccess')}
        </Button>
      </span>
      {failed !== '' && <InlineMessage tone="error">{failed}</InlineMessage>}
    </Banner>
  );
}

/**
 * A navigation entry.
 *
 * Section 9.4.3: the active entry is marked by more than colour. `aria-current`
 * carries it for assistive technology and the stylesheet adds a weight change
 * and a leading rule, so an operator who cannot distinguish the accent still
 * sees which entry is current.
 */
function ShellLink({ to, end, children }: { to: string; end?: boolean; children: string }) {
  return (
    <NavLink
      to={to}
      end={end ?? false}
      className={({ isActive }) => (isActive ? styles.navActive : styles.navLink)}
    >
      {children}
    </NavLink>
  );
}
