import { useState } from 'react';
import { Link, NavLink, Outlet } from 'react-router-dom';
import { useAuth, useCapability } from '@features/auth';
import styles from '@shared/styles/app.module.css';

export function Shell() {
  const auth = useAuth();
  const [signOutError, setSignOutError] = useState('');
  // Signing out revokes the current session, so it shares that capability.
  const signOut = useCapability('session.revoke');
  return (
    <div className={styles.shell}>
      <a className={styles.skipLink} href="#main-content">Skip to content</a>
      <aside className={styles.sidebar}>
        <Link className={styles.brand} to="/environments" aria-label="Guardian Console home">
          <span className={styles.brandMark} aria-hidden="true">G</span>
          <span><strong>Guardian</strong><small>Control Plane</small></span>
        </Link>
        <nav aria-label="Primary navigation">
          <NavLink to="/environments" className={({ isActive }) => isActive ? styles.navActive : styles.navLink}>Environments</NavLink>
        </nav>
        <div className={styles.operator}>
          <span>Signed in as</span>
          <strong>{auth.session?.username}</strong>
          {signOut.allowed ? (
            <button
              className={styles.textButton}
              type="button"
              onClick={() => {
                setSignOutError('');
                // A refused sign-out must stay visible instead of silently
                // leaving the operator on an apparently ended session.
                auth.logout().catch(() => setSignOutError('Sign-out failed. This session is still active.'));
              }}
            >
              Sign out
            </button>
          ) : (
            <Link to="/login">Re-authenticate</Link>
          )}
          {signOutError && <p className={styles.formError} role="alert">{signOutError}</p>}
        </div>
      </aside>
      <div className={styles.workspace}>
        {!signOut.allowed && (
          <div className={styles.reauthBanner} role="status">
            Read-only session restored. <Link to="/login">Re-authenticate</Link> before changing configuration or signing out.
          </div>
        )}
        <main id="main-content" className={styles.main} tabIndex={-1}><Outlet /></main>
      </div>
    </div>
  );
}
