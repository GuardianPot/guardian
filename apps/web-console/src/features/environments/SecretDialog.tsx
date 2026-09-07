import type { EnrollmentSecret } from '@shared/api/types';
import { Button, Dialog, Timestamp } from '@shared/ui';
import styles from '@shared/styles/app.module.css';
import { t, tx } from '@shared/text';

/**
 * The one-time enrollment secret (`W11-C3-A`).
 *
 * The value lives in route-local state only and leaves the DOM on dismissal.
 * The dialog is the shared one, so it inherits the focus trap, the escape
 * handler, and focus return without restating them here.
 */
export function SecretDialog({ secret, onDismiss }: { secret: EnrollmentSecret | null; onDismiss: () => void }) {
  return (
    <Dialog
      open={secret !== null}
      onClose={onDismiss}
      title={t('environment.secret.title')}
      description={t('environment.secret.description')}
      actions={<Button variant="primary" onClick={onDismiss}>{t('environment.secret.dismiss')}</Button>}
    >
      {secret && (
        <div className={styles.secretBox} data-testid="enrollment-secret">
          <span>{t('environment.secret.label')}</span>
          <code>{secret.token}</code>
          {/* Evidence-adjacent: a 15-minute secret needs seconds to be actionable. */}
          <small>{tx('environment.secret.expires', { time: <Timestamp value={secret.expires_at} precision="second" /> })}</small>
        </div>
      )}
    </Dialog>
  );
}
