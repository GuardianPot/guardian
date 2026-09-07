import type { EnrollmentSecret } from '@shared/api/types';
import { OneTimeSecretDialog } from '@shared/ui';

/**
 * The first enrollment secret (`W11-C3-A`).
 *
 * The dialog itself is shared with device re-enrollment since `WCX-09`, so
 * the one-time rules — no query cache, no re-display path, gone on dismissal
 * and on unload — have exactly one implementation. This wrapper only names
 * which wording the enrollment case uses.
 */
export function SecretDialog({ secret, onDismiss }: { secret: EnrollmentSecret | null; onDismiss: () => void }) {
  return (
    <OneTimeSecretDialog
      secret={secret}
      onDismiss={onDismiss}
      titleKey="environment.secret.title"
      descriptionKey="environment.secret.description"
      labelKey="environment.secret.label"
    />
  );
}
