import type { EnrollmentSecret } from '@shared/api/types';
import { Button, Dialog } from '@shared/ui';
import styles from '@shared/styles/app.module.css';
import { SECRET_DIALOG_TEXT as TEXT } from './text';
import { formatTime } from '@features/health';

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
      title={TEXT.title}
      description={TEXT.description}
      actions={<Button variant="primary" onClick={onDismiss}>{TEXT.dismiss}</Button>}
    >
      {secret && (
        <div className={styles.secretBox} data-testid="enrollment-secret">
          <span>{TEXT.label}</span>
          <code>{secret.token}</code>
          <small>{TEXT.expires(formatTime(secret.expires_at))}</small>
        </div>
      )}
    </Dialog>
  );
}
