import { useEffect } from 'react';
import { Button } from '@shared/ui/controls/Button';
import { Dialog } from '@shared/ui/controls/Dialog';
import { Timestamp } from '@shared/ui/time/Timestamp';
import styles from '@shared/styles/app.module.css';
import { t, tx, type PlainCatalogueKey } from '@shared/text';

/**
 * One-time bootstrap material, shown exactly once (W11-C3-A, WCX-09 8.11).
 *
 * `WCX-04` shipped this inside the environment feature for the first
 * enrollment secret. `WCX-09` added a second producer — the re-enrollment
 * token — and moved it here rather than copying it, because the rules it
 * holds are the kind that are only ever broken by a copy drifting:
 *
 * - the value is a prop, never state here and never in a query cache. The
 *   caller holds it in route-local state, so route exit destroys it;
 * - dismissal unmounts the dialog, so the value leaves the DOM;
 * - unload clears it too, through the caller's effect below;
 * - there is no copy control, no reveal toggle, and no second read path. A
 *   source-level check in `oneTimeSecret.test.ts` asserts no re-display path
 *   exists, because the guarantee is about what the code *cannot* do.
 *
 * The value is rendered in a `<code>` element as text. It is issued by the
 * Control Plane rather than by a device, so it is not untrusted content — but
 * it is still never interpolated into markup.
 */
export type OneTimeSecret = {
  token: string;
  expires_at: string;
};

export type OneTimeSecretDialogProps = {
  secret: OneTimeSecret | null;
  onDismiss: () => void;
  /**
   * Titles and body copy differ per producer; the rules do not. The `Key`
   * suffix is what tells the literal-text lint rule these carry catalogue
   * keys, and the type is what stops one carrying a sentence instead.
   */
  titleKey: PlainCatalogueKey;
  descriptionKey: PlainCatalogueKey;
  labelKey: PlainCatalogueKey;
};

export function OneTimeSecretDialog({
  secret,
  onDismiss,
  titleKey,
  descriptionKey,
  labelKey,
}: OneTimeSecretDialogProps) {
  useEffect(() => {
    if (secret === null) return;
    // A tab closed or reloaded while the secret is on screen must not leave
    // it in a restored page or a back-forward cache entry.
    const drop = () => { onDismiss(); };
    window.addEventListener('pagehide', drop);
    return () => { window.removeEventListener('pagehide', drop); };
  }, [secret, onDismiss]);

  return (
    <Dialog
      open={secret !== null}
      onClose={onDismiss}
      title={t(titleKey)}
      description={t(descriptionKey)}
      actions={<Button variant="primary" onClick={onDismiss}>{t('secret.dismiss')}</Button>}
    >
      {secret && (
        <div className={styles.secretBox} data-testid="one-time-secret">
          <span>{t(labelKey)}</span>
          <code>{secret.token}</code>
          {/* Evidence-adjacent: a 15-minute secret needs seconds to be actionable. */}
          <small>{tx('secret.expires', { time: <Timestamp value={secret.expires_at} precision="second" /> })}</small>
        </div>
      )}
    </Dialog>
  );
}
