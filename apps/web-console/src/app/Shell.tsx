import { useState } from 'react';
import { Link, NavLink, Outlet } from 'react-router-dom';
import { useAuth, useCapability } from '@features/auth';
import { Banner, Button, InlineMessage, RouteErrorBoundary } from '@shared/ui';
import styles from '@shared/styles/app.module.css';
import { SHELL_TEXT } from './text';

export function Shell() {
  const auth = useAuth();
  const [signOutError, setSignOutError] = useState('');
  // Signing out revokes the current session, so it shares that capability.
  const signOut = useCapability('session.revoke');
  return (
    <div className={styles.shell}>
      <a className={styles.skipLink} href="#main-content">{SHELL_TEXT.skipToContent}</a>
      <aside className={styles.sidebar}>
        <Link className={styles.brand} to="/environments" aria-label={SHELL_TEXT.homeLabel}>
          <span className={styles.brandMark} aria-hidden="true">G</span>
          <span><strong>{SHELL_TEXT.product}</strong><small>{SHELL_TEXT.productScope}</small></span>
        </Link>
        <nav aria-label={SHELL_TEXT.primaryNavigation}>
          <NavLink to="/environments" className={({ isActive }) => isActive ? styles.navActive : styles.navLink}>{SHELL_TEXT.environments}</NavLink>
        </nav>
        <div className={styles.operator}>
          <span>{SHELL_TEXT.signedInAs}</span>
          <strong>{auth.session?.username}</strong>
          {signOut.allowed ? (
            <Button
              variant="quiet"
              onClick={() => {
                setSignOutError('');
                // A refused sign-out must stay visible instead of silently
                // leaving the operator on an apparently ended session.
                auth.logout().catch(() => setSignOutError(SHELL_TEXT.signOutFailed));
              }}
            >
              {SHELL_TEXT.signOut}
            </Button>
          ) : (
            <Link to="/login">{SHELL_TEXT.reauthenticate}</Link>
          )}
          {signOutError && <InlineMessage tone="error">{signOutError}</InlineMessage>}
        </div>
      </aside>
      <div className={styles.workspace}>
        {!signOut.allowed && (
          <Banner tone="restricted">
            {SHELL_TEXT.readOnlySession} <Link to="/login">{SHELL_TEXT.reauthenticate}</Link> {SHELL_TEXT.beforeChanging}
          </Banner>
        )}
        <main id="main-content" className={styles.main} tabIndex={-1}>
          {/*
            One route boundary around the outlet rather than one per element:
            it survives navigation between screens, so its reset actually has
            something to reset, and a screen that throws leaves the sidebar —
            and therefore sign-out — mounted.
          */}
          <RouteErrorBoundary><Outlet /></RouteErrorBoundary>
        </main>
      </div>
    </div>
  );
}
