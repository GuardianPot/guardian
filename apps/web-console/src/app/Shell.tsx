import { Suspense, useState } from 'react';
import { Link, NavLink, Outlet } from 'react-router';
import { useAuth, useCapability } from '@features/auth';
import { Banner, Button, InlineMessage, LoadingState, RouteErrorBoundary } from '@shared/ui';
import styles from '@shared/styles/app.module.css';
import { t, tx } from '@shared/text';

export function Shell() {
  const auth = useAuth();
  const [signOutError, setSignOutError] = useState('');
  // Signing out revokes the current session, so it shares that capability.
  const signOut = useCapability('session.revoke');
  return (
    <div className={styles.shell}>
      <a className={styles.skipLink} href="#main-content">{t('common.skipToContent')}</a>
      <aside className={styles.sidebar}>
        <Link className={styles.brand} to="/environments" aria-label={t('common.consoleHome')}>
          <span className={styles.brandMark} aria-hidden="true">{t('common.brandMark')}</span>
          <span><strong>{t('common.product')}</strong><small>{t('common.productScope')}</small></span>
        </Link>
        <nav aria-label={t('common.primaryNavigation')}>
          <NavLink to="/environments" className={({ isActive }) => isActive ? styles.navActive : styles.navLink}>{t('environments.heading')}</NavLink>
          <NavLink to="/account" className={({ isActive }) => isActive ? styles.navActive : styles.navLink}>{t('account.heading')}</NavLink>
        </nav>
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
      </aside>
      <div className={styles.workspace}>
        {!signOut.allowed && (
          <Banner tone="restricted">
            {/*
              One catalogue entry, not three fragments around a link. A
              sentence assembled at the call site cannot be reviewed as a
              sentence (WCX-08 section 9.1.6).
            */}
            {tx('common.sessionReadOnlyFull', {
              reauthenticate: <Link to="/login">{t('common.reauthenticate')}</Link>,
            })}
          </Banner>
        )}
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
