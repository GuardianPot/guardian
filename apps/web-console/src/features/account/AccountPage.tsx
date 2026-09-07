import styles from '@shared/styles/app.module.css';
import { t } from '@shared/text';
import { PasswordPanel } from './PasswordPanel';
import { SessionPanel } from './SessionPanel';

/**
 * The owner account area (WCX-09 section 9.4).
 *
 * Two panels, in this order on purpose. The password change is the action an
 * operator came here to take; the session list is the evidence of what it did.
 * Reading them the other way round would put the outcome above the cause.
 */
export function AccountPage() {
  return (
    <div>
      <header className={styles.pageHeader}>
        <div>
          <p className={styles.eyebrow}>{t('account.eyebrow')}</p>
          <h1 tabIndex={-1}>{t('account.heading')}</h1>
        </div>
      </header>
      <PasswordPanel />
      <SessionPanel />
    </div>
  );
}
