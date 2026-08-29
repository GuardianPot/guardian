import * as Dialog from '@radix-ui/react-dialog';
import type { EnrollmentSecret } from '../api/types';
import styles from '../styles/app.module.css';

export function SecretDialog({ secret, onDismiss }: { secret: EnrollmentSecret | null; onDismiss(): void }) {
  return (
    <Dialog.Root open={secret !== null} onOpenChange={(open) => { if (!open) onDismiss(); }}>
      <Dialog.Portal>
        <Dialog.Overlay className={styles.dialogOverlay} />
        <Dialog.Content className={styles.dialog} onEscapeKeyDown={onDismiss}>
          <Dialog.Title>Enrollment secret — shown once</Dialog.Title>
          <Dialog.Description>
            Enter this value directly on the intended Edge host. It will leave this page when you dismiss this dialog.
          </Dialog.Description>
          {secret && (
            <div className={styles.secretBox} data-testid="enrollment-secret">
              <span>Enrollment token</span>
              <code>{secret.token}</code>
              <small>Expires {formatExpiry(secret.expires_at)}</small>
            </div>
          )}
          <div className={styles.dialogActions}>
            <Dialog.Close asChild>
              <button className={styles.primaryButton} type="button">I have stored it securely</button>
            </Dialog.Close>
          </div>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}

function formatExpiry(value: string) {
  return new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value));
}
